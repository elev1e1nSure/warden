package tools

import (
	"io"
	"net/http"
	"strings"
	"time"
)

var validMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true,
	"DELETE": true, "HEAD": true, "OPTIONS": true,
}

type HttpRequestTool struct{}

func (t *HttpRequestTool) Name() string { return "http_request" }

func (t *HttpRequestTool) Execute(args map[string]any) Result {
	urlStr := strings.TrimSpace(getStr(args, "url"))
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		return R("error: URL must start with http:// or https://")
	}
	if !isSSRFSafeURL(urlStr) {
		return R("error: URL is blocked (SSRF or file scheme)")
	}
	method := strings.ToUpper(strings.TrimSpace(getStr(args, "method")))
	if method == "" {
		method = "GET"
	}
	if !validMethods[method] {
		return R("error: unsupported method '" + method + "'")
	}
	timeout := clampInt(getInt(args, "timeout", 30), 1, 120)

	var body io.Reader
	if b, ok := args["body"].(string); ok && b != "" {
		body = strings.NewReader(b)
	}

	req, err := http.NewRequest(method, urlStr, body)
	if err != nil {
		return R("error: " + err.Error())
	}

	if h, ok := args["headers"].(map[string]any); ok {
		for k, v := range h {
			req.Header.Set(k, fmtAny(v))
		}
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "warden/1.0")
	}

	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return R("error: " + err.Error())
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1_000_000))
	if err != nil {
		return R("error: " + err.Error())
	}
	text := string(raw)
	if resp.StatusCode >= 400 {
		return R("error: HTTP " + resp.Status + "\n" + trunc(text, 10000))
	}
	head := "HTTP " + resp.Status
	if text != "" {
		return R(head + "\n" + trunc(text, 10000))
	}
	return R(head)
}
