package tools

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mxschmitt/playwright-go"
)

var (
	pw      *playwright.Playwright
	browser playwright.Browser
	page    playwright.Page
	sessMu  sync.Mutex
)

func closeSession() {
	sessMu.Lock()
	defer sessMu.Unlock()
	if page != nil {
		page.Close()
		page = nil
	}
	if browser != nil {
		browser.Close()
		browser = nil
	}
	if pw != nil {
		pw.Stop()
		pw = nil
	}
}

func getPage() (playwright.Page, error) {
	sessMu.Lock()
	defer sessMu.Unlock()

	if page != nil && !page.IsClosed() {
		return page, nil
	}

	closeSessionLocked()

	var err error
	pw, err = playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("playwright: %w", err)
	}

	browser, err = pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{Headless: playwright.Bool(true)})
	if err != nil {
		pw.Stop()
		pw = nil
		return nil, fmt.Errorf("playwright launch: %w", err)
	}

	ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{Locale: playwright.String("en-US")})
	if err != nil {
		browser.Close()
		browser = nil
		pw.Stop()
		pw = nil
		return nil, fmt.Errorf("playwright context: %w", err)
	}

	page, err = ctx.NewPage()
	if err != nil {
		browser.Close()
		browser = nil
		pw.Stop()
		pw = nil
		return nil, fmt.Errorf("playwright page: %w", err)
	}

	return page, nil
}

func closeSessionLocked() {
	if page != nil {
		page.Close()
		page = nil
	}
	if browser != nil {
		browser.Close()
		browser = nil
	}
	if pw != nil {
		pw.Stop()
		pw = nil
	}
}

func pwSelector(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, ".") ||
		strings.HasPrefix(raw, "[") || strings.HasPrefix(raw, "//") ||
		strings.HasPrefix(raw, "text=") {
		return raw
	}
	for _, c := range "#.[]>=:" {
		if strings.ContainsRune(raw, c) {
			return raw
		}
	}
	if strings.Contains(raw, "//") {
		return raw
	}
	return "text=" + raw
}

func pageSnapshot(p playwright.Page) string {
	text := ""
	result, err := p.Evaluate("() => document.body ? document.body.innerText.slice(0, 800) : ''")
	if err == nil {
		if s, ok := result.(string); ok {
			text = s
		}
	}

	pageURL := p.URL()
	out := "url: " + pageURL
	if strings.TrimSpace(text) != "" {
		out += "\n" + strings.TrimSpace(text)
	}
	if len(out) > 1500 {
		out = out[:1500]
	}
	return out
}

var cookieConsentSelectors = []string{
	`button:has-text("Accept all")`,
	`button:has-text("Reject all")`,
	`button[aria-label*="Accept"]`,
	`#L2AGLb`,
	`button:has-text("Agree")`,
}

func dismissCookieConsent(p playwright.Page) {
	for _, sel := range cookieConsentSelectors {
		err := p.Click(sel, playwright.PageClickOptions{Timeout: playwright.Float(1500)})
		if err == nil {
			return
		}
	}
}

func newBrowserPage() (playwright.Browser, playwright.Page, func(), error) {
	var err error
	tempPW, err := playwright.Run()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("playwright init: %w", err)
	}

	tempBrowser, err := tempPW.Chromium.Launch(playwright.BrowserTypeLaunchOptions{Headless: playwright.Bool(true)})
	if err != nil {
		tempPW.Stop()
		return nil, nil, nil, fmt.Errorf("playwright launch: %w", err)
	}

	ctx, err := tempBrowser.NewContext(playwright.BrowserNewContextOptions{Locale: playwright.String("en-US")})
	if err != nil {
		tempBrowser.Close()
		tempPW.Stop()
		return nil, nil, nil, fmt.Errorf("playwright context: %w", err)
	}

	tempPage, err := ctx.NewPage()
	if err != nil {
		tempBrowser.Close()
		tempPW.Stop()
		return nil, nil, nil, fmt.Errorf("playwright page: %w", err)
	}

	cleanup := func() {
		tempBrowser.Close()
		tempPW.Stop()
	}

	return tempBrowser, tempPage, cleanup, nil
}

// ── BrowserOpenTool ───────────────────────────────────────────────────────────

type BrowserOpenTool struct{}

func (t *BrowserOpenTool) Name() string { return "browser_open" }

func (t *BrowserOpenTool) Execute(args map[string]any) Result {
	rawURL := strings.TrimSpace(getStr(args, "url"))
	if !isSSRFSafeURL(rawURL) {
		return R("error: URL is blocked (SSRF or file scheme)")
	}

	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return R("error: invalid URL")
	}

	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	if err := cmd.Start(); err != nil {
		return R("error: " + err.Error())
	}

	return R("opened: " + rawURL)
}

