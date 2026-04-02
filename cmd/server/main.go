package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

var config = &Config{
	ServerPort: 6606, DBHost: "localhost", DBPort: 3306,
	DBUser: "pms_user", DBPassword: "PmsPass@2026",
	DBName: "project_management", UploadDir: "./uploads",
}

type Config struct {
	ServerPort int; DBHost string; DBPort int; DBUser string
	DBPassword string; DBName string; UploadDir string
}

var db *sql.DB
var sessions = make(map[string]*User)

type User struct {
	ID int; Username string; RealName string; Email string; Status int; IsAdmin int
}

type Project struct {
	ID int; Name string; Code string; ManagerID int; ManagerName string
	Status int; Progress int; StartDate string; EndDate string
	Description string; CreatedAt time.Time
}

type ProjectStage struct {
	ID int; ProjectID int; Name string; Code string; OrderNum int
	Status int; Progress int; PlanStartDate string; PlanEndDate string
}

type Document struct {
	ID int; ProjectID int; ProjectName string; StageID int; StageName string
	Name string; Type string; FileName string; FileSize int64; Version string
	Status int; UploadedBy int; UploadedByName string; CreatedAt time.Time
}

func initDB() error {
	var err error
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4&timeout=5s",
		config.DBUser, config.DBPassword, config.DBHost, config.DBPort, config.DBName)
	db, err = sql.Open("mysql", dsn)
	if err != nil { return err }
	// 配置连接池
	db.SetMaxOpenConns(25)    // 最大打开连接数
	db.SetMaxIdleConns(10)    // 最大空闲连接数
	db.SetConnMaxLifetime(5 * time.Minute) // 连接最大生命周期
	return db.Ping()
}

func getSession(r *http.Request) *User {
	c, err := r.Cookie("pms_session_id")
	if err != nil { return nil }
	if c.Value == "" { return nil }
	u, ok := sessions[c.Value]
	if !ok { return nil }
	return u
}

func setSession(w http.ResponseWriter, u *User) {
	sid := fmt.Sprintf("s%d", time.Now().UnixNano())
	sessions[sid] = u
	http.SetCookie(w, &http.Cookie{Name:"pms_session_id", Value:sid, Path:"/", HttpOnly:true, MaxAge:604800})
}

func clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name:"pms_session_id", Value:"", Path:"/", MaxAge:-1})
}

func auditLog(uid int, atype, acat, tbl string, rid int64, old, nw, ip, ua, url, meth string) {
	// 直接写入数据库（简单可靠）
	if db == nil { 
		log.Println("auditLog: db is nil")
		return 
	}
	var oldData, newData interface{}
	if old == "" { oldData = nil } else { oldData = old }
	if nw == "" { newData = nil } else { newData = nw }
	
	_, err := db.Exec(`INSERT INTO audit_log (user_id,action_type,action_category,table_name,record_id,old_data,new_data,ip_address,user_agent,request_url,request_method,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,NOW())`,
		uid,atype,acat,tbl,rid,oldData,newData,ip,ua,url,meth)
	if err != nil {
		log.Printf("auditLog error: %v\n", err)
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		t, _ := template.ParseFiles("templates/login.html")
		t.Execute(w, nil)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"无效请求"})
		return
	}
	if req.Username == "" || req.Password == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"用户名和密码不能为空"})
		return
	}
	var u User; var hash string
	err := db.QueryRow("SELECT id,username,password_hash,real_name,email,is_admin,status FROM user WHERE username=?", req.Username).
		Scan(&u.ID, &u.Username, &hash, &u.RealName, &u.Email, &u.IsAdmin, &u.Status)
	if err != nil || u.Status != 1 || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"用户名或密码错误"})
		return
	}
	setSession(w, &u)
	auditLog(u.ID,"login","system","",0,"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
	json.NewEncoder(w).Encode(map[string]interface{}{"success":true,"user":u})
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	u := getSession(r)
	if u == nil { http.Redirect(w,r,"/login",303); return }
	var tp,ap,td,tu int
	db.QueryRow("SELECT COUNT(*) FROM project WHERE status != 4").Scan(&tp)
	db.QueryRow("SELECT COUNT(*) FROM project WHERE status IN(1,2)").Scan(&ap)
	db.QueryRow("SELECT COUNT(*) FROM document WHERE status=1").Scan(&td)
	db.QueryRow("SELECT COUNT(*) FROM user WHERE status=1").Scan(&tu)
	stats := map[string]int{"TotalProjects":tp,"ActiveProjects":ap,"TotalDocuments":td,"TotalUsers":tu}
	var ps []Project
	rows,err := db.Query("SELECT id,name,code,status,progress,created_at FROM project WHERE status != 4 ORDER BY created_at DESC LIMIT 5")
	if err == nil {
		for rows.Next() { var p Project; rows.Scan(&p.ID,&p.Name,&p.Code,&p.Status,&p.Progress,&p.CreatedAt); ps=append(ps,p) }
		rows.Close()
	}
	menus := getUserMenus(u)
	tmpl := template.Must(template.ParseFiles("templates/base.html","templates/home.html"))
	tmpl.ExecuteTemplate(w,"base.html",map[string]interface{}{"Title":"首页","CurrentUser":u,"Stats":stats,"RecentProjects":ps,"Menus":menus,"CurrentUrl":"/"})
}

func projectsHandler(w http.ResponseWriter, r *http.Request) {
	u := getSession(r)
	if u == nil { http.Redirect(w,r,"/login",303); return }
	
	// API 路由 - 必须放在最前面并 return
	if r.URL.Path == "/projects/api" {
		handleProjectsAPI(w, r, u)
		return
	}
	
	// 页面路由 - 只显示未归档的项目 (status != 4)
	var ps []Project
	rows,err := db.Query(`SELECT p.id,p.name,p.code,p.status,p.progress,p.start_date,p.end_date,COALESCE(u.real_name,u.username),p.description,p.created_at FROM project p LEFT JOIN user u ON p.manager_id=u.id WHERE p.status != 4 ORDER BY p.created_at DESC`)
	if err == nil {
		for rows.Next() { var p Project; rows.Scan(&p.ID,&p.Name,&p.Code,&p.Status,&p.Progress,&p.StartDate,&p.EndDate,&p.ManagerName,&p.Description,&p.CreatedAt); ps=append(ps,p) }
		rows.Close()
	}
	menus := getUserMenus(u)
	tmpl := template.Must(template.ParseFiles("templates/base.html","templates/projects.html"))
	tmpl.ExecuteTemplate(w,"base.html",map[string]interface{}{"Title":"项目管理","CurrentUser":u,"Projects":ps,"Menus":menus,"CurrentUrl":"/projects"})
}

