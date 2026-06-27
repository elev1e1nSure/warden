package tui

import "github.com/elev1e1nSure/warden/internal/client"

type Backend interface {
	StreamChat(payload map[string]string) <-chan client.Event
	Interrupt() error
	ResetSession() error
	SendQuestion(id string, answers [][]string) error
	SendConfirm(id string, ok bool) error
	SetMode(auto bool) error
	GetStatus() (*client.StatusResult, error)
	Compact() (*client.CompactResult, error)
	SetMemoryState(enabled bool) error
	ClearMemory() (int, error)
	GetMemoryState() (*client.MemoryState, error)
	ListModels() ([]string, string, error)
	SetModel(name string) error
	ListSkills() ([]client.Skill, error)
	LoadSkill(name string) (string, error)
	Connect(provider, apiURL, apiKey, modelName string) error
	BaseURL() string
}
