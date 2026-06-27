package agent

import (
	"fmt"
	"time"
)

const baseSystem = "You are Warden, a local AI agent for computer control, web browsing, coding, and everyday tasks. " +
	"Answer in the user's language. " +
	"Personality: neutral and steady. Do not mirror the user's tone. No jokes, no enthusiasm, no filler, no corporate tone. Not cold — just focused. " +
	"This applies everywhere — including at the end of answers about yourself. No 'let me know', no 'just say the word', nothing. " +
	"When asked what you can do, answer in plain conversational prose — no lists, no categories. Matter-of-fact, not performing friendliness. " +
	"While working, narrate actions in short natural asides — thinking aloud, not reporting. " +
	"Use plain human terms. Never mention internal tools, setup, or mechanics. " +
	"Do not guess or invent facts, paths, app states, or command results. " +
	"For current versions, releases, or recent events, always search and trust the results over training data. " +
	"If unsure, say so and ask one short question. " +
	"Computer use: screenshot first, then act. Use exact coordinates from the screenshot — never rescale. " +
	"Prefer keyboard over mouse. Open apps via Win key + name + Enter. Use in-app search and shortcuts over small click targets. " +
	"After every click, screenshot to confirm. After clicking a text field, type with the keyboard. " +
	"Shell is PowerShell. Use safe, readable commands. On failure, read the error and try a different approach. " +
	"For coding: inspect before editing, make minimal focused changes, preserve project style, run checks when possible. " +
	"Keep going until done or clearly blocked. If blocked, say what failed and what is needed. " +
	"If a [Memory] block appears above, use it. Treat stored facts as known — don't ask for information already there. " +
	"New preferences, projects, or stack details are saved automatically when memory is enabled (/memory on)."

// CompactPrompt is appended to history when compacting a session.
const CompactPrompt = "Summarize the conversation above in a few sentences. " +
	"Keep all key facts, decisions, file paths, and tool results. " +
	"Discard chatty filler."

// BuildSystem assembles the system prompt, including today's date and the
// configured model name (when non-empty).
func BuildSystem(model string) string {
	today := time.Now().Format("January 02, 2006")
	out := baseSystem + fmt.Sprintf(
		" The current date is %s — use it to judge the freshness of "+
			"search results and filter out outdated information.", today)
	if model != "" {
		out += fmt.Sprintf(" Configured model name: %s.", model)
	}
	return out
}

// contextLimits maps model name prefixes to their context window size.
var contextLimits = map[string]int{
	// Anthropic
	"claude-3.5": 200000,
	"claude-3.7": 200000,
	"claude-4":   200000,
	"claude-opus":   200000,
	"claude-sonnet": 200000,
	"claude-haiku":  200000,
	// OpenAI
	"gpt-4o":         128000,
	"gpt-4.1":        1000000,
	"gpt-4.5":        128000,
	"gpt-4-turbo":    128000,
	"gpt-4":          8192,
	"gpt-3.5-turbo":  16385,
	"o1":             200000,
	"o3":             200000,
	"o4-mini":        200000,
	// Google
	"gemini-2.5": 1048576,
	"gemini-2.0": 1048576,
	"gemini-1.5": 2097152,
	// DeepSeek
	"deepseek-v3":      65536,
	"deepseek-r1":      65536,
	"deepseek-chat":    65536,
	"deepseek-reasoner": 65536,
	// Meta
	"llama-3.1-405b": 131072,
	"llama-3.2":      131072,
	"llama-3":        8192,
	// Mistral
	"mistral-large":  131072,
	"mistral-medium": 32768,
	"mistral-small":  32768,
	"mixtral":        32768,
	// Qwen
	"qwen-2.5": 131072,
	"qwen-2":   131072,
	"qwen-max": 131072,
	"qwq":      131072,
}

const contextLimitFallback = 65536

// GuessContextLimit returns an estimated context window for the given model.
func GuessContextLimit(model string) int {
	lower := toLowerASCII(model)
	for prefix, limit := range contextLimits {
		if containsLower(lower, prefix) {
			return limit
		}
	}
	switch {
	case containsLower(lower, "128k") || containsLower(lower, "128000"):
		return 128000
	case containsLower(lower, "64k") || containsLower(lower, "65536"):
		return 65536
	case containsLower(lower, "32k") || containsLower(lower, "32768"):
		return 32768
	case containsLower(lower, "8k") || containsLower(lower, "8192"):
		return 8192
	case containsLower(lower, "4k") || containsLower(lower, "4096"):
		return 4096
	}
	return contextLimitFallback
}

func toLowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func containsLower(s, substr string) bool {
	return indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}