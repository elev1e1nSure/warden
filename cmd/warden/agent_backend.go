package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/elev1e1nSure/warden/agent"
	"github.com/elev1e1nSure/warden/agent/memory"
	"github.com/elev1e1nSure/warden/agent/skills"
	"github.com/elev1e1nSure/warden/internal/client"
)

type AgentBackend struct {
	model               string
	provider            string
	apiURL              string
	apiKey              string
	autoMode            bool
	chat                *agent.ChatSession
	confirmationManager *agent.ConfirmationManager
	questionManager     *agent.QuestionManager
	memoryStore         *memory.MemoryStore
}

func NewAgentBackend(model, apiURL, apiKey string) *AgentBackend {
	store := memory.NewMemoryStore("")
	store.Init()
	confirmMgr := agent.NewConfirmationManager()
	questionMgr := agent.NewQuestionManager()
	b := &AgentBackend{
		model:               model,
		apiURL:              apiURL,
		apiKey:              apiKey,
		confirmationManager: confirmMgr,
		questionManager:     questionMgr,
		memoryStore:         store,
	}
	if model != "" {
		var llm agent.LLMClient
		if apiURL != "" {
			llm = agent.NewOpenAIClient(apiURL, apiKey)
			b.provider = "openrouter"
		} else {
			llm = agent.NewOllamaClient("")
			b.provider = "ollama"
		}
		b.chat = agent.NewChatSession(model, llm, confirmMgr, questionMgr, store)
	}
	return b
}

func (b *AgentBackend) StreamChat(payload map[string]string) <-chan client.Event {
	if b.chat == nil {
		ch := make(chan client.Event, 1)
		ch <- client.EventError{Text: "not connected — run /connect to get started"}
		close(ch)
		return ch
	}
	text := payload["text"]
	skill := payload["skill"]
	args := payload["args"]
	return b.chat.Stream(text, b.autoMode, skill, args)
}

func (b *AgentBackend) Interrupt() error {
	if b.chat != nil {
		b.chat.Cancel()
	}
	return nil
}

func (b *AgentBackend) ResetSession() error {
	b.confirmationManager.CancelAll()
	b.questionManager.CancelAll()
	if b.chat != nil {
		b.chat.Reset()
	}
	return nil
}

func (b *AgentBackend) SendQuestion(id string, answers [][]string) error {
	b.questionManager.Resolve(id, answers)
	return nil
}

func (b *AgentBackend) SendConfirm(id string, ok bool) error {
	b.confirmationManager.Resolve(id, ok)
	return nil
}

func (b *AgentBackend) SetMode(auto bool) error {
	b.autoMode = auto
	return nil
}

func (b *AgentBackend) GetStatus() (*client.StatusResult, error) {
	cwd, _ := os.Getwd()
	count := 0
	limit := 0
	if b.chat != nil {
		count = b.chat.TokenCount
		limit = b.chat.TokenLimit
	}
	return &client.StatusResult{
		Model:      b.model,
		Mode:       b.modeStr(),
		CWD:        cwd,
		TokenCount: count,
		TokenLimit: limit,
	}, nil
}

func (b *AgentBackend) modeStr() string {
	if b.autoMode {
		return "auto"
	}
	return "ask"
}

func (b *AgentBackend) Compact() (*client.CompactResult, error) {
	if b.chat == nil {
		return nil, fmt.Errorf("not connected")
	}
	res := b.chat.Compact(context.Background())
	summary, _ := res["summary"].(string)
	tBefore, _ := res["tokens_before"].(int)
	tAfter, _ := res["tokens_after"].(int)
	if strings.HasPrefix(summary, "error:") {
		return nil, fmt.Errorf("%s", summary)
	}
	return &client.CompactResult{
		Summary:      summary,
		TokensBefore: tBefore,
		TokensAfter:  tAfter,
	}, nil
}

func (b *AgentBackend) SetMemoryState(enabled bool) error {
	return b.memoryStore.SetEnabled(enabled)
}

func (b *AgentBackend) ClearMemory() (int, error) {
	return b.memoryStore.ClearEntries("")
}

func (b *AgentBackend) GetMemoryState() (*client.MemoryState, error) {
	stats := b.memoryStore.GetStats()
	return &client.MemoryState{
		Enabled:   stats.Enabled,
		Entries:   stats.Entries,
		Snapshots: stats.Snapshots,
		DBSize:    int(stats.DBSize),
	}, nil
}

func (b *AgentBackend) ListModels() ([]string, string, error) {
	if b.chat == nil {
		return nil, "", fmt.Errorf("not connected")
	}
	models, err := b.chat.Client.ListModels(context.Background())
	if err != nil {
		return nil, "", err
	}
	return models, b.model, nil
}

func (b *AgentBackend) SetModel(name string) error {
	b.model = name
	if b.chat != nil {
		b.chat.Model = name
		b.chat.TokenLimit = agent.GuessContextLimit(name)
	}
	return nil
}

func (b *AgentBackend) ListSkills() ([]client.Skill, error) {
	list := skills.DiscoverSkills()
	out := make([]client.Skill, len(list))
	for i, s := range list {
		out[i] = client.Skill{
			Name:        s.Name,
			Description: s.Description,
			Location:    s.Location,
		}
	}
	return out, nil
}

func (b *AgentBackend) LoadSkill(name string) (string, error) {
	s := skills.FindSkill(name)
	if s == nil {
		return "", fmt.Errorf("skill not found: %s", name)
	}
	return skills.WrapSkillContent(s), nil
}

func (b *AgentBackend) Connect(provider, apiURL, apiKey, modelName string) error {
	var llm agent.LLMClient
	if provider == "openrouter" {
		llm = agent.NewOpenAIClient(apiURL, apiKey)
	} else if provider == "ollama" {
		llm = agent.NewOllamaClient("")
	} else {
		return fmt.Errorf("unknown provider: %s", provider)
	}
	_, err := llm.ListModels(context.Background())
	if err != nil {
		return fmt.Errorf("connection check failed: %w", err)
	}
	b.model = modelName
	b.provider = provider
	b.apiURL = apiURL
	b.apiKey = apiKey
	b.chat = agent.NewChatSession(modelName, llm, b.confirmationManager, b.questionManager, b.memoryStore)
	return nil
}

func (b *AgentBackend) BaseURL() string {
	return "inline"
}