func handleProjectsAPI(w http.ResponseWriter, r *http.Request, u *User) {
	if r.Method == "POST" {
		if !hasFunctionPermission(u, "project:create") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"无权创建项目，请联系管理员开通权限"})
			return
		}
		var req struct {
			Name        string `json:"name"`
			Code        string `json:"code"`
			StartDate   string `json:"start_date"`
			EndDate     string `json:"end_date"`
			Description string `json:"description"`
			TemplateID  int    `json:"template_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"解析失败"})
			return
		}
		if req.TemplateID == 0 { req.TemplateID = 1 }
		
		res, err := db.Exec(`INSERT INTO project(name,code,template_id,manager_id,status,start_date,end_date,description,created_at) VALUES(?,?,?,?,1,?,?,?,NOW())`,
			req.Name,req.Code,req.TemplateID,u.ID,func() string { if req.StartDate=="" { return "2026-01-01" }; return req.StartDate }(),func() string { if req.EndDate=="" { return "2026-12-31" }; return req.EndDate }(),req.Description)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":err.Error()})
			return
		}
		pid, _ := res.LastInsertId()
		
		// 创建阶段
		rows, err := db.Query("SELECT name,code,order_num FROM template_stage WHERE template_id=? ORDER BY order_num", req.TemplateID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var n,c string; var o int
				rows.Scan(&n,&c,&o)
				db.Exec(`INSERT INTO project_stage(project_id,name,code,order_num,status,created_at)VALUES(?,?,?,?,1,NOW())`,pid,n,c,o)
			}
		}
		
		auditLog(u.ID,"create","project","project",pid,"",fmt.Sprintf("name:%s",req.Name),r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
		json.NewEncoder(w).Encode(map[string]interface{}{"success":true,"id":pid})
		
	} else if r.Method == "PUT" {
		if !hasFunctionPermission(u, "project:edit") {
			json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"无权编辑项目，请联系管理员开通权限"})
			return
		}
		var req struct {
			ID          int    `json:"id"`
			Status      int    `json:"status"`
			Progress    int    `json:"progress"`
			Name        string `json:"name"`
			Code        string `json:"code"`
			Description string `json:"description"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.ID == 0 {
			json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"缺少 id"})
			return
		}
		// 查询旧数据
		var oldName, oldCode, oldDesc string
		var oldStatus, oldProgress int
		db.QueryRow("SELECT name,code,status,progress,description FROM project WHERE id=?",req.ID).
			Scan(&oldName,&oldCode,&oldStatus,&oldProgress,&oldDesc)
		oldData := map[string]interface{}{"id":req.ID,"name":oldName,"code":oldCode,"status":oldStatus,"progress":oldProgress,"description":oldDesc}
		oldJSONBytes, _ := json.Marshal(oldData)
		oldJSON := string(oldJSONBytes)
		// 更新
		_, err := db.Exec(`UPDATE project SET name=?,code=?,status=?,progress=?,description=?,updated_at=NOW() WHERE id=?`,
			req.Name,req.Code,req.Status,req.Progress,req.Description,req.ID)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":err.Error()})
			return
		}
		// 异步写入审计日志（不阻塞主请求）
		go func(uid int, atype, acat, tbl string, rid int64, old, nw, ip, ua, url, meth string) {
			auditLog(uid, atype, acat, tbl, rid, old, nw, ip, ua, url, meth)
		}(u.ID,"update","project","project",int64(req.ID),oldJSON,"",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
		json.NewEncoder(w).Encode(map[string]interface{}{"success":true})
		
	} else if r.Method == "DELETE" {
		if !hasFunctionPermission(u, "project:delete") {
			json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"无权删除项目，请联系管理员开通权限"})
			return
		}
		var req struct {
			ID int `json:"id"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.ID == 0 {
			json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"缺少 id"})
			return
		}
		// 查询旧数据
		var oldName, oldCode, oldDesc string
		var oldStatus int
		db.QueryRow("SELECT name,code,status,description FROM project WHERE id=?",req.ID).
			Scan(&oldName,&oldCode,&oldStatus,&oldDesc)
		oldData := map[string]interface{}{"id":req.ID,"name":oldName,"code":oldCode,"status":oldStatus,"description":oldDesc}
		oldJSONBytes, _ := json.Marshal(oldData)
		oldJSON := string(oldJSONBytes)
		_, err := db.Exec("UPDATE project SET status=4 WHERE id=?",req.ID)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":err.Error()})
			return
		}
		// 异步写入审计日志
		go func(uid int, atype, acat, tbl string, rid int64, old, nw, ip, ua, url, meth string) {
			auditLog(uid, atype, acat, tbl, rid, old, nw, ip, ua, url, meth)
		}(u.ID,"delete","project","project",int64(req.ID),oldJSON,"",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
		json.NewEncoder(w).Encode(map[string]interface{}{"success":true})
		
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"Method not allowed"})
	}
}

func projectDetailHandler(w http.ResponseWriter, r *http.Request) {
	u := getSession(r)
	if u == nil { http.Redirect(w,r,"/login",303); return }
	idStr := r.URL.Path[len("/project/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		http.Error(w, "项目不存在", http.StatusNotFound)
		return
	}
	var p Project
	err = db.QueryRow(`SELECT p.id,p.name,p.code,p.status,p.progress,p.start_date,p.end_date,COALESCE(u.real_name,u.username),p.description FROM project p LEFT JOIN user u ON p.manager_id=u.id WHERE p.id=?`,id).
		Scan(&p.ID,&p.Name,&p.Code,&p.Status,&p.Progress,&p.StartDate,&p.EndDate,&p.ManagerName,&p.Description)
	if err != nil || p.ID == 0 {
		http.Error(w, "项目不存在", http.StatusNotFound)
		return
	}
	var ss []ProjectStage
	rows,err := db.Query("SELECT id,name,code,order_num,status,progress,plan_start_date,plan_end_date FROM project_stage WHERE project_id=? ORDER BY order_num",id)
	if err == nil {
		for rows.Next() { var s ProjectStage; rows.Scan(&s.ID,&s.Name,&s.Code,&s.OrderNum,&s.Status,&s.Progress,&s.PlanStartDate,&s.PlanEndDate); ss=append(ss,s) }
		rows.Close()
	}
	menus := getUserMenus(u)
	tmpl := template.Must(template.ParseFiles("templates/base.html","templates/project_detail.html"))
	tmpl.ExecuteTemplate(w,"base.html",map[string]interface{}{"Title":"项目详情","CurrentUser":u,"Project":p,"Stages":ss,"Menus":menus,"CurrentUrl":"/projects"})
}

func projectStagesAPI(w http.ResponseWriter, r *http.Request) {
	u := getSession(r)
	if u == nil { http.Error(w,"Unauthorized",401); return }
	if !hasFunctionPermission(u, "project:edit") {
		json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"无权编辑项目阶段，请联系管理员开通权限"})
		return
	}
	var req struct {
		StageID  int `json:"stage_id"`
		Status   int `json:"status"`
		Progress int `json:"progress"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.StageID == 0 {
		json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"缺少 stage_id"})
		return
	}
	_, err := db.Exec(`UPDATE project_stage SET status=?,progress=?,updated_at=NOW() WHERE id=?`,req.Status,req.Progress,req.StageID)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":err.Error()})
		return
	}
	// 异步写入审计日志
	go func(uid int, atype, acat, tbl string, rid int64, old, nw, ip, ua, url, meth string) {
		auditLog(uid, atype, acat, tbl, rid, old, nw, ip, ua, url, meth)
	}(u.ID,"update","project_stage","project_stage",int64(req.StageID),"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
	json.NewEncoder(w).Encode(map[string]interface{}{"success":true})
}

