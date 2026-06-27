package client

// Event is a neutral streaming event emitted by the backend.
type Event interface {
	isEvent()
}

type EventWardenStart struct{}

type EventToken struct{ Text string }

type EventThink struct{ Text string }

type EventToolStart struct {
	Name string
	Args string
}

type EventTool struct{ Tool ToolMsg }

type EventConfirm struct {
	ID         string
	Tool       string
	Risk       string
	Title      string
	Summary    string
	Details    []string
	Args       string
	Preview    string
	DefaultVal string
}

type EventQuestion struct {
	ID        string
	Questions []QuestionItem
}

type QuestionOption struct {
	Label       string
	Description string
}

type QuestionItem struct {
	Question string
	Header   string
	Options  []QuestionOption
	Multiple bool
}

type EventDone struct {
	TokenCount int
	TokenLimit int
}

type EventError struct{ Text string }

func (EventWardenStart) isEvent() {}
func (EventToken) isEvent()       {}
func (EventThink) isEvent()       {}
func (EventToolStart) isEvent()   {}
func (EventTool) isEvent()        {}
func (EventConfirm) isEvent()     {}
func (EventQuestion) isEvent()    {}
func (EventDone) isEvent()        {}
func (EventError) isEvent()       {}
