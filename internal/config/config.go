package config

import (
	"os"
	"strconv"
)

// Config 应用配置（可通过环境变量覆盖，未设置时保持与历史默认行为兼容本地开发）
type Config struct {
	ServerPort int
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
	UploadDir  string
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

// Load 从环境变量加载；未设置 PMS_DB_PASSWORD 时使用与旧版一致的默认值以便本地运行
func Load() *Config {
	cfg := &Config{
		ServerPort: getenvInt("PMS_SERVER_PORT", 6606),
		DBHost:     getenv("PMS_DB_HOST", "localhost"),
		DBPort:     getenvInt("PMS_DB_PORT", 3306),
		DBUser:     getenv("PMS_DB_USER", "pms_user"),
		DBPassword: getenv("PMS_DB_PASSWORD", ""),
		DBName:     getenv("PMS_DB_NAME", "project_management"),
		UploadDir:  getenv("PMS_UPLOAD_DIR", "./uploads"),
	}
	if cfg.DBPassword == "" {
		cfg.DBPassword = "PmsPass@2026"
	}
	return cfg
}
