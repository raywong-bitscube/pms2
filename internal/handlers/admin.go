package handlers

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"

	"pms/internal/audit"
)

func (h *Handlers) AuditLogs(w http.ResponseWriter, r *http.Request) {
	u := h.Session.Get(r)
	if u == nil {
		http.Redirect(w, r, "/login", 303)
		return
	}
	type L struct {
		ID, RecordID        int64
		Username            string
		ActionType          string
		ActionCategory      string
		TableName           string
		IPAddress           string
		UserAgent           string
		RequestURL          string
		RequestMethod       string
		OldData, NewData    string
		CreatedAt           string
	}
	var ls []L
	rows, err := h.DB.Query(`SELECT al.id,u.username,al.action_type,al.action_category,al.table_name,al.record_id,al.ip_address,al.user_agent,al.request_url,al.request_method,IFNULL(al.old_data,''),IFNULL(al.new_data,''),DATE_FORMAT(al.created_at,'%Y-%m-%d %H:%i:%s') FROM audit_log al LEFT JOIN user u ON al.user_id=u.id ORDER BY al.created_at DESC LIMIT 100`)
	if err == nil {
		for rows.Next() {
			var x L
			rows.Scan(&x.ID, &x.Username, &x.ActionType, &x.ActionCategory, &x.TableName, &x.RecordID, &x.IPAddress, &x.UserAgent, &x.RequestURL, &x.RequestMethod, &x.OldData, &x.NewData, &x.CreatedAt)
			ls = append(ls, x)
		}
		rows.Close()
	}
	menus := h.getUserMenus(u)
	tmpl := template.Must(template.ParseFiles("templates/base.html", "templates/audit_logs.html"))
	tmpl.ExecuteTemplate(w, "base.html", map[string]interface{}{"Title": "审计日志", "CurrentUser": u, "Logs": ls, "Menus": menus, "CurrentUrl": "/audit-logs"})
}

// ProjectTemplates 项目模板页 /templates
func (h *Handlers) ProjectTemplates(w http.ResponseWriter, r *http.Request) {
	u := h.Session.Get(r)
	if u == nil {
		http.Redirect(w, r, "/login", 303)
		return
	}
	type T struct {
		ID, StageCount int
		Name, Code, Version string
	}
	var ts []T
	rows, err := h.DB.Query(`SELECT t.id,t.name,t.code,t.version,COUNT(ts.id) FROM project_template t LEFT JOIN template_stage ts ON t.id=ts.template_id GROUP BY t.id`)
	if err == nil {
		for rows.Next() {
			var x T
			rows.Scan(&x.ID, &x.Name, &x.Code, &x.Version, &x.StageCount)
			ts = append(ts, x)
		}
		rows.Close()
	}
	menus := h.getUserMenus(u)
	tmpl := template.Must(template.ParseFiles("templates/base.html", "templates/templates.html"))
	tmpl.ExecuteTemplate(w, "base.html", map[string]interface{}{"Title": "项目模板", "CurrentUser": u, "Templates": ts, "Menus": menus, "CurrentUrl": "/templates"})
}

