package session

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"pms/internal/models"
)

const cookieName = "pms_session_id"

// Store 内存会话（与原先 map 行为一致；增加互斥保护并发）
type Store struct {
	mu   sync.RWMutex
	data map[string]*models.User
}

func NewStore() *Store {
	return &Store{data: make(map[string]*models.User)}
}

func (s *Store) Get(r *http.Request) *models.User {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return nil
	}
	if c.Value == "" {
		return nil
	}
	s.mu.RLock()
	u, ok := s.data[c.Value]
	s.mu.RUnlock()
	if !ok {
		return nil
	}
	return u
}

func (s *Store) Set(w http.ResponseWriter, u *models.User) {
	sid := generateID()
	s.mu.Lock()
	s.data[sid] = u
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: sid, Path: "/", HttpOnly: true, MaxAge: 604800})
}

func (s *Store) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1})
}

func generateID() string {
	return fmt.Sprintf("s%d", time.Now().UnixNano())
}
