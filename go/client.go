package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
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
		fmt.Println("[client] starting to read stream")
		for scanner.Scan() {
			line := scanner.Bytes()
			fmt.Printf("[client] got line: %s\n", string(line))
			var base struct{ Type string `json:"type"` }
			if err := json.Unmarshal(line, &base); err != nil {
				fmt.Printf("[client] unmarshal error: %v\n", err)
				continue
			}
			switch base.Type {
			case "token":
				var t TokenMsg
				json.Unmarshal(line, &t)
				fmt.Printf("[client] sending token: %s\n", t.Text)
				ch <- tokenMsg{text: t.Text}
			case "tool":
				var t ToolMsg
				json.Unmarshal(line, &t)
				ch <- toolMsg{tool: t}
			case "done":
				fmt.Println("[client] got done")
				ch <- doneMsg{}
			case "error":
				var e struct{ Text string `json:"text"` }
				json.Unmarshal(line, &e)
				ch <- tokenMsg{text: e.Text}
				ch <- doneMsg{}
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("[client] scanner error: %v\n", err)
		}
	}()
	return ch
}
