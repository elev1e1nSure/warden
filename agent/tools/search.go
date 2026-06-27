package tools

import (
	"crypto/tls"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// --- GoogleSearchTool ---

type GoogleSearchTool struct{}

func (t *GoogleSearchTool) Name() string { return "google_search" }

func (t *GoogleSearchTool) Execute(args map[string]any) Result {
	return R("error: google_search requires duckduckgo-search Python package — not available in Go")
}

// --- WebFetchTool ---

type WebFetchTool struct{}

func (t *WebFetchTool) Name() string { return "webfetch" }

func (t *WebFetchTool) Execute(args map[string]any) Result {
	urlStr := getStr(args, "url")
	fmt_ := getStr(args, "format")
	if fmt_ == "" {
		fmt_ = "markdown"
	}
	timeout := clampInt(getInt(args, "timeout", 30), 1, 120)
	verify := true
	if v, ok := args["verify_ssl"].(bool); !ok {
		verify = v
	}

	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		return R("error: URL must start with http:// or https://")
	}
	if !isSSRFSafeURL(urlStr) {
		return R("error: URL is blocked (SSRF or file scheme)")
	}

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return R("error: " + err.Error())
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	acceptMap := map[string]string{
		"markdown": "text/markdown;q=1.0, text/html;q=0.8, text/plain;q=0.7, */*;q=0.1",
		"text":     "text/plain;q=1.0, text/html;q=0.8, */*;q=0.1",
		"html":     "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8",
	}
	if a, ok := acceptMap[fmt_]; ok {
		req.Header.Set("Accept", a)
	}

	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: !verify},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "certificate") {
			return R("error: SSL certificate verification failed (set verify_ssl=false to bypass)")
		}
		return R("error: " + err.Error())
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 5_000_000))
	return processWebContent(string(raw), resp.Header.Get("Content-Type"), fmt_)
}

func processWebContent(content, contentType, format string) Result {
	decoded := string(content)
	if len(decoded) > 10000 {
		decoded = decoded[:10000]
	}

	isHTML := strings.Contains(contentType, "text/html") || strings.Contains(contentType, "html")
	if format == "markdown" && isHTML {
		return R(htmlToMarkdown(decoded))
	}
	if format == "text" && isHTML {
		return R(htmlToText(decoded))
	}
	return R(decoded)
}

// --- HTML helpers ---

var (
	scriptRe = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	styleRe  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	tagRe    = regexp.MustCompile(`<[^>]+>`)
	spaceRe  = regexp.MustCompile(`\s+`)
)

func htmlToText(html string) string {
	text := scriptRe.ReplaceAllString(html, "")
	text = styleRe.ReplaceAllString(text, "")
	text = tagRe.ReplaceAllString(text, "")
	text = spaceRe.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func htmlToMarkdown(html string) string {
	// Simple HTML→text conversion (no external lib)
	return htmlToText(html)
}
