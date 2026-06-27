package tools

import (
	"fmt"
	"strings"
)

// --- SkillTool ---

type SkillTool struct{}

func (t *SkillTool) Name() string { return "skill" }

func (t *SkillTool) Execute(args map[string]any) Result {
	return R("error: skill tool not implemented in Go yet")
}

// --- TodoWriteTool ---

type TodoItem struct {
	Content  string `json:"content"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
}

type TodoWriteTool struct {
	sessionID string
}

func NewTodoWriteTool() *TodoWriteTool {
	return &TodoWriteTool{sessionID: "default"}
}

func (t *TodoWriteTool) Name() string { return "todowrite" }

func (t *TodoWriteTool) SetSession(id string) {
	t.sessionID = id
}

func (t *TodoWriteTool) Execute(args map[string]any) Result {
	raw, ok := args["todos"]
	if !ok {
		return R("error: todos list is required")
	}
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return R("error: todos list is empty")
	}
	var todos []TodoItem
	for _, r := range items {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		todos = append(todos, TodoItem{
			Content:  getStr(m, "content"),
			Status:   getStr(m, "status"),
			Priority: getStr(m, "priority"),
		})
	}
	if len(todos) == 0 {
		return R("error: todos list is empty")
	}
	active := 0
	for _, t := range todos {
		if t.Status != "completed" {
			active++
		}
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%d todos — %d total:\n", active, len(todos)))
	for _, t := range todos {
		b.WriteString(fmt.Sprintf("  [%s] %s: %s\n", t.Status, t.Priority, t.Content))
	}
	return R(strings.TrimRight(b.String(), "\n"))
}

// --- QuestionTool ---

type QuestionTool struct{}

func (t *QuestionTool) Name() string { return "question" }

func (t *QuestionTool) Execute(args map[string]any) Result {
	return R("error: question tool needs interactive flow")
}
