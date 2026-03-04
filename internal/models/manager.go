package models

import (
	"sync"
)

type Manager struct {
	sessions map[string]*Session
	mux      sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

func (m *Manager) Add(id string, session *Session) {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.sessions[id] = session
}

func (m *Manager) Remove(id string) {
	m.mux.Lock()
	defer m.mux.Unlock()
	delete(m.sessions, id)
}

func (m *Manager) Get(id string) (*Session, bool) {
	m.mux.RLock()
	defer m.mux.RUnlock()
	session, ok := m.sessions[id]
	return session, ok
}
