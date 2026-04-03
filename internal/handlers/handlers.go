package handlers

import (
	"database/sql"

	"pms/internal/config"
	"pms/internal/session"
)

// Handlers 聚合 HTTP 依赖，便于注入测试
type Handlers struct {
	DB      *sql.DB
	Config  *config.Config
	Session *session.Store
}

// New 构造处理器
func New(database *sql.DB, cfg *config.Config, sess *session.Store) *Handlers {
	return &Handlers{DB: database, Config: cfg, Session: sess}
}