func (h *Handlers) Menus(w http.ResponseWriter, r *http.Request) {
	u := h.Session.Get(r)
	if u == nil {
		http.Redirect(w, r, "/login", 303)
		return
	}
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
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "菜单名称不能为空"})
				return
			}
			if req.ID == 0 {
				_, err := h.DB.Exec(`INSERT INTO menu(name,icon,url,parent_id,order_num,status,created_at)VALUES(?,?,?,?,?,1,NOW())`, req.Name, req.Icon, req.Url, req.ParentID, req.OrderNum)
				if err != nil {
					json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "创建失败：" + err.Error()})
					return
				}
				audit.Log(h.DB, u.ID, "create", "menu", "menu", 0, "", "", r.RemoteAddr, r.UserAgent(), r.URL.Path, r.Method)
			} else {
				_, err := h.DB.Exec(`UPDATE menu SET name=?,icon=?,url=?,parent_id=?,order_num=?,status=?,updated_at=NOW() WHERE id=?`, req.Name, req.Icon, req.Url, req.ParentID, req.OrderNum, req.Status, req.ID)
				if err != nil {
					json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "更新失败：" + err.Error()})
					return
				}
				audit.Log(h.DB, u.ID, "update", "menu", "menu", int64(req.ID), "", "", r.RemoteAddr, r.UserAgent(), r.URL.Path, r.Method)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
			return
		}
		if r.Method == "DELETE" {
			var req struct {
				ID int `json:"id"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			if req.ID == 0 {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "id 不能为空"})
				return
			}
			_, err := h.DB.Exec(`DELETE FROM menu WHERE id=?`, req.ID)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "删除失败：" + err.Error()})
				return
			}
			audit.Log(h.DB, u.ID, "delete", "menu", "menu", int64(req.ID), "", "", r.RemoteAddr, r.UserAgent(), r.URL.Path, r.Method)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
			return
		}
	}
	type M struct {
		ID, ParentID, OrderNum, Status, Level int
		Name, Icon, Url, ParentName            string
	}
	var ms []M
	rows, err := h.DB.Query(`SELECT m.id,m.name,m.icon,m.url,m.parent_id,m.order_num,m.status,IFNULL(p.name,'') FROM menu m LEFT JOIN menu p ON m.parent_id=p.id ORDER BY m.parent_id,m.order_num`)
	if err == nil {
		for rows.Next() {
			var x M
			rows.Scan(&x.ID, &x.Name, &x.Icon, &x.Url, &x.ParentID, &x.OrderNum, &x.Status, &x.ParentName)
			if x.ParentID == 0 {
				x.Level = 0
			} else {
				x.Level = 1
			}
			ms = append(ms, x)
		}
		rows.Close()
	}
	navMenus := h.getUserMenus(u)
	tmpl := template.Must(template.ParseFiles("templates/base.html", "templates/menus.html"))
	tmpl.ExecuteTemplate(w, "base.html", map[string]interface{}{"Title": "菜单管理", "CurrentUser": u, "Menus": navMenus, "MenuList": ms, "CurrentUrl": "/menus"})
}

func (h *Handlers) Roles(w http.ResponseWriter, r *http.Request) {
	u := h.Session.Get(r)
	if u == nil {
		http.Redirect(w, r, "/login", 303)
		return
	}
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
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "角色名称和标识不能为空"})
				return
			}
			if req.ID == 0 {
				_, err := h.DB.Exec(`INSERT INTO role(name,code,description,status,created_at)VALUES(?,?,?,?,NOW())`, req.Name, req.Code, req.Description, req.Status)
				if err != nil {
					json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "创建失败：" + err.Error()})
					return
				}
				audit.Log(h.DB, u.ID, "create", "role", "role", 0, "", "", r.RemoteAddr, r.UserAgent(), r.URL.Path, r.Method)
			} else {
				_, err := h.DB.Exec(`UPDATE role SET name=?,code=?,description=?,status=?,updated_at=NOW() WHERE id=?`, req.Name, req.Code, req.Description, req.Status, req.ID)
				if err != nil {
					json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "更新失败：" + err.Error()})
					return
				}
				audit.Log(h.DB, u.ID, "update", "role", "role", int64(req.ID), "", "", r.RemoteAddr, r.UserAgent(), r.URL.Path, r.Method)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
			return
		}
		if r.Method == "DELETE" {
			var req struct {
				ID int `json:"id"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			if req.ID == 0 {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "id 不能为空"})
				return
			}
			_, err := h.DB.Exec(`DELETE FROM role WHERE id=?`, req.ID)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "删除角色失败：" + err.Error()})
				return
			}
			h.DB.Exec(`DELETE FROM role_menu WHERE role_id=?`, req.ID)
			h.DB.Exec(`DELETE FROM role_function WHERE role_id=?`, req.ID)
			audit.Log(h.DB, u.ID, "delete", "role", "role", int64(req.ID), "", "", r.RemoteAddr, r.UserAgent(), r.URL.Path, r.Method)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
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
		rows, err := h.DB.Query(`SELECT id,name,icon FROM menu ORDER BY order_num`)
		if err == nil {
			for rows.Next() {
				var x M
				rows.Scan(&x.ID, &x.Name, &x.Icon)
				ms = append(ms, x)
			}
			rows.Close()
		}
		roleMenus := make([]int, 0)
		rows2, err2 := h.DB.Query(`SELECT menu_id FROM role_menu WHERE role_id=?`, roleID)
		if err2 == nil {
			for rows2.Next() {
				var mid int
				rows2.Scan(&mid)
				roleMenus = append(roleMenus, mid)
			}
			rows2.Close()
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "menus": ms, "role_menus": roleMenus})
		return
	}
	if r.URL.Path == "/roles/menu" && r.Method == "POST" {
		var req struct {
			RoleID  int   `json:"role_id"`
			MenuIDs []int `json:"menu_ids"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.RoleID == 0 {
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "role_id 不能为空"})
			return
		}
		_, err := h.DB.Exec(`DELETE FROM role_menu WHERE role_id=?`, req.RoleID)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "删除旧权限失败：" + err.Error()})
			return
		}
		for _, mid := range req.MenuIDs {
			_, err := h.DB.Exec(`INSERT INTO role_menu(role_id,menu_id)VALUES(?,?)`, req.RoleID, mid)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "添加权限失败：" + err.Error()})
				return
			}
		}
		audit.Log(h.DB, u.ID, "assign_menu_perms", "role", "role_menu", int64(req.RoleID), "", "", r.RemoteAddr, r.UserAgent(), r.URL.Path, r.Method)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
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
		rows, err := h.DB.Query(`SELECT f.id,f.name,f.code,m.name,m.icon FROM system_function f LEFT JOIN menu m ON f.menu_id=m.id ORDER BY m.order_num,f.id`)
		if err == nil {
			for rows.Next() {
				var x F
				rows.Scan(&x.ID, &x.Name, &x.Code, &x.MenuName, &x.MenuIcon)
				fs = append(fs, x)
			}
			rows.Close()
		}
		roleFuncs := make([]int, 0)
		rows2, err2 := h.DB.Query(`SELECT function_id FROM role_function WHERE role_id=?`, roleID)
		if err2 == nil {
			for rows2.Next() {
				var fid int
				rows2.Scan(&fid)
				roleFuncs = append(roleFuncs, fid)
			}
			rows2.Close()
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "functions": fs, "role_funcs": roleFuncs})
		return
	}
	if r.URL.Path == "/roles/function" && r.Method == "POST" {
		var req struct {
			RoleID  int   `json:"role_id"`
			FuncIDs []int `json:"function_ids"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.RoleID == 0 {
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "role_id 不能为空"})
			return
		}
		_, err := h.DB.Exec(`DELETE FROM role_function WHERE role_id=?`, req.RoleID)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "删除旧权限失败：" + err.Error()})
			return
		}
		for _, fid := range req.FuncIDs {
			_, err := h.DB.Exec(`INSERT INTO role_function(role_id,function_id)VALUES(?,?)`, req.RoleID, fid)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "添加权限失败：" + err.Error()})
				return
			}
		}
		audit.Log(h.DB, u.ID, "assign_func_perms", "role", "role_function", int64(req.RoleID), "", "", r.RemoteAddr, r.UserAgent(), r.URL.Path, r.Method)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
		return
	}
	type R struct {
		ID, Status int
		Name, Code, Description string
	}
	var rs []R
	rows, err := h.DB.Query(`SELECT id,name,code,description,status FROM role ORDER BY id`)
	if err == nil {
		for rows.Next() {
			var x R
			rows.Scan(&x.ID, &x.Name, &x.Code, &x.Description, &x.Status)
			rs = append(rs, x)
		}
		rows.Close()
	}
	menus := h.getUserMenus(u)
	tmpl := template.Must(template.ParseFiles("templates/base.html", "templates/roles.html"))
	tmpl.ExecuteTemplate(w, "base.html", map[string]interface{}{"Title": "角色管理", "CurrentUser": u, "Roles": rs, "Menus": menus, "CurrentUrl": "/roles"})
}