func stageDetailHandler(w http.ResponseWriter, r *http.Request) {
	u := getSession(r)
	if u == nil { http.Redirect(w,r,"/login",303); return }
	idStr := r.URL.Path[len("/stage/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		http.Error(w, "阶段不存在", http.StatusNotFound)
		return
	}
	
	// 获取阶段信息
	var s ProjectStage
	err = db.QueryRow("SELECT id,project_id,name,code,order_num,status,progress,plan_start_date,plan_end_date FROM project_stage WHERE id=?",id).
		Scan(&s.ID,&s.ProjectID,&s.Name,&s.Code,&s.OrderNum,&s.Status,&s.Progress,&s.PlanStartDate,&s.PlanEndDate)
	if err != nil || s.ID == 0 {
		http.Error(w, "阶段不存在", http.StatusNotFound)
		return
	}
	// 格式化日期为 YYYY-MM-DD (input type="date" 需要的格式)
	if len(s.PlanStartDate) >= 10 { s.PlanStartDate = s.PlanStartDate[:10] }
	if len(s.PlanEndDate) >= 10 { s.PlanEndDate = s.PlanEndDate[:10] }
	
	// 获取项目信息
	var p Project
	db.QueryRow("SELECT id,name,code FROM project WHERE id=?",s.ProjectID).
		Scan(&p.ID,&p.Name,&p.Code)
	
	// 获取文档列表
	type Doc struct {
		ID, ProjectID int; Name, Type, FileName, FilePath, Version, CreatedAtStr, FileSizeStr string
		FileSize int64; Status, UploadedBy int
	}
	var ds []Doc
	rows,err := db.Query(`SELECT id,project_id,name,type,file_name,file_path,version,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i'),file_size,status,uploaded_by FROM document WHERE stage_id=? AND status=1 ORDER BY created_at DESC`,id)
	if err == nil {
		for rows.Next() {
			var d Doc
			rows.Scan(&d.ID,&d.ProjectID,&d.Name,&d.Type,&d.FileName,&d.FilePath,&d.Version,&d.CreatedAtStr,&d.FileSize,&d.Status,&d.UploadedBy)
			if d.FileSize >= 1048576 { d.FileSizeStr = fmt.Sprintf("%.2f MB",float64(d.FileSize)/1048576) } else if d.FileSize >= 1024 { d.FileSizeStr = fmt.Sprintf("%.2f KB",float64(d.FileSize)/1024) } else { d.FileSizeStr = fmt.Sprintf("%d B",d.FileSize) }
			ds=append(ds,d)
		}
		rows.Close()
	}
	
	menus := getUserMenus(u)
	tmpl := template.Must(template.ParseFiles("templates/base.html","templates/stage_detail.html"))
	tmpl.ExecuteTemplate(w,"base.html",map[string]interface{}{"Title":"阶段详情","CurrentUser":u,"Stage":s,"Project":p,"Documents":ds,"Menus":menus,"CurrentUrl":"/projects"})
}

func stageAPI(w http.ResponseWriter, r *http.Request) {
	u := getSession(r)
	if u == nil { http.Error(w,"Unauthorized",401); return }
	
	if r.Method == "PUT" {
		if !hasFunctionPermission(u, "project:edit") {
			json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"无权编辑项目阶段，请联系管理员开通权限"})
			return
		}
		var req struct {
			ID            int    `json:"id"`
			OrderNum      int    `json:"order_num"`
			Status        int    `json:"status"`
			Progress      int    `json:"progress"`
			Name          string `json:"name"`
			Code          string `json:"code"`
			PlanStartDate string `json:"plan_start_date"`
			PlanEndDate   string `json:"plan_end_date"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"解析失败"})
			return
		}
		if req.ID == 0 {
			json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"缺少 id"})
			return
		}
		_, err := db.Exec(`UPDATE project_stage SET name=?,code=?,order_num=?,status=?,progress=?,plan_start_date=?,plan_end_date=?,updated_at=NOW() WHERE id=?`,
			req.Name,req.Code,req.OrderNum,req.Status,req.Progress,req.PlanStartDate,req.PlanEndDate,req.ID)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":err.Error()})
			return
		}
		// 异步写入审计日志
		go func(uid int, atype, acat, tbl string, rid int64, old, nw, ip, ua, url, meth string) {
			auditLog(uid, atype, acat, tbl, rid, old, nw, ip, ua, url, meth)
		}(u.ID,"update","project_stage","project_stage",int64(req.ID),"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
		json.NewEncoder(w).Encode(map[string]interface{}{"success":true})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"Method not allowed"})
	}
}

func documentsHandler(w http.ResponseWriter, r *http.Request) {
	u := getSession(r)
	if u == nil { http.Redirect(w,r,"/login",303); return }
	if r.URL.Path == "/documents/api" {
		if r.Method == "POST" {
			r.ParseMultipartForm(32<<20)
			f,h,e := r.FormFile("file")
			if e != nil { json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"上传失败"}); return }
			defer f.Close()
			pid,_ := strconv.Atoi(r.FormValue("project_id"))
			if pid <= 0 {
				json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"project_id 无效"})
				return
			}
			sidStr := r.FormValue("stage_id")
			var sid int
			if sidStr != "" { sid,_ = strconv.Atoi(sidStr) }
			os.MkdirAll(fmt.Sprintf("%s/%d",config.UploadDir,pid),0755)
			// 净化文件名，防止路径穿越
			safeName := filepath.Base(h.Filename)
			fp := fmt.Sprintf("%s/%d/%s",config.UploadDir,pid,safeName)
			dst, err := os.Create(fp)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"创建文件失败"})
				return
			}
			defer dst.Close()
			// 使用 io.Copy 完整复制文件，避免大文件截断
			_, err = io.Copy(dst, f)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"写入文件失败"})
				return
			}
			if sid > 0 {
				_, err = db.Exec(`INSERT INTO document(project_id,stage_id,name,type,file_path,file_name,file_size,status,uploaded_by,created_at)VALUES(?,?,?,?,?,?,?,1,?,NOW())`,
					pid,sid,r.FormValue("name"),r.FormValue("type"),fp,h.Filename,h.Size,u.ID)
			} else {
				_, err = db.Exec(`INSERT INTO document(project_id,name,type,file_path,file_name,file_size,status,uploaded_by,created_at)VALUES(?,?,?,?,?,?,1,?,NOW())`,
					pid,r.FormValue("name"),r.FormValue("type"),fp,h.Filename,h.Size,u.ID)
			}
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"上传失败："+err.Error()})
				return
			}
			// 异步写入审计日志
			go func(uid int, atype, acat, tbl string, rid int64, old, nw, ip, ua, url, meth string) {
				auditLog(uid, atype, acat, tbl, rid, old, nw, ip, ua, url, meth)
			}(u.ID,"upload","document","document",0,"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
			json.NewEncoder(w).Encode(map[string]interface{}{"success":true})
		} else if r.Method == "DELETE" {
			var req struct {
				ID int `json:"id"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			if req.ID == 0 {
				json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"缺少 id"})
				return
			}
			// 查询删除前的文档数据
			var docProjectID, docStageID, docFileSize, docStatus, docUploadedBy int
			var docName, docType, docFileName, docFilePath, docVersion string
			err := db.QueryRow(`SELECT project_id,stage_id,name,type,file_path,file_name,file_size,version,status,uploaded_by FROM document WHERE id=?`, req.ID).
				Scan(&docProjectID,&docStageID,&docName,&docType,&docFilePath,&docFileName,&docFileSize,&docVersion,&docStatus,&docUploadedBy)
			var oldJSON string
			if err == nil {
				oldData := map[string]interface{}{
					"id":req.ID,"project_id":docProjectID,"stage_id":docStageID,
					"name":docName,"type":docType,"file_name":docFileName,
					"file_path":docFilePath,"file_size":docFileSize,
					"version":docVersion,"status":docStatus,
				}
				oldJSONBytes, _ := json.Marshal(oldData)
				oldJSON = string(oldJSONBytes)
			} else {
				oldJSON = fmt.Sprintf("query error:%v",err)
			}
			_, err = db.Exec("UPDATE document SET status=0 WHERE id=?",req.ID)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":err.Error()})
				return
			}
			// 异步写入审计日志
			go func(uid int, atype, acat, tbl string, rid int64, old, nw, ip, ua, url, meth string) {
				auditLog(uid, atype, acat, tbl, rid, old, nw, ip, ua, url, meth)
			}(u.ID,"delete","document","document",int64(req.ID),oldJSON,"",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
			json.NewEncoder(w).Encode(map[string]interface{}{"success":true})
		}
		return
	}
	var ds []Document
	rows,err := db.Query(`SELECT d.id,d.project_id,p.name,d.stage_id,IFNULL(s.name,'-'),d.name,d.type,d.file_name,d.file_size,d.version,d.status,d.uploaded_by,IFNULL(u.real_name,u.username),d.created_at FROM document d LEFT JOIN project p ON d.project_id=p.id LEFT JOIN project_stage s ON d.stage_id=s.id LEFT JOIN user u ON d.uploaded_by=u.id WHERE d.status=1 ORDER BY d.created_at DESC`)
	if err == nil {
		for rows.Next() { var d Document; rows.Scan(&d.ID,&d.ProjectID,&d.ProjectName,&d.StageID,&d.StageName,&d.Name,&d.Type,&d.FileName,&d.FileSize,&d.Version,&d.Status,&d.UploadedBy,&d.UploadedByName,&d.CreatedAt); ds=append(ds,d) }
		rows.Close()
	}
	menus := getUserMenus(u)
	tmpl := template.Must(template.ParseFiles("templates/base.html","templates/documents.html"))
	tmpl.ExecuteTemplate(w,"base.html",map[string]interface{}{"Title":"文档管理","CurrentUser":u,"Documents":ds,"Menus":menus,"CurrentUrl":"/documents"})
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
	u := getSession(r)
	if u == nil { http.Redirect(w,r,"/login",303); return }
	if r.URL.Path == "/users/api" {
		if r.Method == "POST" {
			// 重置密码接口
			var resetReq struct {
				Action   string `json:"action"`
				UserID   int    `json:"user_id"`
			}
			json.NewDecoder(r.Body).Decode(&resetReq)
			if resetReq.Action == "reset_password" && resetReq.UserID > 0 {
				if !hasFunctionPermission(u, "user:reset_password") {
					json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"无权重置用户密码，请联系管理员开通权限"})
					return
				}
				h,_ := bcrypt.GenerateFromPassword([]byte("123"),12)
				_, err := db.Exec(`UPDATE user SET password_hash=?,updated_at=NOW() WHERE id=?`, h, resetReq.UserID)
				if err != nil {
					json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"重置失败："+err.Error()})
					return
				}
				auditLog(u.ID,"reset_password","user","user",int64(resetReq.UserID),"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
				json.NewEncoder(w).Encode(map[string]interface{}{"success":true,"message":"密码已重置为 123"})
				return
			}
			
			var req struct {
				ID       int    `json:"id"`
				Username string `json:"username"`
				Password string `json:"password"`
				RealName string `json:"real_name"`
				Email    string `json:"email"`
				IsAdmin  int    `json:"is_admin"`
				Status   int    `json:"status"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			if req.Username == "" {
				json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"用户名不能为空"})
				return
			}
			if req.ID == 0 {
				// 新建用户
				if req.Password == "" {
					json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"密码不能为空"})
					return
				}
				h,_ := bcrypt.GenerateFromPassword([]byte(req.Password),12)
				_, err := db.Exec(`INSERT INTO user(username,password_hash,real_name,email,is_admin,status,created_at)VALUES(?,?,?,?,?,?,NOW())`,req.Username,string(h),req.RealName,req.Email,req.IsAdmin,req.Status)
				if err != nil {
					json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"创建失败："+err.Error()})
					return
				}
				auditLog(u.ID,"create","user","user",0,"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
			} else {
				// 编辑用户 - 不修改密码，密码由用户自己修改
				_, err := db.Exec(`UPDATE user SET username=?,real_name=?,email=?,is_admin=?,status=?,updated_at=NOW() WHERE id=?`,req.Username,req.RealName,req.Email,req.IsAdmin,req.Status,req.ID)
				if err != nil {
					json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"更新失败："+err.Error()})
					return
				}
				auditLog(u.ID,"update","user","user",int64(req.ID),"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"success":true})
			return
		}
		if r.Method == "GET" {
			// 获取用户角色
			userID := r.URL.Query().Get("user_id")
			type Role struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
				Code string `json:"code"`
			}
			var roles []Role
			rows,err := db.Query(`SELECT id,name,code FROM role WHERE status=1 ORDER BY id`)
			if err == nil {
				for rows.Next() { var x Role; rows.Scan(&x.ID,&x.Name,&x.Code); roles=append(roles,x) }
				rows.Close()
			}
			userRoles := make([]int, 0)
			rows2,err2 := db.Query(`SELECT role_id FROM user_role WHERE user_id=?`,userID)
			if err2 == nil {
				for rows2.Next() { var rid int; rows2.Scan(&rid); userRoles=append(userRoles,rid) }
				rows2.Close()
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"success":true,"roles":roles,"user_roles":userRoles})
			return
		}
		if r.Method == "PUT" {
			// 保存用户角色
			var req struct{ UserID int `json:"user_id"`; RoleIDs []int `json:"role_ids"` }
			json.NewDecoder(r.Body).Decode(&req)
			if req.UserID == 0 {
				json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"user_id 不能为空"})
				return
			}
			_, err := db.Exec(`DELETE FROM user_role WHERE user_id=?`,req.UserID)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"删除旧角色失败："+err.Error()})
				return
			}
			for _, rid := range req.RoleIDs {
				_, err := db.Exec(`INSERT INTO user_role(user_id,role_id)VALUES(?,?)`,req.UserID,rid)
				if err != nil {
					json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"添加角色失败："+err.Error()})
					return
				}
			}
			auditLog(u.ID,"assign_roles","user","user_role",int64(req.UserID),"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
			json.NewEncoder(w).Encode(map[string]interface{}{"success":true})
			return
		}
	}
	type U struct{ID,Status,IsAdmin int;Username,RealName,Email string;CreatedAt string}
	var us []U
	rows,err := db.Query("SELECT id,username,real_name,email,is_admin,status,DATE_FORMAT(created_at,'%Y-%m-%d') FROM user ORDER BY created_at DESC")
	if err == nil {
		for rows.Next() { var x U; rows.Scan(&x.ID,&x.Username,&x.RealName,&x.Email,&x.IsAdmin,&x.Status,&x.CreatedAt); us=append(us,x) }
		rows.Close()
	}
	menus := getUserMenus(u)
	funcs := getUserFunctions(u)
	tmpl := template.Must(template.ParseFiles("templates/base.html","templates/users.html"))
	tmpl.ExecuteTemplate(w,"base.html",map[string]interface{}{"Title":"用户管理","CurrentUser":u,"Users":us,"Menus":menus,"UserFunctions":funcs,"CurrentUrl":"/users"})
}

func auditLogsHandler(w http.ResponseWriter, r *http.Request) {
	u := getSession(r)
	if u == nil { http.Redirect(w,r,"/login",303); return }
	type L struct {
		ID, RecordID   int64
		Username       string
		ActionType     string
		ActionCategory string
		TableName      string
		IPAddress      string
		UserAgent      string
		RequestURL     string
		RequestMethod  string
		OldData        string
		NewData        string
		CreatedAt      string
	}
	var ls []L
	rows,err := db.Query(`SELECT al.id,u.username,al.action_type,al.action_category,al.table_name,al.record_id,al.ip_address,al.user_agent,al.request_url,al.request_method,IFNULL(al.old_data,''),IFNULL(al.new_data,''),DATE_FORMAT(al.created_at,'%Y-%m-%d %H:%i:%s') FROM audit_log al LEFT JOIN user u ON al.user_id=u.id ORDER BY al.created_at DESC LIMIT 100`)
	if err == nil {
		for rows.Next() {
			var x L
			rows.Scan(&x.ID,&x.Username,&x.ActionType,&x.ActionCategory,&x.TableName,&x.RecordID,&x.IPAddress,&x.UserAgent,&x.RequestURL,&x.RequestMethod,&x.OldData,&x.NewData,&x.CreatedAt)
			ls=append(ls,x)
		}
		rows.Close()
	}
	menus := getUserMenus(u)
	tmpl := template.Must(template.ParseFiles("templates/base.html","templates/audit_logs.html"))
	tmpl.ExecuteTemplate(w,"base.html",map[string]interface{}{"Title":"审计日志","CurrentUser":u,"Logs":ls,"Menus":menus,"CurrentUrl":"/audit-logs"})
}

func templatesHandler(w http.ResponseWriter, r *http.Request) {
	u := getSession(r)
	if u == nil { http.Redirect(w,r,"/login",303); return }
	type T struct{ID,StageCount int;Name,Code,Version string}
	var ts []T
	rows,err := db.Query(`SELECT t.id,t.name,t.code,t.version,COUNT(ts.id) FROM project_template t LEFT JOIN template_stage ts ON t.id=ts.template_id GROUP BY t.id`)
	if err == nil {
		for rows.Next() { var x T; rows.Scan(&x.ID,&x.Name,&x.Code,&x.Version,&x.StageCount); ts=append(ts,x) }
		rows.Close()
	}
	menus := getUserMenus(u)
	tmpl := template.Must(template.ParseFiles("templates/base.html","templates/templates.html"))
	tmpl.ExecuteTemplate(w,"base.html",map[string]interface{}{"Title":"项目模板","CurrentUser":u,"Templates":ts,"Menus":menus,"CurrentUrl":"/templates"})
}

func menusHandler(w http.ResponseWriter, r *http.Request) {
	u := getSession(r)
	if u == nil { http.Redirect(w,r,"/login",303); return }
	if r.URL.Path == "/menus/api" {
		if r.Method == "POST" {
			var req struct {
				ID       int    `json:"id"`
				Name     string `json:"name"`
				Icon     string `json:"icon"`
				Url      string `json:"url"`
				ParentID int    `json:"parent_id"`
				OrderNum int    `json:"order_num"`
				Status   int    `json:"status"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			if req.Name == "" {
				json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"菜单名称不能为空"})
				return
			}
			if req.ID == 0 {
				_, err := db.Exec(`INSERT INTO menu(name,icon,url,parent_id,order_num,status,created_at)VALUES(?,?,?,?,?,1,NOW())`,req.Name,req.Icon,req.Url,req.ParentID,req.OrderNum)
				if err != nil {
					json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"创建失败："+err.Error()})
					return
				}
				auditLog(u.ID,"create","menu","menu",0,"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
			} else {
				_, err := db.Exec(`UPDATE menu SET name=?,icon=?,url=?,parent_id=?,order_num=?,status=?,updated_at=NOW() WHERE id=?`,req.Name,req.Icon,req.Url,req.ParentID,req.OrderNum,req.Status,req.ID)
				if err != nil {
					json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"更新失败："+err.Error()})
					return
				}
				auditLog(u.ID,"update","menu","menu",int64(req.ID),"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"success":true})
			return
		}
		if r.Method == "DELETE" {
			var req struct{ ID int `json:"id"` }
			json.NewDecoder(r.Body).Decode(&req)
			if req.ID == 0 {
				json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"id 不能为空"})
				return
			}
			_, err := db.Exec(`DELETE FROM menu WHERE id=?`,req.ID)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"删除失败："+err.Error()})
				return
			}
			auditLog(u.ID,"delete","menu","menu",int64(req.ID),"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
			json.NewEncoder(w).Encode(map[string]interface{}{"success":true})
			return
		}
	}
	type M struct {
		ID, ParentID, OrderNum, Status, Level int
		Name, Icon, Url, ParentName string
	}
	var ms []M
	rows,err := db.Query(`SELECT m.id,m.name,m.icon,m.url,m.parent_id,m.order_num,m.status,IFNULL(p.name,'') FROM menu m LEFT JOIN menu p ON m.parent_id=p.id ORDER BY m.parent_id,m.order_num`)
	if err == nil {
		for rows.Next() {
			var x M
			rows.Scan(&x.ID,&x.Name,&x.Icon,&x.Url,&x.ParentID,&x.OrderNum,&x.Status,&x.ParentName)
			if x.ParentID == 0 { x.Level = 0 } else { x.Level = 1 }
			ms=append(ms,x)
		}
		rows.Close()
	}
	navMenus := getUserMenus(u)
	tmpl := template.Must(template.ParseFiles("templates/base.html","templates/menus.html"))
	tmpl.ExecuteTemplate(w,"base.html",map[string]interface{}{"Title":"菜单管理","CurrentUser":u,"Menus":navMenus,"MenuList":ms,"CurrentUrl":"/menus"})
}