// ── BrowserReadTool ───────────────────────────────────────────────────────────

type BrowserReadTool struct{}

func (t *BrowserReadTool) Name() string { return "browser_read" }

func (t *BrowserReadTool) Execute(args map[string]any) Result {
	rawURL := strings.TrimSpace(getStr(args, "url"))
	if !isSSRFSafeURL(rawURL) {
		return R("error: URL is blocked (SSRF or file scheme)")
	}

	_, tempPage, cleanup, err := newBrowserPage()
	if err != nil {
		return R("error: playwright not available — run `playwright install chromium`")
	}
	defer cleanup()

	if _, err := tempPage.Goto(rawURL, playwright.PageGotoOptions{Timeout: playwright.Float(20000)}); err != nil {
		return R("error: " + err.Error())
	}

	dismissCookieConsent(tempPage)

	tempPage.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateNetworkidle,
		Timeout: playwright.Float(5000),
	})

	result, err := tempPage.Evaluate(`() => {
		const text = document.body.innerText.slice(0, 2000);
		const links = [...document.querySelectorAll('a[href]')]
			.map(a => ({text: (a.innerText || a.title || '').trim().slice(0, 80), url: a.href}))
			.filter(l => l.text && l.url && !l.url.startsWith('javascript') && !l.url.startsWith('mailto'))
			.slice(0, 40);
		return {text, links};
	}`)
	if err != nil {
		return R("error: " + err.Error())
	}

	data, ok := result.(map[string]interface{})
	if !ok {
		return R("error: unexpected page content")
	}

	var out string
	if t, ok := data["text"].(string); ok {
		out = t
	}

	if links, ok := data["links"].([]interface{}); ok && len(links) > 0 {
		out += "\n\nLinks:\n"
		for _, l := range links {
			if lm, ok := l.(map[string]interface{}); ok {
				txt := ""
				linkURL := ""
				if v, ok := lm["text"].(string); ok {
					txt = v
				}
				if v, ok := lm["url"].(string); ok {
					linkURL = v
				}
				if txt != "" && linkURL != "" {
					out += fmt.Sprintf("- %s: %s\n", txt, linkURL)
				}
			}
		}
	}

	if len(out) > 3000 {
		out = out[:3000]
	}
	return R(out)
}

// ── YouTubeSearchTool ─────────────────────────────────────────────────────────

type YouTubeSearchTool struct{}

func (t *YouTubeSearchTool) Name() string { return "youtube_search" }

func (t *YouTubeSearchTool) Execute(args map[string]any) Result {
	query := strings.TrimSpace(getStr(args, "query"))
	if query == "" {
		return R("error: query is required")
	}

	_, tempPage, cleanup, err := newBrowserPage()
	if err != nil {
		return R("error: playwright not available — run `playwright install chromium`")
	}
	defer cleanup()

	searchURL := "https://www.youtube.com/results?search_query=" + url.QueryEscape(query)
	if _, err := tempPage.Goto(searchURL, playwright.PageGotoOptions{Timeout: playwright.Float(20000)}); err != nil {
		return R("error: " + err.Error())
	}

	for _, sel := range cookieConsentSelectors {
		err := tempPage.Click(sel, playwright.PageClickOptions{Timeout: playwright.Float(2000)})
		if err == nil {
			break
		}
	}

	tempPage.WaitForSelector("ytd-video-renderer", playwright.PageWaitForSelectorOptions{Timeout: playwright.Float(8000)})

	result, err := tempPage.Evaluate(`() => {
		const items = document.querySelectorAll('ytd-video-renderer');
		return [...items].slice(0, 8).map(item => {
			const a = item.querySelector('a#video-title');
			const meta = item.querySelector('#metadata-line');
			return {
				title: (a?.textContent || '').trim(),
				url: a?.href || '',
				meta: (meta?.textContent || '').trim().replace(/\s+/g, ' ')
			};
		}).filter(r => r.title && r.url);
	}`)
	if err != nil {
		return R("error: " + err.Error())
	}

	items, ok := result.([]interface{})
	if !ok || len(items) == 0 {
		return R("no results")
	}

	var b strings.Builder
	for i, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		title := ""
		videoURL := ""
		meta := ""
		if v, ok := m["title"].(string); ok {
			title = v
		}
		if v, ok := m["url"].(string); ok {
			videoURL = v
		}
		if v, ok := m["meta"].(string); ok {
			meta = v
		}

		line := fmt.Sprintf("%d. %s", i+1, title)
		if meta != "" {
			line += " - " + meta
		}
		line += "\n   " + videoURL
		b.WriteString(line)
		if i < len(items)-1 {
			b.WriteString("\n")
		}
	}

	return R(b.String())
}

