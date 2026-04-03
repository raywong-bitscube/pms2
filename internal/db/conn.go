package db

import (
	"database/sql"
	"fmt"
	"time"

	"pms/internal/config"

	_ "github.com/go-sql-driver/mysql"
)

// Open 打开 MySQL 连接池，行为与原先 initDB 一致
func Open(cfg *config.Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4&timeout=5s",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)
	database, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(25)
	database.SetMaxIdleConns(10)
	database.SetConnMaxLifetime(5 * time.Minute)
	if err := database.Ping(); err != nil {
		_ = database.Close()
		return nil, err
	}
	return database, nil
}
