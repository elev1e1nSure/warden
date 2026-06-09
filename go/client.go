package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"

	tea "github.com/charmbracelet/bubbletea"
)

type Message struct {
	Type string `json:"type"`
	Text string `json:"text"`
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

type DoneMsg struct {
	Type string `json:"type"`
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
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) SetMode(auto bool) error {
	body, _ := json.Marshal(map[string]any{"auto": auto})
	resp, err := http.Post(c.BaseURL+"/mode", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) SendConfirm(id string, ok bool) error {
	body, _ := json.Marshal(map[string]any{"id": id, "ok": ok})
	resp, err := http.Post(c.BaseURL+"/confirm", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) SendMessage(text string) <-chan tea.Msg {
	ch := make(chan tea.Msg, 64)
	go func() {
		defer close(ch)
		msg := Message{Type: "message", Text: text}
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
				json.Unmarshal(line, &t)
				ch <- tokenMsg{text: t.Text}
			case "think":
				var t TokenMsg
				json.Unmarshal(line, &t)
				ch <- thinkMsg{text: t.Text}
			case "tool_start":
				var t struct {
					Name string `json:"name"`
					Args string `json:"args"`
				}
				json.Unmarshal(line, &t)
				ch <- toolStartMsg{name: t.Name, args: t.Args}
			case "tool":
				var t ToolMsg
				json.Unmarshal(line, &t)
				ch <- toolMsg{tool: t}
			case "confirm":
				var t struct {
					ID   string `json:"id"`
					Tool string `json:"tool"`
					Args string `json:"args"`
				}
				json.Unmarshal(line, &t)
				ch <- confirmMsg{id: t.ID, tool: t.Tool, args: t.Args}
			case "done":
				ch <- doneMsg{}
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
