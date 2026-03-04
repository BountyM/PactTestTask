package models

import (
	"context"
	"sync"

	"github.com/gotd/td/session"
)

// memorySession – ваше in-memory хранилище сессий (без изменений)
type MemorySession struct {
	mux  sync.RWMutex
	data []byte
}

func (s *MemorySession) LoadSession(context.Context) ([]byte, error) {
	if s == nil {
		return nil, session.ErrNotFound
	}
	s.mux.RLock()
	defer s.mux.RUnlock()
	if len(s.data) == 0 {
		return nil, session.ErrNotFound
	}

	return append([]byte(nil), s.data...), nil
}

func (s *MemorySession) StoreSession(ctx context.Context, data []byte) error {
	s.mux.Lock()
	s.data = data
	s.mux.Unlock()
	return nil
}
