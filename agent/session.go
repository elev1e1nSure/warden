package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/elev1e1nSure/warden/agent/memory"
	"github.com/elev1e1nSure/warden/agent/skills"
	"github.com/elev1e1nSure/warden/internal/client"
)

var emojiRe = regexp.MustCompile(`[\x{1f1e6}-\x{1f1ff}\x{1f300}-\x{1faff}\x{2700}-\x{27bf}\x{2600}-\x{26ff}]+`)

func cleanVisibleText(text string) string {
	return emojiRe.ReplaceAllString(text, "")
}

func hasImages(messages []map[string]any) bool {
	for _, msg := range messages {
		if _, ok := msg["images"]; ok {
			return true
		}
		if contentList, ok := msg["content"].([]any); ok {
			for _, part := range contentList {
				if m, ok := part.(map[string]any); ok && m["type"] == "image_url" {
					return true
				}
			}
		}
	}
	return false
}

func stripImages(messages []map[string]any) []map[string]any {
	note := " [note: attached image not sent — model cannot view images]"
	result := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		m := make(map[string]any)
		for k, v := range msg {
			if k != "images" {
				m[k] = v
			}
		}
		if _, ok := msg["images"]; ok {
			contentStr, _ := msg["content"].(string)
			m["content"] = contentStr + note
			result = append(result, m)
		} else if contentList, ok := msg["content"].([]any); ok {
			var textParts []string
			for _, part := range contentList {
				if pm, ok := part.(map[string]any); ok {
					if pm["type"] == "text" {
						textParts = append(textParts, fmt.Sprint(pm["text"]))
					}
				}
			}
			m["content"] = strings.Join(textParts, " ") + note
			result = append(result, m)
		} else {
			result = append(result, msg)
		}
	}
	return result
}

func isVisionError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	keywords := []string{
		"image", "vision", "multimodal",
		"does not support", "unsupported content",
		"image_url", "not support image",
	}
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

type ChatSession struct {
	Model               string
	History             []map[string]any
	Client              LLMClient
	ConfirmationManager *ConfirmationManager
	QuestionManager     *QuestionManager
	MemoryStore         *memory.MemoryStore
	SessionID           string
	TokenCount          int
	TokenLimit          int
	mu                  sync.Mutex
	cancelled           int32 // atomic bool: 0 is false, 1 is true
}

func NewChatSession(
	model string,
	llmClient LLMClient,
	confirmMgr *ConfirmationManager,
	questionMgr *QuestionManager,
	memStore *memory.MemoryStore,
) *ChatSession {
	return &ChatSession{
		Model:               model,
		Client:              llmClient,
		ConfirmationManager: confirmMgr,
		QuestionManager:     questionMgr,
		MemoryStore:         memStore,
		SessionID:           newID(),
		TokenLimit:          GuessContextLimit(model),
	}
}

func (s *ChatSession) Cancel() {
	atomic.StoreInt32(&s.cancelled, 1)
}

func (s *ChatSession) ResetCancellation() {
	atomic.StoreInt32(&s.cancelled, 0)
}

func (s *ChatSession) IsCancelled() bool {
	return atomic.LoadInt32(&s.cancelled) == 1
}

func (s *ChatSession) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.MemoryStore != nil {
		memory.Finalize(s.MemoryStore, s.SessionID)
	}
	s.History = nil
	s.TokenCount = 0
	s.SessionID = newID()
}

func (s *ChatSession) EstimateTokens() int {
	total := 0
	for _, msg := range s.History {
		content := msg["content"]
		if str, ok := content.(string); ok {
			total += len(str) / 4
		} else if list, ok := content.([]any); ok {
			for _, part := range list {
				if pm, ok := part.(map[string]any); ok {
					textStr, _ := pm["text"].(string)
					total += len(textStr) / 4
				}
			}
		}
	}
	if total < 0 {
		return 0
	}
	return total
}