// ── BrowserScreenshotTool ─────────────────────────────────────────────────────

type BrowserScreenshotTool struct{}

func (t *BrowserScreenshotTool) Name() string { return "browser_screenshot" }

func (t *BrowserScreenshotTool) Execute(args map[string]any) Result {
	rawURL := strings.TrimSpace(getStr(args, "url"))
	if !isSSRFSafeURL(rawURL) {
		return R("error: URL is blocked (SSRF or file scheme)")
	}

	_, tempPage, cleanup, err := newBrowserPage()
	if err != nil {
		return R("error: playwright not available — run `playwright install chromium`")
	}
	defer cleanup()

	if _, err := tempPage.Goto(rawURL, playwright.PageGotoOptions{Timeout: playwright.Float(20000)}); err != nil {
		return R("error: " + err.Error())
	}

	dir := screenshotDir()
	cleanupOldScreenshots(dir, 5*time.Minute)

	name := filepath.Join(dir, fmt.Sprintf("browser_%s.png", time.Now().Format("20060102_150405")))
	if _, err := tempPage.Screenshot(playwright.PageScreenshotOptions{
		Path:     playwright.String(name),
		FullPage: playwright.Bool(true),
	}); err != nil {
		return R("error: " + err.Error())
	}

	return R("saved: " + name)
}

// ── BrowserClickTool ──────────────────────────────────────────────────────────

type BrowserClickTool struct{}

func (t *BrowserClickTool) Name() string { return "browser_click" }

func (t *BrowserClickTool) Execute(args map[string]any) Result {
	sel := strings.TrimSpace(getStr(args, "selector"))
	if sel == "" {
		return R("error: selector is required")
	}

	rawURL := strings.TrimSpace(getStr(args, "url"))
	if rawURL != "" && !isSSRFSafeURL(rawURL) {
		return R("error: URL is blocked (SSRF or file scheme)")
	}

	timeoutSec := clampInt(getInt(args, "timeout", 15), 1, 60)
	timeoutMs := float64(timeoutSec * 1000)

	p, err := getPage()
	if err != nil {
		return R("error: playwright not available — run `playwright install chromium`")
	}

	if rawURL != "" {
		if _, err := p.Goto(rawURL, playwright.PageGotoOptions{Timeout: &timeoutMs}); err != nil {
			return R("error: " + err.Error())
		}
	}

	if err := p.Click(pwSelector(sel), playwright.PageClickOptions{Timeout: &timeoutMs}); err != nil {
		return R("error: " + err.Error())
	}

	p.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateNetworkidle,
		Timeout: playwright.Float(5000),
	})

	snap := pageSnapshot(p)
	return R("clicked: " + sel + "\n" + snap)
}

// ── BrowserFillTool ───────────────────────────────────────────────────────────

type BrowserFillTool struct{}

func (t *BrowserFillTool) Name() string { return "browser_fill" }

func (t *BrowserFillTool) Execute(args map[string]any) Result {
	sel := strings.TrimSpace(getStr(args, "selector"))
	if sel == "" {
		return R("error: selector is required")
	}
	value := getStr(args, "value")

	rawURL := strings.TrimSpace(getStr(args, "url"))
	if rawURL != "" && !isSSRFSafeURL(rawURL) {
		return R("error: URL is blocked (SSRF or file scheme)")
	}

	submit := false
	if v, ok := args["submit"].(bool); ok {
		submit = v
	}

	timeoutSec := clampInt(getInt(args, "timeout", 15), 1, 60)
	timeoutMs := float64(timeoutSec * 1000)

	p, err := getPage()
	if err != nil {
		return R("error: playwright not available — run `playwright install chromium`")
	}

	if rawURL != "" {
		if _, err := p.Goto(rawURL, playwright.PageGotoOptions{Timeout: &timeoutMs}); err != nil {
			return R("error: " + err.Error())
		}
	}

	if err := p.Fill(pwSelector(sel), value, playwright.PageFillOptions{Timeout: &timeoutMs}); err != nil {
		return R("error: " + err.Error())
	}

	if submit {
		if err := p.Press(pwSelector(sel), "Enter"); err != nil {
			return R("error: " + err.Error())
		}
		p.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State:   playwright.LoadStateNetworkidle,
			Timeout: playwright.Float(5000),
		})
	}

	snap := pageSnapshot(p)
	return R("filled: " + sel + "\n" + snap)
}

// playwrightCheck returns true if playwright is installed.
func playwrightCheck() bool {
	_, err := os.Stat(filepath.Join(os.Getenv("LOCALAPPDATA"), "ms-playwright"))
	return err == nil
}
