package models

import "time"

// User 登录用户 / 会话
type User struct {
	ID       int
	Username string
	RealName string
	Email    string
	Status   int
	IsAdmin  int
}

// Project 项目
type Project struct {
	ID          int
	Name        string
	Code        string
	ManagerID   int
	ManagerName string
	Status      int
	Progress    int
	StartDate   string
	EndDate     string
	Description string
	CreatedAt   time.Time
}

// ProjectStage 项目阶段
type ProjectStage struct {
	ID            int
	ProjectID     int
	Name          string
	Code          string
	OrderNum      int
	Status        int
	Progress      int
	PlanStartDate string
	PlanEndDate   string
}

// Document 文档列表展示
type Document struct {
	ID             int
	ProjectID      int
	ProjectName    string
	StageID        int
	StageName      string
	Name           string
	Type           string
	FileName       string
	FileSize       int64
	Version        string
	Status         int
	UploadedBy     int
	UploadedByName string
	CreatedAt      time.Time
}