func (h *Handlers) Functions(w http.ResponseWriter, r *http.Request) {
	u := h.Session.Get(r)
	if u == nil {
		http.Redirect(w, r, "/login", 303)
		return
	}
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
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "功能名称和标识不能为空"})
				return
			}
			if req.ID == 0 {
				_, err := h.DB.Exec(`INSERT INTO system_function(menu_id,name,code,url,method,description,status,created_at)VALUES(?,?,?,?,?,?,1,NOW())`, req.MenuID, req.Name, req.Code, req.Url, req.Method, req.Description)
				if err != nil {
					json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "创建失败：" + err.Error()})
					return
				}
				audit.Log(h.DB, u.ID, "create", "function", "system_function", 0, "", "", r.RemoteAddr, r.UserAgent(), r.URL.Path, r.Method)
			} else {
				_, err := h.DB.Exec(`UPDATE system_function SET menu_id=?,name=?,code=?,url=?,method=?,description=?,status=?,updated_at=NOW() WHERE id=?`, req.MenuID, req.Name, req.Code, req.Url, req.Method, req.Description, req.Status, req.ID)
				if err != nil {
					json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "更新失败：" + err.Error()})
					return
				}
				audit.Log(h.DB, u.ID, "update", "function", "system_function", int64(req.ID), "", "", r.RemoteAddr, r.UserAgent(), r.URL.Path, r.Method)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
			return
		}
		if r.Method == "DELETE" {
			var req struct {
				ID int `json:"id"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			if req.ID == 0 {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "id 不能为空"})
				return
			}
			_, err := h.DB.Exec(`DELETE FROM system_function WHERE id=?`, req.ID)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "删除失败：" + err.Error()})
				return
			}
			audit.Log(h.DB, u.ID, "delete", "function", "system_function", int64(req.ID), "", "", r.RemoteAddr, r.UserAgent(), r.URL.Path, r.Method)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
			return
		}
	}
	type F struct {
		ID, MenuID, Status                     int
		Name, Code, MenuName, Url, Method, Description string
	}
	var fs []F
	rows, err := h.DB.Query(`SELECT f.id,f.menu_id,f.name,f.code,m.name,f.url,f.method,f.description,f.status FROM system_function f LEFT JOIN menu m ON f.menu_id=m.id ORDER BY f.menu_id,f.id`)
	if err == nil {
		for rows.Next() {
			var x F
			rows.Scan(&x.ID, &x.MenuID, &x.Name, &x.Code, &x.MenuName, &x.Url, &x.Method, &x.Description, &x.Status)
			fs = append(fs, x)
		}
		rows.Close()
	}
	type M struct {
		ID   int
		Name string
	}
	var ms []M
	rows2, err2 := h.DB.Query(`SELECT id,name FROM menu WHERE status=1 ORDER BY order_num`)
	if err2 == nil {
		for rows2.Next() {
			var x M
			rows2.Scan(&x.ID, &x.Name)
			ms = append(ms, x)
		}
		rows2.Close()
	}
	menus := h.getUserMenus(u)
	tmpl := template.Must(template.ParseFiles("templates/base.html", "templates/functions.html"))
	if err := tmpl.ExecuteTemplate(w, "base.html", map[string]interface{}{"Title": "功能管理", "CurrentUser": u, "Functions": fs, "Menus": menus, "MenuList": ms, "CurrentUrl": "/functions"}); err != nil {
		log.Printf("functions template exec: %v", err)
	}
}
