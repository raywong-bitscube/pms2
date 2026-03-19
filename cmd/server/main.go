package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

var config = &Config{
	ServerPort: 8808, DBHost: "localhost", DBPort: 3306,
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
	ID int; Username string; RealName string; Email string; Status int
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
	ID int; ProjectID int; Name string; Type string; FileName string
	FileSize int64; Version string; Status int; UploadedBy int; CreatedAt time.Time
}

func initDB() error {
	var err error
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4",
		config.DBUser, config.DBPassword, config.DBHost, config.DBPort, config.DBName)
	db, err = sql.Open("mysql", dsn)
	if err != nil { return err }
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
	if db == nil { return }
	db.Exec(`INSERT INTO audit_log (user_id,action_type,action_category,table_name,record_id,old_data,new_data,ip_address,user_agent,request_url,request_method,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,NOW())`,
		uid,atype,acat,tbl,rid,old,nw,ip,ua,url,meth)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		t, _ := template.ParseFiles("templates/login.html")
		t.Execute(w, nil)
		return
	}
	var req struct{ Username, Password string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"无效请求"})
		return
	}
	var u User; var hash string
	err := db.QueryRow("SELECT id,username,password_hash,real_name,email,status FROM user WHERE username=?", req.Username).
		Scan(&u.ID, &u.Username, &hash, &u.RealName, &u.Email, &u.Status)
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
	db.QueryRow("SELECT COUNT(*) FROM project").Scan(&tp)
	db.QueryRow("SELECT COUNT(*) FROM project WHERE status IN(1,2)").Scan(&ap)
	db.QueryRow("SELECT COUNT(*) FROM document WHERE status=1").Scan(&td)
	db.QueryRow("SELECT COUNT(*) FROM user WHERE status=1").Scan(&tu)
	stats := map[string]int{"TotalProjects":tp,"ActiveProjects":ap,"TotalDocuments":td,"TotalUsers":tu}
	var ps []Project
	rows,_ := db.Query("SELECT id,name,code,status,progress,created_at FROM project ORDER BY created_at DESC LIMIT 5")
	for rows.Next() { var p Project; rows.Scan(&p.ID,&p.Name,&p.Code,&p.Status,&p.Progress,&p.CreatedAt); ps=append(ps,p) }
	rows.Close()
	tmpl := template.Must(template.ParseFiles("templates/base.html","templates/home.html"))
	tmpl.ExecuteTemplate(w,"base.html",map[string]interface{}{"Title":"首页","CurrentUser":u,"Stats":stats,"RecentProjects":ps})
}

func projectsHandler(w http.ResponseWriter, r *http.Request) {
	u := getSession(r)
	if u == nil { http.Redirect(w,r,"/login",303); return }
	
	// API 路由 - 必须放在最前面并 return
	if r.URL.Path == "/projects/api" {
		handleProjectsAPI(w, r, u)
		return
	}
	
	// 页面路由
	var ps []Project
	rows,_ := db.Query(`SELECT p.id,p.name,p.code,p.status,p.progress,p.start_date,p.end_date,COALESCE(u.real_name,u.username),p.description,p.created_at FROM project p LEFT JOIN user u ON p.manager_id=u.id ORDER BY p.created_at DESC`)
	for rows.Next() { var p Project; rows.Scan(&p.ID,&p.Name,&p.Code,&p.Status,&p.Progress,&p.StartDate,&p.EndDate,&p.ManagerName,&p.Description,&p.CreatedAt); ps=append(ps,p) }
	rows.Close()
	tmpl := template.Must(template.ParseFiles("templates/base.html","templates/projects.html"))
	tmpl.ExecuteTemplate(w,"base.html",map[string]interface{}{"Title":"项目管理","CurrentUser":u,"Projects":ps})
}

