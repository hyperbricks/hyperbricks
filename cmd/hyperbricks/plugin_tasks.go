package main

import (
	"fmt"
	"sync"
	"time"
)

type pluginTask struct {
	mu         sync.Mutex
	ID         string
	Status     string
	StartedAt  time.Time
	FinishedAt time.Time
	Message    string
	Log        string
}

type pluginTaskSnapshot struct {
	ID         string `json:"task_id"`
	Status     string `json:"status"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	Message    string `json:"message,omitempty"`
}

type pluginTaskStore struct {
	mu    sync.Mutex
	tasks map[string]*pluginTask
}

func newPluginTaskStore() *pluginTaskStore {
	return &pluginTaskStore{
		tasks: make(map[string]*pluginTask),
	}
}

func (s *pluginTaskStore) newTask() *pluginTask {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := fmt.Sprintf("plug_%d", time.Now().UnixNano())
	task := &pluginTask{
		ID:     id,
		Status: "queued",
	}
	s.tasks[id] = task
	return task
}

func (s *pluginTaskStore) get(id string) (*pluginTask, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[id]
	return task, ok
}

func (t *pluginTask) start() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = "running"
	t.StartedAt = time.Now().UTC()
}

func (t *pluginTask) finish(err error, log string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err != nil {
		t.Status = "error"
		t.Message = err.Error()
	} else {
		t.Status = "done"
		t.Message = "ok"
	}
	t.FinishedAt = time.Now().UTC()
	t.Log = log
}

func (t *pluginTask) snapshot() pluginTaskSnapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	return pluginTaskSnapshot{
		ID:         t.ID,
		Status:     t.Status,
		StartedAt:  formatTaskTime(t.StartedAt),
		FinishedAt: formatTaskTime(t.FinishedAt),
		Message:    t.Message,
	}
}

func (t *pluginTask) logText() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.Log
}

func formatTaskTime(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339)
}
