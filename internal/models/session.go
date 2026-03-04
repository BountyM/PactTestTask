package models

import (
	"context"
	"sync"

	"github.com/gotd/td/telegram"
)

type Session struct {
	ID       string
	Client   *telegram.Client
	Storage  *MemorySession
	Messages chan *MessageUpdate
	Cancel   context.CancelFunc
	Active   bool
	mux      sync.RWMutex
	Err      chan error
}

func (s *Session) SetActive(active bool) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.Active = active
}

func (s *Session) IsActive() bool {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.Active
}

type MessageUpdate struct {
	MessageID int64
	From      string
	Text      string
	Timestamp int64
}
