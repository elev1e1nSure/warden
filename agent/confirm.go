package agent

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/elev1e1nSure/warden/internal/client"
)

const pendingTimeout = 5 * time.Minute

func newID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

type pendingEntry struct {
	event     chan struct{}
	createdAt time.Time
	resolved  bool
}

func newPendingEntry() pendingEntry {
	return pendingEntry{
		event:     make(chan struct{}),
		createdAt: time.Now(),
	}
}

func (e *pendingEntry) expired() bool {
	return time.Since(e.createdAt) > pendingTimeout
}

// --- ConfirmationManager ---

type ConfirmationManager struct {
	mu      sync.Mutex
	pending map[string]*confirmEntry
}

type confirmEntry struct {
	pendingEntry
	ok bool
}

func NewConfirmationManager() *ConfirmationManager {
	return &ConfirmationManager{pending: make(map[string]*confirmEntry)}
}

func (m *ConfirmationManager) CancelAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, e := range m.pending {
		e.ok = false
		e.resolved = true
		close(e.event)
		delete(m.pending, id)
	}
}

func (m *ConfirmationManager) Register() (string, chan struct{}) {
	id := newID()
	e := &confirmEntry{pendingEntry: newPendingEntry()}
	m.mu.Lock()
	m.pending[id] = e
	m.mu.Unlock()
	return id, e.event
}

func (m *ConfirmationManager) Resolve(id string, ok bool) bool {
	m.mu.Lock()
	e, exists := m.pending[id]
	m.mu.Unlock()
	if !exists || e.resolved {
		return false
	}
	e.ok = ok
	e.resolved = true
	close(e.event)
	return true
}

func (m *ConfirmationManager) Get(id string) (ok bool, found bool) {
	m.mu.Lock()
	e, exists := m.pending[id]
	m.mu.Unlock()
	if !exists {
		return false, false
	}
	if e.expired() {
		m.cancelEntry(id)
		return false, false
	}
	return e.ok, true
}

func (m *ConfirmationManager) Pop(id string) (ok bool, found bool) {
	m.mu.Lock()
	e, exists := m.pending[id]
	if !exists {
		m.mu.Unlock()
		return false, false
	}
	delete(m.pending, id)
	m.mu.Unlock()
	if e.expired() {
		m.cancelEntry(id)
		return false, false
	}
	return e.ok, true
}

func (m *ConfirmationManager) Wait(id string) bool {
	m.mu.Lock()
	e, exists := m.pending[id]
	m.mu.Unlock()
	if !exists {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), pendingTimeout)
	defer cancel()

	select {
	case <-e.event:
	case <-ctx.Done():
		m.cancelEntry(id)
	}

	m.mu.Lock()
	e, exists = m.pending[id]
	if exists {
		delete(m.pending, id)
		ok := e.ok
		m.mu.Unlock()
		return ok
	}
	m.mu.Unlock()
	return false
}

func (m *ConfirmationManager) ActiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, e := range m.pending {
		if e.expired() {
			e.ok = false
			e.resolved = true
			close(e.event)
			delete(m.pending, id)
		}
	}
	return len(m.pending)
}

func (m *ConfirmationManager) cancelEntry(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, exists := m.pending[id]
	if !exists {
		return
	}
	e.ok = false
	e.resolved = true
	close(e.event)
	delete(m.pending, id)
}

// --- QuestionManager ---

type QuestionManager struct {
	mu      sync.Mutex
	pending map[string]*questionEntry
}

type questionEntry struct {
	pendingEntry
	questions []client.QuestionItem
	answers   [][]string
}

func NewQuestionManager() *QuestionManager {
	return &QuestionManager{pending: make(map[string]*questionEntry)}
}

func (m *QuestionManager) CancelAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, e := range m.pending {
		e.answers = nil
		e.resolved = true
		close(e.event)
		delete(m.pending, id)
	}
}

func (m *QuestionManager) Register(questions []client.QuestionItem) (string, chan struct{}) {
	id := newID()
	e := &questionEntry{
		pendingEntry: newPendingEntry(),
		questions:    questions,
		answers:      nil,
	}
	m.mu.Lock()
	m.pending[id] = e
	m.mu.Unlock()
	return id, e.event
}

func (m *QuestionManager) Resolve(id string, answers [][]string) bool {
	m.mu.Lock()
	e, exists := m.pending[id]
	m.mu.Unlock()
	if !exists || e.resolved {
		return false
	}
	e.answers = answers
	if e.answers == nil {
		e.answers = []([]string){}
	}
	e.resolved = true
	close(e.event)
	return true
}

func (m *QuestionManager) Pop(id string) (questions []client.QuestionItem, answers [][]string, found bool) {
	m.mu.Lock()
	e, exists := m.pending[id]
	if !exists {
		m.mu.Unlock()
		return nil, nil, false
	}
	delete(m.pending, id)
	m.mu.Unlock()
	if e.expired() {
		if e.answers != nil {
			m.cancelEntry(id)
			return nil, e.answers, false
		}
		m.cancelEntry(id)
		return nil, nil, false
	}
	return e.questions, e.answers, true
}

func (m *QuestionManager) Wait(id string) [][]string {
	m.mu.Lock()
	e, exists := m.pending[id]
	m.mu.Unlock()
	if !exists {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), pendingTimeout)
	defer cancel()

	select {
	case <-e.event:
	case <-ctx.Done():
		m.cancelEntry(id)
	}

	m.mu.Lock()
	e, exists = m.pending[id]
	if exists {
		delete(m.pending, id)
		answers := e.answers
		m.mu.Unlock()
		return answers
	}
	m.mu.Unlock()
	return nil
}

func (m *QuestionManager) PendingCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, e := range m.pending {
		if e.expired() {
			e.answers = nil
			e.resolved = true
			close(e.event)
			delete(m.pending, id)
		}
	}
	return len(m.pending)
}

func (m *QuestionManager) cancelEntry(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, exists := m.pending[id]
	if !exists {
		return
	}
	e.answers = nil
	e.resolved = true
	close(e.event)
	delete(m.pending, id)
}
