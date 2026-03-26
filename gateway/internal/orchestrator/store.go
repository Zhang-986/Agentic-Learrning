package orchestrator

import (
	"sync"

	"github.com/agentic-learning/gateway/internal/model"
)

// SessionStore 定义了持久化 Harness 会话的接口
type SessionStore interface {
	Save(session *model.HarnessSession) error
	Get(id string) (*model.HarnessSession, bool)
	List() []*model.HarnessSession
}

// InMemSessionStore 内存实现的 SessionStore (用于演示，实际应使用 Redis/Postgres)
type InMemSessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*model.HarnessSession
}

func NewInMemSessionStore() *InMemSessionStore {
	return &InMemSessionStore{
		sessions: make(map[string]*model.HarnessSession),
	}
}

func (s *InMemSessionStore) Save(session *model.HarnessSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = session
	return nil
}

func (s *InMemSessionStore) Get(id string) (*model.HarnessSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	return sess, ok
}

func (s *InMemSessionStore) List() []*model.HarnessSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]*model.HarnessSession, 0, len(s.sessions))
	for _, sess := range s.sessions {
		res = append(res, sess)
	}
	return res
}