func (s *ChatSession) Compact(ctx context.Context) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.History) < 2 {
		return map[string]any{
			"summary":       "nothing to compact",
			"tokens_before": s.TokenCount,
			"tokens_after":  s.TokenCount,
		}
	}

	tokensBefore := s.EstimateTokens()
	system := BuildSystem(s.Model)
	messages := make([]map[string]any, 0, len(s.History)+2)
	messages = append(messages, map[string]any{"role": "system", "content": system})
	messages = append(messages, s.History...)
	messages = append(messages, map[string]any{"role": "user", "content": CompactPrompt})

	ch, err := s.Client.Chat(ctx, s.Model, messages, nil)
	if err != nil {
		return map[string]any{
			"summary":       "error: " + err.Error(),
			"tokens_before": tokensBefore,
			"tokens_after":  tokensBefore,
		}
	}

	var summaryBuilder strings.Builder
	for chunk := range ch {
		if chunk.Content != "" {
			summaryBuilder.WriteString(chunk.Content)
		}
	}
	summary := summaryBuilder.String()

	var tail []map[string]any
	for idx := len(s.History) - 1; idx >= 0; idx-- {
		msg := s.History[idx]
		if msg["role"] == "assistant" {
			if tcList, ok := msg["tool_calls"].([]any); ok && len(tcList) > 0 {
				pendingIDs := make(map[string]bool)
				for _, tc := range tcList {
					if tcMap, ok := tc.(map[string]any); ok {
						if id, ok := tcMap["id"].(string); ok {
							pendingIDs[id] = true
						}
					}
				}
				resolvedIDs := make(map[string]bool)
				for laterIdx := idx + 1; laterIdx < len(s.History); laterIdx++ {
					later := s.History[laterIdx]
					if later["role"] == "tool" {
						if id, ok := later["tool_call_id"].(string); ok {
							resolvedIDs[id] = true
						}
					}
				}
				hasUnresolved := false
				for id := range pendingIDs {
					if !resolvedIDs[id] {
						hasUnresolved = true
						break
					}
				}
				if hasUnresolved {
					tail = s.History[idx:]
				}
				break
			}
		}
		if msg["role"] == "tool" {
			continue
		}
		break
	}

	s.History = make([]map[string]any, 0, len(tail)+2)
	s.History = append(s.History, map[string]any{"role": "user", "content": "[Conversation summary]"})
	s.History = append(s.History, map[string]any{"role": "assistant", "content": summary})
	s.History = append(s.History, tail...)
	s.TokenCount = s.EstimateTokens()

	return map[string]any{
		"summary":       summary,
		"tokens_before": tokensBefore,
		"tokens_after":  s.TokenCount,
	}
}

func (s *ChatSession) AddUser(text string) {
	s.History = append(s.History, map[string]any{"role": "user", "content": text})
	if s.MemoryStore != nil && s.MemoryStore.GetEnabled() {
		facts := memory.ExtractFacts(text)
		for _, fact := range facts {
			s.MemoryStore.UpsertEntry(s.SessionID, fact.Category, fact.Key, fact.Value, fact.Confidence, "")
		}
	}
}

func (s *ChatSession) AddAssistant(text string, toolCalls []ToolCall) {
	msg := map[string]any{
		"role":    "assistant",
		"content": text,
	}
	if len(toolCalls) > 0 {
		tcs := make([]map[string]any, len(toolCalls))
		for i, tc := range toolCalls {
			tcs[i] = map[string]any{
				"id":   tc.ID,
				"type": tc.Type,
				"function": map[string]any{
					"name":      tc.Function.Name,
					"arguments": tc.Function.Arguments,
				},
			}
		}
		msg["tool_calls"] = tcs
	}
	s.History = append(s.History, msg)
}

func (s *ChatSession) AddToolResult(toolName string, result string, toolCallID string) {
	entry := map[string]any{
		"role":    "tool",
		"content": result,
		"name":    toolName,
	}
	if toolCallID != "" {
		entry["tool_call_id"] = toolCallID
	}
	s.History = append(s.History, entry)
}

func (s *ChatSession) callLLM(
	ctx context.Context,
	messages []map[string]any,
	ch chan<- client.Event,
) (string, []ToolCall, int, error) {
	toolsList := Definitions()
	chunkCh, err := s.Client.Chat(ctx, s.Model, messages, toolsList)
	if err != nil {
		if isVisionError(err) && hasImages(messages) {
			stripped := stripImages(messages)
			chunkCh, err = s.Client.Chat(ctx, s.Model, stripped, toolsList)
			if err != nil {
				ch <- client.EventToken{Text: "\nconnection error: " + err.Error()}
				return "", nil, 0, err
			}
		} else {
			ch <- client.EventToken{Text: "\nconnection error: " + err.Error()}
			return "", nil, 0, err
		}
	}

	var fullContent strings.Builder
	var collectedToolCalls []ToolCall
	inThink := false
	usageTokens := 0

	for chunk := range chunkCh {
		if s.IsCancelled() {
			break
		}
		if chunk.UsageTokens > 0 {
			usageTokens = chunk.UsageTokens
			continue
		}
		if len(chunk.ToolCalls) > 0 {
			collectedToolCalls = append(collectedToolCalls, chunk.ToolCalls...)
		}

		if chunk.Thinking != "" {
			ch <- client.EventThink{Text: chunk.Thinking}
		} else if chunk.Reasoning != "" {
			ch <- client.EventThink{Text: chunk.Reasoning}
		} else if len(chunk.ReasoningDetails) > 0 {
			reasoningText := reasoningDetailsText(chunk.ReasoningDetails)
			if reasoningText != "" {
				ch <- client.EventThink{Text: reasoningText}
			}
		}

		if chunk.Content == "" {
			continue
		}

		textChunk := chunk.Content
		for textChunk != "" {
			if !inThink {
				idx := strings.Index(textChunk, "<think>")
				if idx == -1 {
					clean := cleanVisibleText(textChunk)
					if clean != "" {
						ch <- client.EventToken{Text: clean}
						fullContent.WriteString(clean)
					}
					textChunk = ""
				} else {
					if idx > 0 {
						clean := cleanVisibleText(textChunk[:idx])
						if clean != "" {
							ch <- client.EventToken{Text: clean}
							fullContent.WriteString(clean)
						}
					}
					textChunk = textChunk[idx+7:]
					inThink = true
				}
			} else {
				idx := strings.Index(textChunk, "</think>")
				if idx == -1 {
					ch <- client.EventThink{Text: textChunk}
					textChunk = ""
				} else {
					if idx > 0 {
						ch <- client.EventThink{Text: textChunk[:idx]}
					}
					textChunk = textChunk[idx+8:]
					inThink = false
				}
			}
		}
	}

	return fullContent.String(), collectedToolCalls, usageTokens, nil
}