// 获取用户有权限的菜单
func getUserMenus(u *User) []map[string]interface{} {
	var menus []map[string]interface{}
	var rows *sql.Rows
	var err error
	if u.IsAdmin == 1 {
		rows, err = db.Query(`SELECT id,name,icon,url,parent_id,order_num FROM menu WHERE status=1 ORDER BY parent_id,order_num`)
	} else {
		rows, err = db.Query(`SELECT DISTINCT m.id,m.name,m.icon,m.url,m.parent_id,m.order_num FROM menu m INNER JOIN role_menu rm ON m.id=rm.menu_id INNER JOIN user_role ur ON rm.role_id=ur.role_id WHERE ur.user_id=? AND m.status=1 ORDER BY m.parent_id,m.order_num`, u.ID)
	}
	if err != nil { return menus }
	defer rows.Close()
	for rows.Next() {
		var id, parentID, orderNum int
		var name, icon, url string
		rows.Scan(&id,&name,&icon,&url,&parentID,&orderNum)
		menus = append(menus, map[string]interface{}{
			"ID": id, "Name": name, "Icon": icon, "Url": url, "ParentID": parentID, "OrderNum": orderNum,
		})
	}
	return menus
}

// 获取用户有权限的功能
func getUserFunctions(u *User) map[string]bool {
	funcs := make(map[string]bool)
	var rows *sql.Rows
	var err error
	if u.IsAdmin == 1 {
		rows, err = db.Query(`SELECT code FROM system_function WHERE status=1`)
	} else {
		rows, err = db.Query(`SELECT DISTINCT f.code FROM system_function f INNER JOIN role_function rf ON f.id=rf.function_id INNER JOIN user_role ur ON rf.role_id=ur.role_id WHERE ur.user_id=? AND f.status=1`, u.ID)
	}
	if err != nil { return funcs }
	defer rows.Close()
	for rows.Next() {
		var code string
		rows.Scan(&code)
		funcs[code] = true
	}
	return funcs
}

