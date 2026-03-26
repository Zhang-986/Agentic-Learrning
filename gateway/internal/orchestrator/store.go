package orchestrator

import (
	"sync"
	"time"

	"github.com/agentic-learning/gateway/internal/model"
)

// SessionStore 定义了持久化 Harness 会话的接口
type SessionStore interface {
	Save(session *model.HarnessSession) error
	Get(id string) (*model.HarnessSession, bool)
	List() []*model.HarnessSession
}

// ==================== InMemSessionStore (带 TTL 逐出) ====================

const (
	defaultSessionTTL    = 2 * time.Hour  // 空闲 session 2h 后逐出
	defaultEvictInterval = 10 * time.Minute // 每 10 分钟扫描一次
	defaultMaxSessions   = 1000            // 最大保留数量
)

// sessionEntry 包装了 session 和最后访问时间
type sessionEntry struct {
	session    *model.HarnessSession
	lastAccess time.Time
}

// InMemSessionStore 内存实现的 SessionStore，带 TTL 逐出防止内存泄漏
type InMemSessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*sessionEntry

	ttl          time.Duration
	maxSessions  int
	stopEvict    chan struct{}
}

type StoreOption func(*InMemSessionStore)

func WithTTL(ttl time.Duration) StoreOption {
	return func(s *InMemSessionStore) { s.ttl = ttl }
}

func WithMaxSessions(max int) StoreOption {
	return func(s *InMemSessionStore) { s.maxSessions = max }
}

func NewInMemSessionStore(opts ...StoreOption) *InMemSessionStore {
	s := &InMemSessionStore{
		sessions:    make(map[string]*sessionEntry),
		ttl:         defaultSessionTTL,
		maxSessions: defaultMaxSessions,
		stopEvict:   make(chan struct{}),
	}
	for _, opt := range opts {
		opt(s)
	}

	// 启动后台逐出协程
	go s.evictLoop()

	return s
}

func (s *InMemSessionStore) Save(session *model.HarnessSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[session.ID] = &sessionEntry{
		session:    session,
		lastAccess: time.Now(),
	}

	// 硬上限检查：超过最大数量时，淘汰最旧的
	if len(s.sessions) > s.maxSessions {
		s.evictOldestLocked()
	}

	return nil
}

func (s *InMemSessionStore) Get(id string) (*model.HarnessSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.sessions[id]
	if !ok {
		return nil, false
	}
	// 读取时刷新 lastAccess
	entry.lastAccess = time.Now()
	return entry.session, true
}

func (s *InMemSessionStore) List() []*model.HarnessSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]*model.HarnessSession, 0, len(s.sessions))
	for _, entry := range s.sessions {
		res = append(res, entry.session)
	}
	return res
}

// Len 返回当前 session 数量（用于测试/监控）
func (s *InMemSessionStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

// Stop 停止后台逐出协程，用于优雅关闭
func (s *InMemSessionStore) Stop() {
	close(s.stopEvict)
}

// evictLoop 定期清理过期 session
func (s *InMemSessionStore) evictLoop() {
	ticker := time.NewTicker(defaultEvictInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.evictExpired()
		case <-s.stopEvict:
			return
		}
	}
}

// evictExpired 清理超过 TTL 的 session
func (s *InMemSessionStore) evictExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-s.ttl)
	for id, entry := range s.sessions {
		if entry.lastAccess.Before(cutoff) {
			delete(s.sessions, id)
		}
	}
}

// evictOldestLocked 在持有锁的情况下淘汰最旧的 session
func (s *InMemSessionStore) evictOldestLocked() {
	var oldestID string
	var oldestTime time.Time

	for id, entry := range s.sessions {
		if oldestID == "" || entry.lastAccess.Before(oldestTime) {
			oldestID = id
			oldestTime = entry.lastAccess
		}
	}
	if oldestID != "" {
		delete(s.sessions, oldestID)
	}
}
