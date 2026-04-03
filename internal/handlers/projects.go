package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"pms/internal/audit"
	"pms/internal/models"
)

// Projects 项目管理页与 /projects/api
func (h *Handlers) Projects(w http.ResponseWriter, r *http.Request) {
	u := h.Session.Get(r)
	if u == nil {
		http.Redirect(w, r, "/login", 303)
		return
	}
	if r.URL.Path == "/projects/api" {
		h.handleProjectsAPI(w, r, u)
		return
	}
	var ps []models.Project
	rows, err := h.DB.Query(`SELECT p.id,p.name,p.code,p.status,p.progress,p.start_date,p.end_date,COALESCE(u.real_name,u.username),p.description,p.created_at FROM project p LEFT JOIN user u ON p.manager_id=u.id WHERE p.status != 4 ORDER BY p.created_at DESC`)
	if err == nil {
		for rows.Next() {
			var p models.Project
			rows.Scan(&p.ID, &p.Name, &p.Code, &p.Status, &p.Progress, &p.StartDate, &p.EndDate, &p.ManagerName, &p.Description, &p.CreatedAt)
			ps = append(ps, p)
		}
		rows.Close()
	}
	menus := h.getUserMenus(u)
	tmpl := template.Must(template.ParseFiles("templates/base.html", "templates/projects.html"))
	tmpl.ExecuteTemplate(w, "base.html", map[string]interface{}{"Title": "项目管理", "CurrentUser": u, "Projects": ps, "Menus": menus, "CurrentUrl": "/projects"})
}