// 检查用户是否有某个功能权限
func hasFunctionPermission(u *User, funcCode string) bool {
	if u.IsAdmin == 1 { return true }
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM system_function f INNER JOIN role_function rf ON f.id=rf.function_id INNER JOIN user_role ur ON rf.role_id=ur.role_id WHERE ur.user_id=? AND f.code=? AND f.status=1`, u.ID, funcCode).Scan(&count)
	return count > 0
}

// 权限检查中间件
func checkPermission(funcCode string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			u := getSession(r)
			if u == nil {
				http.Redirect(w, r, "/login", 303)
				return
			}
			if !hasFunctionPermission(u, funcCode) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"message": "无权访问此功能，请联系管理员开通权限",
				})
				return
			}
			next(w, r)
		}
	}
}

func rolesHandler(w http.ResponseWriter, r *http.Request) {
	u := getSession(r)
	if u == nil { http.Redirect(w,r,"/login",303); return }
	if r.URL.Path == "/roles/api" {
		if r.Method == "POST" {
			var req struct {
				ID          int    `json:"id"`
				Name        string `json:"name"`
				Code        string `json:"code"`
				Description string `json:"description"`
				Status      int    `json:"status"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			if req.Name == "" || req.Code == "" {
				json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"角色名称和标识不能为空"})
				return
			}
			if req.ID == 0 {
				// 创建角色
				_, err := db.Exec(`INSERT INTO role(name,code,description,status,created_at)VALUES(?,?,?,?,NOW())`,req.Name,req.Code,req.Description,req.Status)
				if err != nil {
					json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"创建失败："+err.Error()})
					return
				}
				auditLog(u.ID,"create","role","role",0,"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
			} else {
				// 编辑角色
				_, err := db.Exec(`UPDATE role SET name=?,code=?,description=?,status=?,updated_at=NOW() WHERE id=?`,req.Name,req.Code,req.Description,req.Status,req.ID)
				if err != nil {
					json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"更新失败："+err.Error()})
					return
				}
				auditLog(u.ID,"update","role","role",int64(req.ID),"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"success":true})
			return
		}
		if r.Method == "DELETE" {
			var req struct{ ID int `json:"id"` }
			json.NewDecoder(r.Body).Decode(&req)
			if req.ID == 0 {
				json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"id 不能为空"})
				return
			}
			_, err := db.Exec(`DELETE FROM role WHERE id=?`,req.ID)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"删除角色失败："+err.Error()})
				return
			}
			db.Exec(`DELETE FROM role_menu WHERE role_id=?`,req.ID)
			db.Exec(`DELETE FROM role_function WHERE role_id=?`,req.ID)
			auditLog(u.ID,"delete","role","role",int64(req.ID),"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
			json.NewEncoder(w).Encode(map[string]interface{}{"success":true})
			return
		}
	}
	if r.URL.Path == "/roles/menu" && r.Method == "GET" {
		roleID := r.URL.Query().Get("role_id")
		type M struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Icon string `json:"icon"`
		}
		var ms []M
		rows,err := db.Query(`SELECT id,name,icon FROM menu ORDER BY order_num`)
		if err == nil {
			for rows.Next() { var x M; rows.Scan(&x.ID,&x.Name,&x.Icon); ms=append(ms,x) }
			rows.Close()
		}
		roleMenus := make([]int, 0)
		rows2,err2 := db.Query(`SELECT menu_id FROM role_menu WHERE role_id=?`,roleID)
		if err2 == nil {
			for rows2.Next() { var mid int; rows2.Scan(&mid); roleMenus=append(roleMenus,mid) }
			rows2.Close()
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success":true,"menus":ms,"role_menus":roleMenus})
		return
	}
	if r.URL.Path == "/roles/menu" && r.Method == "POST" {
		var req struct{ RoleID int `json:"role_id"`; MenuIDs []int `json:"menu_ids"` }
		json.NewDecoder(r.Body).Decode(&req)
		if req.RoleID == 0 {
			json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"role_id 不能为空"})
			return
		}
		_, err := db.Exec(`DELETE FROM role_menu WHERE role_id=?`,req.RoleID)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"删除旧权限失败："+err.Error()})
			return
		}
		for _, mid := range req.MenuIDs {
			_, err := db.Exec(`INSERT INTO role_menu(role_id,menu_id)VALUES(?,?)`,req.RoleID,mid)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"添加权限失败："+err.Error()})
				return
			}
		}
		auditLog(u.ID,"assign_menu_perms","role","role_menu",int64(req.RoleID),"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
		json.NewEncoder(w).Encode(map[string]interface{}{"success":true})
		return
	}
	if r.URL.Path == "/roles/function" && r.Method == "GET" {
		roleID := r.URL.Query().Get("role_id")
		type F struct {
			ID       int    `json:"id"`
			Name     string `json:"name"`
			Code     string `json:"code"`
			MenuName string `json:"menu_name"`
			MenuIcon string `json:"menu_icon"`
		}
		var fs []F
		rows,err := db.Query(`SELECT f.id,f.name,f.code,m.name,m.icon FROM system_function f LEFT JOIN menu m ON f.menu_id=m.id ORDER BY m.order_num,f.id`)
		if err == nil {
			for rows.Next() { var x F; rows.Scan(&x.ID,&x.Name,&x.Code,&x.MenuName,&x.MenuIcon); fs=append(fs,x) }
			rows.Close()
		}
		roleFuncs := make([]int, 0)
		rows2,err2 := db.Query(`SELECT function_id FROM role_function WHERE role_id=?`,roleID)
		if err2 == nil {
			for rows2.Next() { var fid int; rows2.Scan(&fid); roleFuncs=append(roleFuncs,fid) }
			rows2.Close()
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success":true,"functions":fs,"role_funcs":roleFuncs})
		return
	}
	if r.URL.Path == "/roles/function" && r.Method == "POST" {
		var req struct{ RoleID int `json:"role_id"`; FuncIDs []int `json:"function_ids"` }
		json.NewDecoder(r.Body).Decode(&req)
		if req.RoleID == 0 {
			json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"role_id 不能为空"})
			return
		}
		_, err := db.Exec(`DELETE FROM role_function WHERE role_id=?`,req.RoleID)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"删除旧权限失败："+err.Error()})
			return
		}
		for _, fid := range req.FuncIDs {
			_, err := db.Exec(`INSERT INTO role_function(role_id,function_id)VALUES(?,?)`,req.RoleID,fid)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"添加权限失败："+err.Error()})
				return
			}
		}
		auditLog(u.ID,"assign_func_perms","role","role_function",int64(req.RoleID),"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
		json.NewEncoder(w).Encode(map[string]interface{}{"success":true})
		return
	}
	type R struct{ ID, Status int; Name, Code, Description string }
	var rs []R
	rows,err := db.Query(`SELECT id,name,code,description,status FROM role ORDER BY id`)
	if err == nil {
		for rows.Next() { var x R; rows.Scan(&x.ID,&x.Name,&x.Code,&x.Description,&x.Status); rs=append(rs,x) }
		rows.Close()
	}
	menus := getUserMenus(u)
	tmpl := template.Must(template.ParseFiles("templates/base.html","templates/roles.html"))
	tmpl.ExecuteTemplate(w,"base.html",map[string]interface{}{"Title":"角色管理","CurrentUser":u,"Roles":rs,"Menus":menus,"CurrentUrl":"/roles"})
}

