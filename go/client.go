package tui

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"

	tea "github.com/charmbracelet/bubbletea"
)

type StatusResult struct {
	Model      string `json:"model"`
	Provider   string `json:"provider"`
	Mode       string `json:"mode"`
	Thinking   bool   `json:"thinking"`
	CWD        string `json:"cwd"`
	TokenCount int    `json:"token_count"`
	TokenLimit int    `json:"token_limit"`
}

type CompactResult struct {
	Summary      string `json:"summary"`
	TokensBefore int    `json:"tokens_before"`
	TokensAfter  int    `json:"tokens_after"`
}

type TokenMsg struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ToolMsg struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Args   string `json:"args"`
	Result string `json:"result"`
}

type Client struct {
	BaseURL string
}

func NewClient(url string) *Client {
	return &Client{BaseURL: url}
}

func (c *Client) ResetSession() error {
	resp, err := http.Post(c.BaseURL+"/reset", "application/json", nil)
	if err != nil {
		logError("reset failed: " + err.Error())
		return err
	}
	resp.Body.Close()
	request("POST", "/reset", resp.StatusCode)
	info("session reset")
	return nil
}

func (c *Client) SetMode(auto bool) error {
	body, _ := json.Marshal(map[string]any{"auto": auto})
	resp, err := http.Post(c.BaseURL+"/mode", "application/json", bytes.NewReader(body))
	if err != nil {
		logError("set mode failed: " + err.Error())
		return err
	}
	resp.Body.Close()
	request("POST", "/mode", resp.StatusCode)
	mode := "AUTO"
	if !auto {
		mode = "SAFE"
	}
	info("mode: " + mode)
	return nil
}

func (c *Client) SetThinking(enabled bool) error {
	body, _ := json.Marshal(map[string]any{"enabled": enabled})
	resp, err := http.Post(c.BaseURL+"/thinking", "application/json", bytes.NewReader(body))
	if err != nil {
		logError("set thinking failed: " + err.Error())
		return err
	}
	resp.Body.Close()
	request("POST", "/thinking", resp.StatusCode)
	status := "enabled"
	if !enabled {
		status = "disabled"
	}
	info("thinking: " + status)
	return nil
}

func (c *Client) SendQuestion(id string, answers [][]string) error {
	body, _ := json.Marshal(map[string]any{"id": id, "answers": answers})
	resp, err := http.Post(c.BaseURL+"/question", "application/json", bytes.NewReader(body))
	if err != nil {
		logError("send question failed: " + err.Error())
		return err
	}
	resp.Body.Close()
	request("POST", "/question", resp.StatusCode)
	info("question sent")
	return nil
}

func (c *Client) SendConfirm(id string, ok bool) error {
	body, _ := json.Marshal(map[string]any{"id": id, "ok": ok})
	resp, err := http.Post(c.BaseURL+"/confirm", "application/json", bytes.NewReader(body))
	if err != nil {
		logError("send confirm failed: " + err.Error())
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) GetStatus() (*StatusResult, error) {
	resp, err := http.Get(c.BaseURL + "/status")
	if err != nil {
		logError("get status failed: " + err.Error())
		return nil, err
	}
	defer resp.Body.Close()
	request("GET", "/status", resp.StatusCode)
	var result StatusResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) Compact() (*CompactResult, error) {
	resp, err := http.Post(c.BaseURL+"/compact", "application/json", nil)
	if err != nil {
		logError("compact failed: " + err.Error())
		return nil, err
	}
	defer resp.Body.Close()
	request("POST", "/compact", resp.StatusCode)
	var result CompactResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetTools() ([]string, error) {
	resp, err := http.Get(c.BaseURL + "/tools")
	if err != nil {
		logError("get tools failed: " + err.Error())
		return nil, err
	}
	defer resp.Body.Close()
	request("GET", "/tools", resp.StatusCode)
	var result struct {
		Tools []string `json:"tools"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Tools, nil
}

func (c *Client) SendMessage(text string) <-chan tea.Msg {
	ch := make(chan tea.Msg, 64)
	go func() {
		defer close(ch)
		msg := map[string]string{"type": "message", "text": text}
		body, _ := json.Marshal(msg)

		resp, err := http.Post(c.BaseURL+"/chat", "application/json", bytes.NewReader(body))
		if err != nil {
			ch <- tokenMsg{text: "\nnetwork error: " + err.Error()}
			ch <- doneMsg{}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			ch <- tokenMsg{text: "\nserver error: " + resp.Status}
			ch <- doneMsg{}
			return
		}
		request("POST", "/chat", resp.StatusCode)

		scanner := bufio.NewScanner(resp.Body)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			var base struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(line, &base); err != nil {
				logError("json unmarshal error: " + err.Error())
				continue
			}
			switch base.Type {
			case "warden_start":
				ch <- wardenStartMsg{}
			case "token":
				var t TokenMsg
				if err := json.Unmarshal(line, &t); err != nil {
					logError("json unmarshal token error: " + err.Error())
					continue
				}
				ch <- tokenMsg{text: t.Text}
			case "think":
				var t TokenMsg
				if err := json.Unmarshal(line, &t); err != nil {
					logError("json unmarshal think error: " + err.Error())
					continue
				}
				ch <- thinkMsg{text: t.Text}
			case "tool_start":
				var t struct {
					Name string `json:"name"`
					Args string `json:"args"`
				}
				if err := json.Unmarshal(line, &t); err != nil {
					logError("json unmarshal tool_start error: " + err.Error())
					continue
				}
				ch <- toolStartMsg{name: t.Name, args: t.Args}
			case "tool":
				var t ToolMsg
				if err := json.Unmarshal(line, &t); err != nil {
					logError("json unmarshal tool error: " + err.Error())
					continue
				}
				ch <- toolMsg{tool: t}
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
					logError("json unmarshal confirm error: " + err.Error())
					continue
				}
				ch <- confirmMsg{
					id:      t.ID,
					tool:    t.Tool,
					risk:    t.Risk,
					title:   t.Title,
					summary: t.Summary,
					details: t.Details,
					args:    t.Args,
					preview: t.Preview,
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
					logError("json unmarshal question error: " + err.Error())
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
				ch <- questionMsg{id: t.ID, questions: items}
			case "done":
				var d struct {
					TokenCount int `json:"token_count"`
					TokenLimit int `json:"token_limit"`
				}
				json.Unmarshal(line, &d)
				ch <- doneMsg{tokenCount: d.TokenCount, tokenLimit: d.TokenLimit}
			case "error":
				var e struct {
					Text string `json:"text"`
				}
				json.Unmarshal(line, &e)
				ch <- tokenMsg{text: e.Text}
				ch <- doneMsg{}
			}
		}
	}()
	return ch
}
