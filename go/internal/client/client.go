package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL      string
	HTTPClient   *http.Client
	StreamClient *http.Client
}

func NewClient(url string) *Client {
	return &Client{
		baseURL: url,
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
	return c.HTTPClient.Post(c.baseURL+path, "application/json", body)
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
	resp, err := c.HTTPClient.Get(c.baseURL + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(v)
}

func (c *Client) BaseURL() string { return c.baseURL }

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

func (c *Client) Interrupt() error {
	return c.postOK("/interrupt", nil)
}

func (c *Client) GetMemoryState() (*MemoryState, error) {
	var result MemoryState
	if err := c.getJSON("/memory/state", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) SetMemoryState(enabled bool) error {
	return c.postOK("/memory/state", map[string]any{"enabled": enabled})
}

func (c *Client) ClearMemory() (int, error) {
	var result struct {
		Cleared int `json:"cleared"`
	}
	if err := c.postDecode("/memory/clear", nil, &result); err != nil {
		return 0, err
	}
	return result.Cleared, nil
}

func (c *Client) GetMemorySnapshot() (map[string]any, error) {
	var result map[string]any
	if err := c.getJSON("/memory/snapshot", &result); err != nil {
		return nil, err
	}
	return result, nil
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
	resp, err := c.HTTPClient.Get(c.baseURL + "/skill/" + name)
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
