package memory

import (
	"regexp"
	"strings"
)

type MemoryFact struct {
	Category   string
	Key        string
	Value      string
	Confidence float64
}

type trigger struct {
	pattern  *regexp.Regexp
	category string
	keyTmpl  string
}

var triggers = []trigger{
	{regexp.MustCompile(`(?i)(?:tech\s*stack|stack)["'\s:]*([^,.;]{1,60})`), "tech_stack", "stack"},
	{regexp.MustCompile(`(?i)(?:using|use|на\s*\w+\s*написано\s*на|использу[юем]|стек)["'\s:]*([^,.;]{1,60})`), "tech_stack", "stack"},
	{regexp.MustCompile(`(?i)(?:prefer|like|want|love|предпочитаю|люблю|хочу)["'\s:]*([^,.;]{1,60})`), "preference", "preference"},
	{regexp.MustCompile(`(?i)(?:style|стиль)["'\s:]*([^,.;]{1,60})`), "preference", "style"},
	{regexp.MustCompile(`(?i)(?:project|проект)["'\s:]*(?:name|название)?["'\s:]*([^,.;]{1,60})`), "project", "name"},
	{regexp.MustCompile(`(?i)(?:работаю\s+над|working\s+on)["'\s:]*([^,.;]{1,60})`), "project", "name"},
	{regexp.MustCompile(`(?i)(?:my\s+name\s+is|меня\s+зовут)["'\s:]*([^,.;]{1,60})`), "user", "name"},
	{regexp.MustCompile(`(?i)(?:я\s+|I["'\s]m\s+)([A-Z][a-z]{1,20})(?:\s|$|[,.])`), "user", "name"},
	{regexp.MustCompile(`(?i)(?:preferred\s+language|язык)["'\s:]*([^,.;]{1,30})`), "user", "preferred_language"},
}

var techKeywords = map[string]bool{
	"python": true, "go": true, "golang": true, "rust": true,
	"javascript": true, "typescript": true, "java": true, "kotlin": true,
	"swift": true, "c++": true, "c#": true, "ruby": true, "php": true,
	"react": true, "vue": true, "svelte": true, "angular": true,
	"nextjs": true, "nuxtjs": true, "django": true, "flask": true,
	"fastapi": true, "express": true, "rails": true,
	"sqlite": true, "postgres": true, "mysql": true, "mongodb": true,
	"redis": true, "docker": true, "kubernetes": true,
	"aws": true, "gcp": true, "azure": true,
	"linux": true, "windows": true, "macos": true,
	"bash": true, "powershell": true, "ollama": true,
	"openai": true, "openrouter": true, "anthropic": true,
	"tailwind": true, "shadcn": true, "bootstrap": true,
	"css": true, "html": true,
}

var fillerRe = regexp.MustCompile(`(?i)^(?:is|are|was|were|be|being|been|на|в|с|of|for|to|that|this|—|:|\s|\-|\.)+`)

func ExtractFacts(text string) []MemoryFact {
	if text == "" {
		return nil
	}

	var facts []MemoryFact
	seen := make(map[string]bool)

	for _, tr := range triggers {
		matches := tr.pattern.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			raw := strings.Trim(m[1], "\"' ")
			if raw == "" {
				continue
			}
			raw = cleanValue(raw)
			if raw == "" {
				continue
			}
			key := deriveKey(tr.keyTmpl, raw)
			seenKey := tr.category + ":" + key
			if seen[seenKey] {
				continue
			}
			seen[seenKey] = true
			conf := calcConfidence(m, text)
			facts = append(facts, MemoryFact{Category: tr.category, Key: key, Value: raw, Confidence: conf})
		}
	}

	facts = append(facts, extractTechKeywords(text, seen)...)
	return facts
}

func cleanValue(value string) string {
	v := strings.TrimSpace(value)
	v = fillerRe.ReplaceAllString(v, "")
	return strings.TrimSpace(v)
}

func deriveKey(tmpl, value string) string {
	if tmpl == "stack" {
		wordRe := regexp.MustCompile(`[a-zA-Z+#0-9]+`)
		words := wordRe.FindAllString(value, -1)
		for _, w := range words {
			lw := strings.ToLower(w)
			if techKeywords[lw] {
				return lw
			}
		}
		return "stack"
	}
	if tmpl == "name" {
		return "name"
	}
	return tmpl
}

func calcConfidence(match []string, fullText string) float64 {
	base := 0.7

	// Boost if match is at sentence start
	pos := strings.Index(fullText, match[0])
	if pos > 0 {
		ch := fullText[pos-1]
		if ch == '.' || ch == '!' || ch == '?' || ch == ';' || ch == '\n' {
			base += 0.15
		}
	} else if pos == 0 {
		base += 0.15
	}

	valLen := len(strings.TrimSpace(match[1]))
	if valLen > 40 {
		base -= 0.15
	}
	if valLen <= 15 {
		base += 0.1
	}

	if base < 0.1 {
		base = 0.1
	}
	if base > 1.0 {
		base = 1.0
	}

	return float64(int(base*100)) / 100.0
}

var techContextRe = regexp.MustCompile(`(?i)(?:tech|stack|using|build|backend|frontend|framework|language|библиотек|фреймворк|стек|язык)`)

func extractTechKeywords(text string, seen map[string]bool) []MemoryFact {
	if !techContextRe.MatchString(text) {
		return nil
	}

	var facts []MemoryFact
	for kw := range techKeywords {
		seenKey := "tech_stack:" + kw
		if seen[seenKey] {
			continue
		}
		pat := regexp.MustCompile(`\b` + regexp.QuoteMeta(kw) + `\b`)
		if pat.MatchString(strings.ToLower(text)) {
			seen[seenKey] = true
			facts = append(facts, MemoryFact{Category: "tech_stack", Key: kw, Value: kw, Confidence: 0.5})
		}
	}
	return facts
}
