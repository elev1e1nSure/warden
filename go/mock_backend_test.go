package tui

import "github.com/elev1e1n/warden/internal/client"

var _ Backend = (*mockBackend)(nil)

type mockBackend struct {
	status     *client.StatusResult
	statusErr  error
	compact    *client.CompactResult
	compactErr error
	memory     *client.MemoryState
	memoryErr  error
	models     []string
	modelCurr  string
	modelsErr  error
	skills     []client.Skill
	skillsErr  error
	skillContent string
	skillErr     error
	connectErr   error
	clearMemCount int
	clearMemErr   error
	baseURL       string
	eventCh       chan client.Event
}

func (m *mockBackend) StreamChat(payload map[string]string) <-chan client.Event {
	if m.eventCh != nil {
		return m.eventCh
	}
	ch := make(chan client.Event)
	close(ch)
	return ch
}

func (m *mockBackend) Interrupt() error            { return nil }
func (m *mockBackend) ResetSession() error           { return nil }
func (m *mockBackend) SendQuestion(id string, answers [][]string) error { return nil }
func (m *mockBackend) SendConfirm(id string, ok bool) error             { return nil }
func (m *mockBackend) SetMode(auto bool) error                          { return nil }
func (m *mockBackend) GetStatus() (*client.StatusResult, error)         { return m.status, m.statusErr }
func (m *mockBackend) Compact() (*client.CompactResult, error)          { return m.compact, m.compactErr }
func (m *mockBackend) SetMemoryState(enabled bool) error                { return nil }
func (m *mockBackend) ClearMemory() (int, error)                       { return m.clearMemCount, m.clearMemErr }
func (m *mockBackend) GetMemoryState() (*client.MemoryState, error)    { return m.memory, m.memoryErr }
func (m *mockBackend) ListModels() ([]string, string, error)           { return m.models, m.modelCurr, m.modelsErr }
func (m *mockBackend) SetModel(name string) error                      { return nil }
func (m *mockBackend) ListSkills() ([]client.Skill, error)             { return m.skills, m.skillsErr }
func (m *mockBackend) LoadSkill(name string) (string, error)           { return m.skillContent, m.skillErr }
func (m *mockBackend) Connect(provider, apiURL, apiKey, modelName string) error { return m.connectErr }
func (m *mockBackend) BaseURL() string                                 { return m.baseURL }