func (h *Handlers) handleProjectsAPI(w http.ResponseWriter, r *http.Request, u *models.User) {
	if r.Method == "POST" {
		if !h.hasFunctionPermission(u, "project:create") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "无权创建项目，请联系管理员开通权限"})
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
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "解析失败"})
			return
		}
		if req.TemplateID == 0 {
			req.TemplateID = 1
		}
		res, err := h.DB.Exec(`INSERT INTO project(name,code,template_id,manager_id,status,start_date,end_date,description,created_at) VALUES(?,?,?,?,1,?,?,?,NOW())`,
			req.Name, req.Code, req.TemplateID, u.ID, func() string {
				if req.StartDate == "" {
					return "2026-01-01"
				}
				return req.StartDate
			}(), func() string {
				if req.EndDate == "" {
					return "2026-12-31"
				}
				return req.EndDate
			}(), req.Description)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": err.Error()})
			return
		}
		pid, _ := res.LastInsertId()
		rows, err := h.DB.Query("SELECT name,code,order_num FROM template_stage WHERE template_id=? ORDER BY order_num", req.TemplateID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var n, c string
				var o int
				rows.Scan(&n, &c, &o)
				h.DB.Exec(`INSERT INTO project_stage(project_id,name,code,order_num,status,created_at)VALUES(?,?,?,?,1,NOW())`, pid, n, c, o)
			}
		}
		audit.Log(h.DB, u.ID, "create", "project", "project", pid, "", fmt.Sprintf("name:%s", req.Name), r.RemoteAddr, r.UserAgent(), r.URL.Path, r.Method)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "id": pid})
	} else if r.Method == "PUT" {
		if !h.hasFunctionPermission(u, "project:edit") {
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "无权编辑项目，请联系管理员开通权限"})
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
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "缺少 id"})
			return
		}
		var oldName, oldCode, oldDesc string
		var oldStatus, oldProgress int
		h.DB.QueryRow("SELECT name,code,status,progress,description FROM project WHERE id=?", req.ID).
			Scan(&oldName, &oldCode, &oldStatus, &oldProgress, &oldDesc)
		oldData := map[string]interface{}{"id": req.ID, "name": oldName, "code": oldCode, "status": oldStatus, "progress": oldProgress, "description": oldDesc}
		oldJSONBytes, _ := json.Marshal(oldData)
		oldJSON := string(oldJSONBytes)
		_, err := h.DB.Exec(`UPDATE project SET name=?,code=?,status=?,progress=?,description=?,updated_at=NOW() WHERE id=?`,
			req.Name, req.Code, req.Status, req.Progress, req.Description, req.ID)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": err.Error()})
			return
		}
		go func(uid int, atype, acat, tbl string, rid int64, old, nw, ip, ua, url, meth string) {
			audit.Log(h.DB, uid, atype, acat, tbl, rid, old, nw, ip, ua, url, meth)
		}(u.ID, "update", "project", "project", int64(req.ID), oldJSON, "", r.RemoteAddr, r.UserAgent(), r.URL.Path, r.Method)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	} else if r.Method == "DELETE" {
		if !h.hasFunctionPermission(u, "project:delete") {
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "无权删除项目，请联系管理员开通权限"})
			return
		}
		var req struct {
			ID int `json:"id"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.ID == 0 {
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "缺少 id"})
			return
		}
		var oldName, oldCode, oldDesc string
		var oldStatus int
		h.DB.QueryRow("SELECT name,code,status,description FROM project WHERE id=?", req.ID).
			Scan(&oldName, &oldCode, &oldStatus, &oldDesc)
		oldData := map[string]interface{}{"id": req.ID, "name": oldName, "code": oldCode, "status": oldStatus, "description": oldDesc}
		oldJSONBytes, _ := json.Marshal(oldData)
		oldJSON := string(oldJSONBytes)
		_, err := h.DB.Exec("UPDATE project SET status=4 WHERE id=?", req.ID)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": err.Error()})
			return
		}
		go func(uid int, atype, acat, tbl string, rid int64, old, nw, ip, ua, url, meth string) {
			audit.Log(h.DB, uid, atype, acat, tbl, rid, old, nw, ip, ua, url, meth)
		}(u.ID, "delete", "project", "project", int64(req.ID), oldJSON, "", r.RemoteAddr, r.UserAgent(), r.URL.Path, r.Method)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Method not allowed"})
	}
}

// ProjectDetail /project/{id}
func (h *Handlers) ProjectDetail(w http.ResponseWriter, r *http.Request) {
	u := h.Session.Get(r)
	if u == nil {
		http.Redirect(w, r, "/login", 303)
		return
	}
	idStr := r.URL.Path[len("/project/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		http.Error(w, "项目不存在", http.StatusNotFound)
		return
	}
	var p models.Project
	err = h.DB.QueryRow(`SELECT p.id,p.name,p.code,p.status,p.progress,p.start_date,p.end_date,COALESCE(u.real_name,u.username),p.description FROM project p LEFT JOIN user u ON p.manager_id=u.id WHERE p.id=?`, id).
		Scan(&p.ID, &p.Name, &p.Code, &p.Status, &p.Progress, &p.StartDate, &p.EndDate, &p.ManagerName, &p.Description)
	if err != nil || p.ID == 0 {
		http.Error(w, "项目不存在", http.StatusNotFound)
		return
	}
	var ss []models.ProjectStage
	rows, err := h.DB.Query("SELECT id,name,code,order_num,status,progress,plan_start_date,plan_end_date FROM project_stage WHERE project_id=? ORDER BY order_num", id)
	if err == nil {
		for rows.Next() {
			var s models.ProjectStage
			rows.Scan(&s.ID, &s.Name, &s.Code, &s.OrderNum, &s.Status, &s.Progress, &s.PlanStartDate, &s.PlanEndDate)
			ss = append(ss, s)
		}
		rows.Close()
	}
	menus := h.getUserMenus(u)
	tmpl := template.Must(template.ParseFiles("templates/base.html", "templates/project_detail.html"))
	tmpl.ExecuteTemplate(w, "base.html", map[string]interface{}{"Title": "项目详情", "CurrentUser": u, "Project": p, "Stages": ss, "Menus": menus, "CurrentUrl": "/projects"})
}

// ProjectStagesAPI POST /project/stages/api
func (h *Handlers) ProjectStagesAPI(w http.ResponseWriter, r *http.Request) {
	u := h.Session.Get(r)
	if u == nil {
		http.Error(w, "Unauthorized", 401)
		return
	}
	if !h.hasFunctionPermission(u, "project:edit") {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "无权编辑项目阶段，请联系管理员开通权限"})
		return
	}
	var req struct {
		StageID  int `json:"stage_id"`
		Status   int `json:"status"`
		Progress int `json:"progress"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.StageID == 0 {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "缺少 stage_id"})
		return
	}
	_, err := h.DB.Exec(`UPDATE project_stage SET status=?,progress=?,updated_at=NOW() WHERE id=?`, req.Status, req.Progress, req.StageID)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": err.Error()})
		return
	}
	go func(uid int, atype, acat, tbl string, rid int64, old, nw, ip, ua, url, meth string) {
		audit.Log(h.DB, uid, atype, acat, tbl, rid, old, nw, ip, ua, url, meth)
	}(u.ID, "update", "project_stage", "project_stage", int64(req.StageID), "", "", r.RemoteAddr, r.UserAgent(), r.URL.Path, r.Method)
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// StageDetail /stage/{id}
func (h *Handlers) StageDetail(w http.ResponseWriter, r *http.Request) {
	u := h.Session.Get(r)
	if u == nil {
		http.Redirect(w, r, "/login", 303)
		return
	}
	idStr := r.URL.Path[len("/stage/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		http.Error(w, "阶段不存在", http.StatusNotFound)
		return
	}
	var s models.ProjectStage
	err = h.DB.QueryRow("SELECT id,project_id,name,code,order_num,status,progress,plan_start_date,plan_end_date FROM project_stage WHERE id=?", id).
		Scan(&s.ID, &s.ProjectID, &s.Name, &s.Code, &s.OrderNum, &s.Status, &s.Progress, &s.PlanStartDate, &s.PlanEndDate)
	if err != nil || s.ID == 0 {
		http.Error(w, "阶段不存在", http.StatusNotFound)
		return
	}
	if len(s.PlanStartDate) >= 10 {
		s.PlanStartDate = s.PlanStartDate[:10]
	}
	if len(s.PlanEndDate) >= 10 {
		s.PlanEndDate = s.PlanEndDate[:10]
	}
	var p models.Project
	h.DB.QueryRow("SELECT id,name,code FROM project WHERE id=?", s.ProjectID).
		Scan(&p.ID, &p.Name, &p.Code)
	type Doc struct {
		ID, ProjectID int
		Name, Type, FileName, FilePath, Version, CreatedAtStr, FileSizeStr string
		FileSize int64
		Status, UploadedBy int
	}
	var ds []Doc
	rows, err := h.DB.Query(`SELECT id,project_id,name,type,file_name,file_path,version,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i'),file_size,status,uploaded_by FROM document WHERE stage_id=? AND status=1 ORDER BY created_at DESC`, id)
	if err == nil {
		for rows.Next() {
			var d Doc
			rows.Scan(&d.ID, &d.ProjectID, &d.Name, &d.Type, &d.FileName, &d.FilePath, &d.Version, &d.CreatedAtStr, &d.FileSize, &d.Status, &d.UploadedBy)
			if d.FileSize >= 1048576 {
				d.FileSizeStr = fmt.Sprintf("%.2f MB", float64(d.FileSize)/1048576)
			} else if d.FileSize >= 1024 {
				d.FileSizeStr = fmt.Sprintf("%.2f KB", float64(d.FileSize)/1024)
			} else {
				d.FileSizeStr = fmt.Sprintf("%d B", d.FileSize)
			}
			ds = append(ds, d)
		}
		rows.Close()
	}
	menus := h.getUserMenus(u)
	tmpl := template.Must(template.ParseFiles("templates/base.html", "templates/stage_detail.html"))
	tmpl.ExecuteTemplate(w, "base.html", map[string]interface{}{"Title": "阶段详情", "CurrentUser": u, "Stage": s, "Project": p, "Documents": ds, "Menus": menus, "CurrentUrl": "/projects"})
}

// StageAPI PUT /stage/api
func (h *Handlers) StageAPI(w http.ResponseWriter, r *http.Request) {
	u := h.Session.Get(r)
	if u == nil {
		http.Error(w, "Unauthorized", 401)
		return
	}
	if r.Method == "PUT" {
		if !h.hasFunctionPermission(u, "project:edit") {
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "无权编辑项目阶段，请联系管理员开通权限"})
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
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "解析失败"})
			return
		}
		if req.ID == 0 {
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "缺少 id"})
			return
		}
		_, err := h.DB.Exec(`UPDATE project_stage SET name=?,code=?,order_num=?,status=?,progress=?,plan_start_date=?,plan_end_date=?,updated_at=NOW() WHERE id=?`,
			req.Name, req.Code, req.OrderNum, req.Status, req.Progress, req.PlanStartDate, req.PlanEndDate, req.ID)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": err.Error()})
			return
		}
		go func(uid int, atype, acat, tbl string, rid int64, old, nw, ip, ua, url, meth string) {
			audit.Log(h.DB, uid, atype, acat, tbl, rid, old, nw, ip, ua, url, meth)
		}(u.ID, "update", "project_stage", "project_stage", int64(req.ID), "", "", r.RemoteAddr, r.UserAgent(), r.URL.Path, r.Method)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Method not allowed"})
	}
}
