package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

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

// StreamChat sends a chat payload and returns a channel of neutral Events.
func (c *Client) StreamChat(payload map[string]string) <-chan Event {
	ch := make(chan Event, 64)
	go func() {
		defer close(ch)
		body, err := json.Marshal(payload)
		if err != nil {
			ch <- EventError{Text: "\njson error: " + err.Error()}
			ch <- EventDone{}
			return
		}

		req, err := http.NewRequest("POST", c.baseURL+"/chat", bytes.NewReader(body))
		if err != nil {
			ch <- EventError{Text: "\nrequest error: " + err.Error()}
			ch <- EventDone{}
			return
		}
		req.Header.Set("Content-Type", "application/json")
		if c.authToken != "" {
			req.Header.Set("Authorization", "Bearer "+c.authToken)
		}
		resp, err := c.StreamClient.Do(req)
		if err != nil {
			ch <- EventError{Text: "\nnetwork error: " + err.Error()}
			ch <- EventDone{}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			ch <- EventError{Text: "\nserver error: " + resp.Status}
			ch <- EventDone{}
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 10*1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			var base struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(line, &base); err != nil {
				continue
			}
			switch base.Type {
			case "warden_start":
				ch <- EventWardenStart{}
			case "token":
				var t TokenMsg
				if err := json.Unmarshal(line, &t); err != nil {
					continue
				}
				ch <- EventToken{Text: t.Text}
			case "think":
				var t TokenMsg
				if err := json.Unmarshal(line, &t); err != nil {
					continue
				}
				ch <- EventThink{Text: t.Text}
			case "tool_start":
				var t struct {
					Name string `json:"name"`
					Args string `json:"args"`
				}
				if err := json.Unmarshal(line, &t); err != nil {
					continue
				}
				ch <- EventToolStart{Name: t.Name, Args: t.Args}
			case "tool":
				var t ToolMsg
				if err := json.Unmarshal(line, &t); err != nil {
					continue
				}
				ch <- EventTool{Tool: t}
			case "confirm":
				var t struct {
					ID      string   `json:"id"`
					Tool    string   `json:"tool"`
					Risk    string   `json:"risk"`
					Title   string   `json:"title"`
					Summary string   `json:"summary"`
					Details []string `json:"details"`
					Args    string   `json:"args"`
					Preview string   `json:"preview"`
					Default string   `json:"default"`
				}
				if err := json.Unmarshal(line, &t); err != nil {
					continue
				}
				ch <- EventConfirm{
					ID:         t.ID,
					Tool:       t.Tool,
					Risk:       t.Risk,
					Title:      t.Title,
					Summary:    t.Summary,
					Details:    t.Details,
					Args:       t.Args,
					Preview:    t.Preview,
					DefaultVal: t.Default,
				}
			case "question":
				var t struct {
					ID        string `json:"id"`
					Questions []struct {
						Question string `json:"question"`
						Header   string `json:"header"`
						Multiple bool   `json:"multiple"`
						Options  []struct {
							Label       string `json:"label"`
							Description string `json:"description"`
						} `json:"options"`
					} `json:"questions"`
				}
				if err := json.Unmarshal(line, &t); err != nil {
					continue
				}
				items := make([]QuestionItem, len(t.Questions))
				for i, q := range t.Questions {
					opts := make([]QuestionOption, len(q.Options))
					for j, o := range q.Options {
						opts[j] = QuestionOption{Label: o.Label, Description: o.Description}
					}
					items[i] = QuestionItem{
						Question: q.Question,
						Header:   q.Header,
						Multiple: q.Multiple,
						Options:  opts,
					}
				}
				ch <- EventQuestion{ID: t.ID, Questions: items}
			case "done":
				var d struct {
					TokenCount int `json:"token_count"`
					TokenLimit int `json:"token_limit"`
				}
				json.Unmarshal(line, &d)
				ch <- EventDone{TokenCount: d.TokenCount, TokenLimit: d.TokenLimit}
			case "error":
				var e struct {
					Text string `json:"text"`
				}
				json.Unmarshal(line, &e)
				ch <- EventError{Text: e.Text}
				ch <- EventDone{}
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- EventError{Text: fmt.Sprintf("\nstream error: %v", err)}
			ch <- EventDone{}
		}
	}()
	return ch
}
