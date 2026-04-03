package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"pms/internal/config"
	"pms/internal/db"
	"pms/internal/handlers"
	"pms/internal/session"
)

func main() {
	cfg := config.Load()
	sqlDB, err := db.Open(cfg)
	if err != nil {
		log.Fatalf("DB error: %v", err)
	}
	defer sqlDB.Close()
	log.Printf("Database connected")

	sess := session.NewStore()
	h := handlers.New(sqlDB, cfg, sess)

	if err := os.MkdirAll(cfg.UploadDir, 0755); err != nil {
		log.Fatalf("upload dir: %v", err)
	}

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	http.HandleFunc("/login", h.Login)
	http.HandleFunc("/logout", h.Logout)

	http.HandleFunc("/documents/api", h.CheckPermission("document:manage")(h.Documents))
	http.HandleFunc("/documents", h.Documents)
	http.HandleFunc("/projects", h.Projects)
	http.HandleFunc("/projects/api", h.CheckPermission("project:manage")(h.Projects))
	http.HandleFunc("/project/", h.ProjectDetail)
	http.HandleFunc("/project/stages/api", h.CheckPermission("project:edit")(h.ProjectStagesAPI))
	http.HandleFunc("/stage/api", h.CheckPermission("project:edit")(h.StageAPI))
	http.HandleFunc("/stage/", h.StageDetail)
	http.HandleFunc("/users/api", h.Users)
	http.HandleFunc("/users", h.Users)
	http.HandleFunc("/audit-logs", h.CheckPermission("audit:view")(h.AuditLogs))
	http.HandleFunc("/templates", h.ProjectTemplates)
	http.HandleFunc("/menus/api", h.CheckPermission("menu:edit")(h.Menus))
	http.HandleFunc("/menus", h.Menus)
	http.HandleFunc("/functions/api", h.CheckPermission("function:manage")(h.Functions))
	http.HandleFunc("/functions", h.Functions)
	http.HandleFunc("/roles/api", h.CheckPermission("role:manage")(h.Roles))
	http.HandleFunc("/roles/menu", h.CheckPermission("role:menu")(h.Roles))
	http.HandleFunc("/roles/function", h.CheckPermission("role:function")(h.Roles))
	http.HandleFunc("/roles", h.Roles)
	http.HandleFunc("/", h.Home)

	log.Printf("PMS starting on port %d", cfg.ServerPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", cfg.ServerPort), nil))
}