func handleProjectsAPI(w http.ResponseWriter, r *http.Request, u *User) {
	if r.Method == "POST" {
		var req struct{Name,Code,StartDate,EndDate,Description string; TemplateID int}
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
		var req struct{ID,Status,Progress int;Name,Code,Description string}
		json.NewDecoder(r.Body).Decode(&req)
		db.Exec(`UPDATE project SET name=?,code=?,status=?,progress=?,description=?,updated_at=NOW() WHERE id=?`,
			req.Name,req.Code,req.Status,req.Progress,req.Description,req.ID)
		auditLog(u.ID,"update","project","project",int64(req.ID),"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
		json.NewEncoder(w).Encode(map[string]interface{}{"success":true})
		
	} else if r.Method == "DELETE" {
		var req struct{ID int}
		json.NewDecoder(r.Body).Decode(&req)
		db.Exec("UPDATE project SET status=4 WHERE id=?",req.ID)
		auditLog(u.ID,"delete","project","project",int64(req.ID),"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
		json.NewEncoder(w).Encode(map[string]interface{}{"success":true})
		
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{"success":false,"message":"Method not allowed"})
	}
}

func projectDetailHandler(w http.ResponseWriter, r *http.Request) {
	u := getSession(r)
	if u == nil { http.Redirect(w,r,"/login",303); return }
	id,_ := strconv.Atoi(r.URL.Path[len("/project/"):])
	var p Project
	db.QueryRow(`SELECT p.id,p.name,p.code,p.status,p.progress,p.start_date,p.end_date,COALESCE(u.real_name,u.username),p.description FROM project p LEFT JOIN user u ON p.manager_id=u.id WHERE p.id=?`,id).
		Scan(&p.ID,&p.Name,&p.Code,&p.Status,&p.Progress,&p.StartDate,&p.EndDate,&p.ManagerName,&p.Description)
	var ss []ProjectStage
	rows,_ := db.Query("SELECT id,name,code,order_num,status,progress,plan_start_date,plan_end_date FROM project_stage WHERE project_id=? ORDER BY order_num",id)
	for rows.Next() { var s ProjectStage; rows.Scan(&s.ID,&s.Name,&s.Code,&s.OrderNum,&s.Status,&s.Progress,&s.PlanStartDate,&s.PlanEndDate); ss=append(ss,s) }
	rows.Close()
	tmpl := template.Must(template.ParseFiles("templates/base.html","templates/project_detail.html"))
	tmpl.ExecuteTemplate(w,"base.html",map[string]interface{}{"Title":"项目详情","CurrentUser":u,"Project":p,"Stages":ss})
}

func projectStagesAPI(w http.ResponseWriter, r *http.Request) {
	u := getSession(r)
	if u == nil { http.Error(w,"Unauthorized",401); return }
	var req struct{StageID,Status,Progress int}
	json.NewDecoder(r.Body).Decode(&req)
	db.Exec(`UPDATE project_stage SET status=?,progress=?,updated_at=NOW() WHERE id=?`,req.Status,req.Progress,req.StageID)
	auditLog(u.ID,"update","project_stage","project_stage",int64(req.StageID),"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
	json.NewEncoder(w).Encode(map[string]interface{}{"success":true})
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
			os.MkdirAll(fmt.Sprintf("%s/%d",config.UploadDir,pid),0755)
			fp := fmt.Sprintf("%s/%d/%s",config.UploadDir,pid,h.Filename)
			dst,_ := os.Create(fp); defer dst.Close()
			buf := make([]byte,32<<20); n,_ := f.Read(buf); dst.Write(buf[:n])
			db.Exec(`INSERT INTO document(project_id,name,type,file_path,file_name,file_size,status,uploaded_by,created_at)VALUES(?,?,?,?,?,?,1,?,NOW())`,
				pid,r.FormValue("name"),r.FormValue("type"),fp,h.Filename,h.Size,u.ID)
			auditLog(u.ID,"upload","document","document",0,"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
			json.NewEncoder(w).Encode(map[string]interface{}{"success":true})
		} else if r.Method == "DELETE" {
			var req struct{ID int}
			json.NewDecoder(r.Body).Decode(&req)
			db.Exec("UPDATE document SET status=0 WHERE id=?",req.ID)
			auditLog(u.ID,"delete","document","document",int64(req.ID),"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
			json.NewEncoder(w).Encode(map[string]interface{}{"success":true})
		}
		return
	}
	var ds []Document
	rows,_ := db.Query(`SELECT id,project_id,name,type,file_name,file_size,version,status,uploaded_by,created_at FROM document WHERE status=1 ORDER BY created_at DESC`)
	for rows.Next() { var d Document; rows.Scan(&d.ID,&d.ProjectID,&d.Name,&d.Type,&d.FileName,&d.FileSize,&d.Version,&d.Status,&d.UploadedBy,&d.CreatedAt); ds=append(ds,d) }
	rows.Close()
	tmpl := template.Must(template.ParseFiles("templates/base.html","templates/documents.html"))
	tmpl.ExecuteTemplate(w,"base.html",map[string]interface{}{"Title":"文档管理","CurrentUser":u,"Documents":ds})
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
	u := getSession(r)
	if u == nil { http.Redirect(w,r,"/login",303); return }
	if r.URL.Path == "/users/api" && r.Method == "POST" {
		var req struct{Username,Password,RealName,Email string}
		json.NewDecoder(r.Body).Decode(&req)
		h,_ := bcrypt.GenerateFromPassword([]byte(req.Password),12)
		db.Exec(`INSERT INTO user(username,password_hash,real_name,email,status,created_at)VALUES(?,?,?,?,1,NOW())`,req.Username,string(h),req.RealName,req.Email)
		auditLog(u.ID,"create","user","user",0,"","",r.RemoteAddr,r.UserAgent(),r.URL.Path,r.Method)
		json.NewEncoder(w).Encode(map[string]interface{}{"success":true})
		return
	}
	type U struct{ID,Status int;Username,RealName,Email string;CreatedAt string}
	var us []U
	rows,_ := db.Query("SELECT id,username,real_name,email,status,DATE_FORMAT(created_at,'%Y-%m-%d') FROM user ORDER BY created_at DESC")
	for rows.Next() { var x U; rows.Scan(&x.ID,&x.Username,&x.RealName,&x.Email,&x.Status,&x.CreatedAt); us=append(us,x) }
	rows.Close()
	tmpl := template.Must(template.ParseFiles("templates/base.html","templates/users.html"))
	tmpl.ExecuteTemplate(w,"base.html",map[string]interface{}{"Title":"用户管理","CurrentUser":u,"Users":us})
}

func auditLogsHandler(w http.ResponseWriter, r *http.Request) {
	u := getSession(r)
	if u == nil { http.Redirect(w,r,"/login",303); return }
	type L struct{ID,RecordID int64;Username,ActionType,ActionCategory,TableName,IPAddress,CreatedAt string}
	var ls []L
	rows,_ := db.Query(`SELECT al.id,u.username,al.action_type,al.action_category,al.table_name,al.record_id,al.ip_address,DATE_FORMAT(al.created_at,'%Y-%m-%d %H:%i:%s') FROM audit_log al LEFT JOIN user u ON al.user_id=u.id ORDER BY al.created_at DESC LIMIT 100`)
	for rows.Next() { var x L; rows.Scan(&x.ID,&x.Username,&x.ActionType,&x.ActionCategory,&x.TableName,&x.RecordID,&x.IPAddress,&x.CreatedAt); ls=append(ls,x) }
	rows.Close()
	tmpl := template.Must(template.ParseFiles("templates/base.html","templates/audit_logs.html"))
	tmpl.ExecuteTemplate(w,"base.html",map[string]interface{}{"Title":"审计日志","CurrentUser":u,"Logs":ls})
}

func templatesHandler(w http.ResponseWriter, r *http.Request) {
	u := getSession(r)
	if u == nil { http.Redirect(w,r,"/login",303); return }
	type T struct{ID,StageCount int;Name,Code,Version string}
	var ts []T
	rows,_ := db.Query(`SELECT t.id,t.name,t.code,t.version,COUNT(ts.id) FROM project_template t LEFT JOIN template_stage ts ON t.id=ts.template_id GROUP BY t.id`)
	for rows.Next() { var x T; rows.Scan(&x.ID,&x.Name,&x.Code,&x.Version,&x.StageCount); ts=append(ts,x) }
	rows.Close()
	tmpl := template.Must(template.ParseFiles("templates/base.html","templates/templates.html"))
	tmpl.ExecuteTemplate(w,"base.html",map[string]interface{}{"Title":"项目模板","CurrentUser":u,"Templates":ts})
}

func logoutHandler(w http.ResponseWriter, r *http.Request) { clearSession(w); http.Redirect(w,r,"/login",303) }

func main() {
	if err := initDB(); err != nil { log.Fatalf("DB error:%v",err) }
	log.Printf("Database connected")
	os.MkdirAll(config.UploadDir,0755)
	http.HandleFunc("/login",loginHandler)
	http.HandleFunc("/logout",logoutHandler)
	http.HandleFunc("/projects/api",projectsHandler)
	http.HandleFunc("/",homeHandler)
	http.HandleFunc("/project/",projectDetailHandler)
	http.HandleFunc("/project/stages/api",projectStagesAPI)
	http.HandleFunc("/documents",documentsHandler)
	http.HandleFunc("/users",usersHandler)
	http.HandleFunc("/audit-logs",auditLogsHandler)
	http.HandleFunc("/templates",templatesHandler)
	http.Handle("/static/",http.StripPrefix("/static/",http.FileServer(http.Dir("./static"))))
	log.Printf("PMS starting on port %d",config.ServerPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d",config.ServerPort),nil))
}