func functionsHandler(w http.ResponseWriter, r *http.Request) {
	u := getSession(r)
	if u == nil { http.Redirect(w,r,"/login",303); return }
	if r.URL.Path == "/functions/api" {
		if r.Method == "POST" {
			var req struct {
				ID          int    `json:"id"`
				Name        string `json:"name"`
				Code        string `json:"code"`
				MenuID      int    `json:"menu_id"`
				Url         string `json:"url"`
				Method      string `json:"method"`
				Description string `json:"description"`
				Status      int    `json:"status"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			if req.Name == "" || req.Code == "" {
				json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"功能名称和标识不能为空"})
				return
			}
			if req.ID == 0 {
				_, err := db.Exec(`INSERT INTO system_function(menu_id,name,code,url,method,description,status,created_at)VALUES(?,?,?,?,?,?,1,NOW())`,req.MenuID,req.Name,req.Code,req.Url,req.Method,req.Description)
				if err != nil {
					json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"创建失败："+err.Error()})
					return
				}
				auditLog(u.ID,"create","function","system_function",0,"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
			} else {
				_, err := db.Exec(`UPDATE system_function SET menu_id=?,name=?,code=?,url=?,method=?,description=?,status=?,updated_at=NOW() WHERE id=?`,req.MenuID,req.Name,req.Code,req.Url,req.Method,req.Description,req.Status,req.ID)
				if err != nil {
					json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"更新失败："+err.Error()})
					return
				}
				auditLog(u.ID,"update","function","system_function",int64(req.ID),"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"success":true})
			return
		}
		if r.Method == "DELETE" {
			var req struct{ ID int `json:"id"` }
			json.NewDecoder(r.Body).Decode(&req)
			if req.ID == 0 {
				json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"id 不能为空"})
				return
			}
			_, err := db.Exec(`DELETE FROM system_function WHERE id=?`,req.ID)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"删除失败："+err.Error()})
				return
			}
			auditLog(u.ID,"delete","function","system_function",int64(req.ID),"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
			json.NewEncoder(w).Encode(map[string]interface{}{"success":true})
			return
		}
	}
	type F struct {
		ID, MenuID, Status int
		Name, Code, MenuName, Url, Method, Description string
	}
	var fs []F
	rows,err := db.Query(`SELECT f.id,f.menu_id,f.name,f.code,m.name,f.url,f.method,f.description,f.status FROM system_function f LEFT JOIN menu m ON f.menu_id=m.id ORDER BY f.menu_id,f.id`)
	if err == nil {
		for rows.Next() {
			var x F
			rows.Scan(&x.ID,&x.MenuID,&x.Name,&x.Code,&x.MenuName,&x.Url,&x.Method,&x.Description,&x.Status)
			fs=append(fs,x)
		}
		rows.Close()
	}
	
	type M struct{ ID int; Name string }
	var ms []M
	rows2,err2 := db.Query(`SELECT id,name FROM menu WHERE status=1 ORDER BY order_num`)
	if err2 == nil {
		for rows2.Next() {
			var x M
			rows2.Scan(&x.ID,&x.Name)
			ms=append(ms,x)
		}
		rows2.Close()
	}
	
	menus := getUserMenus(u)
	tmpl := template.Must(template.ParseFiles("templates/base.html","templates/functions.html"))
	tmpl.ExecuteTemplate(w,"base.html",map[string]interface{}{"Title":"功能管理","CurrentUser":u,"Functions":fs,"Menus":menus,"MenuList":ms,"CurrentUrl":"/functions"})
}

func logoutHandler(w http.ResponseWriter, r *http.Request) { clearSession(w); http.Redirect(w,r,"/login",303) }

func main() {
	if err := initDB(); err != nil { log.Fatalf("DB error:%v",err) }
	log.Printf("Database connected")
	os.MkdirAll(config.UploadDir,0755)
	http.Handle("/static/",http.StripPrefix("/static/",http.FileServer(http.Dir("./static"))))
	http.HandleFunc("/login",loginHandler)
	http.HandleFunc("/logout",logoutHandler)
	// API 路由使用权限中间件
	http.HandleFunc("/documents/api", checkPermission("document:manage")(documentsHandler))
	http.HandleFunc("/documents",documentsHandler)
	http.HandleFunc("/projects",projectsHandler)
	http.HandleFunc("/projects/api", checkPermission("project:manage")(projectsHandler))
	http.HandleFunc("/project/",projectDetailHandler)
	http.HandleFunc("/project/stages/api", checkPermission("project:edit")(projectStagesAPI))
	http.HandleFunc("/stage/api", checkPermission("project:edit")(stageAPI))
	http.HandleFunc("/stage/",stageDetailHandler)
	http.HandleFunc("/users/api", checkPermission("user:manage")(usersHandler))
	http.HandleFunc("/users",usersHandler)
	http.HandleFunc("/audit-logs", checkPermission("audit:view")(auditLogsHandler))
	http.HandleFunc("/templates",templatesHandler)
	http.HandleFunc("/menus/api", checkPermission("menu:edit")(menusHandler))
	http.HandleFunc("/menus",menusHandler)
	http.HandleFunc("/functions/api", checkPermission("function:manage")(functionsHandler))
	http.HandleFunc("/functions",functionsHandler)
	http.HandleFunc("/roles/api", checkPermission("role:manage")(rolesHandler))
	http.HandleFunc("/roles/menu", checkPermission("role:menu")(rolesHandler))
	http.HandleFunc("/roles/function", checkPermission("role:function")(rolesHandler))
	http.HandleFunc("/roles",rolesHandler)
	http.HandleFunc("/",homeHandler)
	log.Printf("PMS starting on port %d",config.ServerPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d",config.ServerPort),nil))
}
