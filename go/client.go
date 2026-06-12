package tui

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type StatusResult struct {
	Model      string `json:"model"`
	Mode       string `json:"mode"`
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
	Diff   string `json:"diff"`
}

type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Location    string `json:"location"`
}

type Client struct {
	BaseURL      string
	HTTPClient   *http.Client
	StreamClient *http.Client
}

func NewClient(url string) *Client {
	return &Client{
		BaseURL: url,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		StreamClient: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 30 * time.Second,
			},
		},
	}
}

// postJSON marshals payload (when non-nil) and POSTs it to path.
func (c *Client) postJSON(path string, payload any) (*http.Response, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	return c.HTTPClient.Post(c.BaseURL+path, "application/json", body)
}

// postOK POSTs payload to path and expects a 200 response.
func (c *Client) postOK(path string, payload any) error {
	resp, err := c.postJSON(path, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}
	return nil
}

// postDecode POSTs payload to path and decodes the JSON response into v.
func (c *Client) postDecode(path string, payload any, v any) error {
	resp, err := c.postJSON(path, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(v)
}

// getJSON GETs path and decodes the JSON response into v.
func (c *Client) getJSON(path string, v any) error {
	resp, err := c.HTTPClient.Get(c.BaseURL + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(v)
}

func (c *Client) ResetSession() error {
	return c.postOK("/reset", nil)
}

func (c *Client) SetMode(auto bool) error {
	return c.postOK("/mode", map[string]any{"auto": auto})
}

func (c *Client) SendQuestion(id string, answers [][]string) error {
	return c.postOK("/question", map[string]any{"id": id, "answers": answers})
}

func (c *Client) SendConfirm(id string, ok bool) error {
	return c.postOK("/confirm", map[string]any{"id": id, "ok": ok})
}

func (c *Client) ListModels() ([]string, string, error) {
	var result struct {
		Models  []string `json:"models"`
		Current string   `json:"current"`
		Error   string   `json:"error"`
	}
	if err := c.getJSON("/models", &result); err != nil {
		return nil, "", err
	}
	if result.Error != "" {
		return nil, "", fmt.Errorf("%s", result.Error)
	}
	return result.Models, result.Current, nil
}

func (c *Client) Connect(provider, apiURL, apiKey, model string) error {
	payload := map[string]any{
		"provider": provider,
		"api_url":  apiURL,
		"api_key":  apiKey,
		"model":    model,
	}
	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := c.postDecode("/connect", payload, &result); err != nil {
		return err
	}
	if !result.OK {
		return fmt.Errorf("%s", result.Error)
	}
	return nil
}

func (c *Client) SetModel(model string) error {
	return c.postOK("/model/set", map[string]any{"model": model})
}

func (c *Client) GetStatus() (*StatusResult, error) {
	var result StatusResult
	if err := c.getJSON("/status", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) Compact() (*CompactResult, error) {
	var result CompactResult
	if err := c.postDecode("/compact", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) Shutdown() error {
	return c.postOK("/shutdown", nil)
}

func (c *Client) ListSkills() ([]Skill, error) {
	var result struct {
		Skills []Skill `json:"skills"`
	}
	if err := c.getJSON("/skills", &result); err != nil {
		return nil, err
	}
	return result.Skills, nil
}

func (c *Client) LoadSkill(name string) (string, error) {
	resp, err := c.HTTPClient.Get(c.BaseURL + "/skill/" + name)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("skill not found: %s", name)
	}
	var result struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Content, nil
}

func (c *Client) SendMessage(text string) <-chan tea.Msg {
	ch := make(chan tea.Msg, 64)
	go func() {
		defer close(ch)
		msg := map[string]string{"type": "message", "text": text}
		body, err := json.Marshal(msg)
		if err != nil {
			ch <- tokenMsg{text: "\njson error: " + err.Error()}
			ch <- doneMsg{}
			return
		}

		resp, err := c.StreamClient.Post(c.BaseURL+"/chat", "application/json", bytes.NewReader(body))
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

		scanner := bufio.NewScanner(resp.Body)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)
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
				ch <- wardenStartMsg{}
			case "token":
				var t TokenMsg
				if err := json.Unmarshal(line, &t); err != nil {
					continue
				}
				ch <- tokenMsg{text: t.Text}
			case "think":
				var t TokenMsg
				if err := json.Unmarshal(line, &t); err != nil {
					continue
				}
				ch <- thinkMsg{text: t.Text}
			case "tool_start":
				var t struct {
					Name string `json:"name"`
					Args string `json:"args"`
				}
				if err := json.Unmarshal(line, &t); err != nil {
					continue
				}
				ch <- toolStartMsg{name: t.Name, args: t.Args}
			case "tool":
				var t ToolMsg
				if err := json.Unmarshal(line, &t); err != nil {
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
					continue
				}
				ch <- confirmMsg{
					id:         t.ID,
					tool:       t.Tool,
					risk:       t.Risk,
					title:      t.Title,
					summary:    t.Summary,
					details:    t.Details,
					args:       t.Args,
					preview:    t.Preview,
					defaultVal: t.Default,
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
