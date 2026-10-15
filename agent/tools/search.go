package tools

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// --- GoogleSearchTool ---

type GoogleSearchTool struct{}

func (t *GoogleSearchTool) Name() string { return "google_search" }

func (t *GoogleSearchTool) Execute(args map[string]any) Result {
	query := strings.TrimSpace(getStr(args, "query"))
	if query == "" {
		return R("error: query is required")
	}
	numResults := clampInt(getInt(args, "num_results", 8), 1, 20)
	timeout := clampInt(getInt(args, "timeout", 15), 5, 60)

	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	data := url.Values{"q": {query}}

	resp, err := client.PostForm("https://lite.duckduckgo.com/lite/", data)
	if err != nil {
		return R("error: search request failed: " + err.Error())
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 500_000))
	body := string(raw)

	results := parseDuckDuckGoResults(body, numResults)
	if len(results) == 0 {
		return R("no results")
	}

	lines := make([]string, 0, len(results)+1)
	lines = append(lines, fmt.Sprintf("search results for \"%s\":", query))
	for i, r := range results {
		lines = append(lines, fmt.Sprintf("\n%d. %s", i+1, r.title))
		if r.snippet != "" {
			lines = append(lines, "   "+r.snippet)
		}
		lines = append(lines, "   "+r.url)
	}

	return R(strings.Join(lines, ""))
}

type ddgResult struct {
	title   string
	url     string
	snippet string
}

func parseDuckDuckGoResults(html string, limit int) []ddgResult {
	var results []ddgResult
	// DuckDuckGo Lite returns tables: class="result-link" for links, class="result-snippet" for snippets
	linkRe := regexp.MustCompile(`<a[^>]+class="result-link"[^>]*href="([^"]*)"[^>]*>(.*?)</a>`)
	snippetRe := regexp.MustCompile(`<td[^>]+class="result-snippet"[^>]*>(.*?)</td>`)

	linkMatches := linkRe.FindAllStringSubmatch(html, -1)
	snippetMatches := snippetRe.FindAllStringSubmatch(html, -1)

	for i, m := range linkMatches {
		if len(m) < 3 || i >= limit {
			break
		}
		r := ddgResult{
			url:   htmlUnescape(m[1]),
			title: htmlUnescape(stripTags(m[2])),
		}
		if i < len(snippetMatches) {
			r.snippet = htmlUnescape(stripTags(snippetMatches[i][1]))
		}
		results = append(results, r)
	}
	return results
}

func htmlUnescape(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#x27;", "'")
	s = strings.ReplaceAll(s, "&#39;", "'")
	return s
}

func stripTags(s string) string {
	tagRe := regexp.MustCompile(`<[^>]+>`)
	return strings.TrimSpace(tagRe.ReplaceAllString(s, ""))
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
