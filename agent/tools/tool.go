package tools

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*[mGKHFJABCDsu]|\x1b\\][^\x07]*\x07|\x1b=|\x1b>")

type Result struct {
	Text string
	Diff string
}

func R(text string) Result { return Result{Text: text} }
func RD(text, diff string) Result { return Result{Text: text, Diff: diff} }

func (r Result) String() string { return r.Text }

type Tool interface {
	Name() string
	Execute(args map[string]any) Result
}

func diffStats(old, new string) string {
	added, removed := 0, 0
	oldLines := strings.Split(old, "\n")
	newLines := strings.Split(new, "\n")
	lcs := lcsLines(oldLines, newLines)
	i, j := 0, 0
	for _, l := range lcs {
		for i < len(oldLines) && oldLines[i] != l {
			removed++
			i++
		}
		for j < len(newLines) && newLines[j] != l {
			added++
			j++
		}
		i++
		j++
	}
	removed += len(oldLines) - i
	added += len(newLines) - j
	if added == 0 && removed == 0 {
		return ""
	}
	return fmt.Sprintf("+%d -%d", added, removed)
}

func diffFull(old, new, path string) string {
	var b strings.Builder
	oldLines := strings.Split(old, "\n")
	newLines := strings.Split(new, "\n")
	lcs := lcsLines(oldLines, newLines)

	b.WriteString(fmt.Sprintf("--- a/%s\n", path))
	b.WriteString(fmt.Sprintf("+++ b/%s\n", path))

	var hdrOld, hdrNew int
	var hunks []string

	flush := func() {
		if len(hunks) == 0 {
			return
		}
		b.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", hdrOld, len(oldLines), hdrNew, len(newLines)))
		for _, h := range hunks {
			b.WriteString(h)
			b.WriteString("\n")
		}
		hunks = nil
	}

	oi, ni := 0, 0
	for _, l := range lcs {
		for oi < len(oldLines) && oldLines[oi] != l {
			if len(hunks) == 0 {
				hdrOld = oi + 1
				hdrNew = ni + 1
			}
			hunks = append(hunks, "-"+oldLines[oi])
			oi++
		}
		for ni < len(newLines) && newLines[ni] != l {
			if len(hunks) == 0 {
				hdrOld = oi + 1
				hdrNew = ni + 1
			}
			hunks = append(hunks, "+"+newLines[ni])
			ni++
		}
		if len(hunks) > 0 {
			hunks = append(hunks, " "+l)
		}
		if len(hunks) >= 10 {
			flush()
		}
		oi++
		ni++
	}
	for oi < len(oldLines) {
		hunks = append(hunks, "-"+oldLines[oi])
		oi++
	}
	for ni < len(newLines) {
		hunks = append(hunks, "+"+newLines[ni])
		ni++
	}
	flush()

	return strings.TrimRight(b.String(), "\n")
}

func lcsLines(a, b []string) []string {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				if dp[i-1][j] > dp[i][j-1] {
					dp[i][j] = dp[i-1][j]
				} else {
					dp[i][j] = dp[i][j-1]
				}
			}
		}
	}
	var result []string
	i, j := m, n
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			result = append(result, a[i-1])
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return result
}

func clean(text string) string {
	text = ansiRe.ReplaceAllString(text, "")
	lines := strings.Split(text, "\n")
	var out []string
	for _, line := range lines {
		parts := strings.Split(line, "\r")
		cleaned := strings.TrimRight(parts[len(parts)-1], " \t\r")
		if cleaned != "" {
			out = append(out, cleaned)
		}
	}
	return strings.Join(out, "\n")
}

func inCwd(path string) bool {
	target, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	target = filepath.Clean(target)
	cwd, err := os.Getwd()
	if err != nil {
		return false
	}
	cwd = filepath.Clean(cwd)
	rel, err := filepath.Rel(cwd, target)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

func parseArgs(arguments any) map[string]any {
	switch v := arguments.(type) {
	case map[string]any:
		return v
	case string:
		var result map[string]any
		if json.Unmarshal([]byte(v), &result) == nil {
			return result
		}
	}
	return map[string]any{}
}

func isSSRFSafeURL(rawURL string) bool {
	if rawURL == "" {
		return false
	}
	rawURL = strings.TrimSpace(rawURL)
	lower := strings.ToLower(rawURL)
	if strings.HasPrefix(lower, "file:") || strings.Contains(lower, "file://") {
		return false
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if parsed.Scheme == "" {
		parsed, err = url.Parse("http://" + rawURL)
		if err != nil {
			return false
		}
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme == "file" {
		return false
	}
	hostname := parsed.Hostname()
	if hostname == "" {
		if scheme == "http" || scheme == "https" {
			return false
		}
		return true
	}
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	if hostname == "localhost" || hostname == "localhost." || hostname == "loopback" || hostname == "loopback." {
		return false
	}
	ip := net.ParseIP(hostname)
	if ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
			return false
		}
	}
	// DNS resolve check
	addrs, err := net.DefaultResolver.LookupHost(nil, hostname)
	if err == nil {
		for _, a := range addrs {
			ip := net.ParseIP(a)
			if ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
				return false
			}
		}
	}
	return true
}