func reasoningDetailsText(details []map[string]any) string {
	var parts []string
	for _, item := range details {
		for _, key := range []string{"text", "summary", "content"} {
			if val, ok := item[key].(string); ok && strings.TrimSpace(val) != "" {
				parts = append(parts, val)
				break
			}
		}
	}
	return strings.Join(parts, "")
}

func (s *ChatSession) Stream(
	text string,
	autoMode bool,
	skillName string,
	skillArgs string,
) <-chan client.Event {
	ch := make(chan client.Event, 64)
	go func() {
		defer close(ch)
		s.streamLoop(ch, text, autoMode, skillName, skillArgs)
	}()
	return ch
}

func (s *ChatSession) streamLoop(
	ch chan<- client.Event,
	text string,
	autoMode bool,
	skillName string,
	skillArgs string,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var turnContext []map[string]any
	if skillName != "" {
		skill := skills.FindSkill(skillName)
		if skill == nil {
			ch <- client.EventToken{Text: "skill not found: " + skillName}
			return
		}
		turnContext = skillContextMessages(skill, skillArgs)
	}

	historyInsertAt := len(s.History) + 1
	s.AddUser(text)
	s.ResetCancellation()
	iterCount := 0

	// Set session on todowrite tool if it implements SessionSettable
	todowriteTool := Registry()["todowrite"]
	if settable, ok := todowriteTool.(interface{ SetSession(id string) }); ok {
		settable.SetSession(s.SessionID)
	}

	for iterCount < 20 { // MAX_ITER is 20
		iterCount++
		if s.IsCancelled() {
			break
		}

		ch <- client.EventWardenStart{}

		system := BuildSystem(s.Model)
		if s.MemoryStore != nil && s.MemoryStore.GetEnabled() {
			memCtx := s.MemoryStore.GetContextText(s.SessionID, 0.5, 30)
			if memCtx != "" {
				system = memCtx + "\n\n" + system
			}
		}

		var history []map[string]any
		if len(turnContext) > 0 {
			history = make([]map[string]any, 0, len(s.History)+len(turnContext))
			history = append(history, s.History[:historyInsertAt]...)
			history = append(history, turnContext...)
			history = append(history, s.History[historyInsertAt:]...)
		} else {
			history = s.History
		}

		messages := make([]map[string]any, 0, len(history)+1)
		messages = append(messages, map[string]any{"role": "system", "content": system})
		messages = append(messages, history...)

		ctx := context.Background()
		content, toolCalls, usage, err := s.callLLM(ctx, messages, ch)
		if err != nil {
			break
		}
		if s.IsCancelled() {
			break
		}

		s.AddAssistant(content, toolCalls)
		if usage > 0 {
			s.TokenCount = usage
		} else {
			s.TokenCount = s.EstimateTokens()
		}

		if len(toolCalls) == 0 {
			break
		}

		// Execute tool calls
		for _, tc := range toolCalls {
			if s.IsCancelled() {
				break
			}
			executeToolCall(
				ctx,
				s.Client,
				s.Model,
				tc,
				autoMode,
				&s.History,
				s.ConfirmationManager,
				s.QuestionManager,
				s.AddToolResult,
				ch,
			)
		}
	}

	if s.IsCancelled() {
		ch <- client.EventToken{Text: "\n[interrupted]"}
	} else if iterCount >= 20 {
		ch <- client.EventToken{Text: "\n[iteration limit reached]"}
	}
}
