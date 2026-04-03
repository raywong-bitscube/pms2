package handlers

import (
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"

	"pms/internal/audit"

	"golang.org/x/crypto/bcrypt"
)

func (h *Handlers) Users(w http.ResponseWriter, r *http.Request) {
	u := h.Session.Get(r)
	if u == nil {
		http.Redirect(w, r, "/login", 303)
		return
	}
	if r.URL.Path == "/users/api" {
		if r.Method == "POST" {
			body, errRead := io.ReadAll(r.Body)
			if errRead != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "读取请求失败"})
				return
			}
			var resetReq struct {
				Action string `json:"action"`
				UserID int    `json:"user_id"`
			}
			_ = json.Unmarshal(body, &resetReq)
			if resetReq.Action == "reset_password" && resetReq.UserID > 0 {
				if !h.hasFunctionPermission(u, "user:reset_password") {
					json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "无权重置用户密码，请联系管理员开通权限"})
					return
				}
				hash, _ := bcrypt.GenerateFromPassword([]byte("123"), 12)
				_, err := h.DB.Exec(`UPDATE user SET password_hash=?,updated_at=NOW() WHERE id=?`, hash, resetReq.UserID)
				if err != nil {
					json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "重置失败：" + err.Error()})
					return
				}
				audit.Log(h.DB, u.ID, "reset_password", "user", "user", int64(resetReq.UserID), "", "", r.RemoteAddr, r.UserAgent(), r.URL.Path, r.Method)
				json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "密码已重置为 123"})
				return
			}
			if !h.hasFunctionPermission(u, "user:manage") {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "无权操作用户，请联系管理员开通权限"})
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
			if err := json.Unmarshal(body, &req); err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "解析失败"})
				return
			}
			if req.Username == "" {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "用户名不能为空"})
				return
			}
			if req.ID == 0 {
				if req.Password == "" {
					json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "密码不能为空"})
					return
				}
				hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
				_, err := h.DB.Exec(`INSERT INTO user(username,password_hash,real_name,email,is_admin,status,created_at)VALUES(?,?,?,?,?,?,NOW())`, req.Username, string(hash), req.RealName, req.Email, req.IsAdmin, req.Status)
				if err != nil {
					json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "创建失败：" + err.Error()})
					return
				}
				audit.Log(h.DB, u.ID, "create", "user", "user", 0, "", "", r.RemoteAddr, r.UserAgent(), r.URL.Path, r.Method)
			} else {
				_, err := h.DB.Exec(`UPDATE user SET username=?,real_name=?,email=?,is_admin=?,status=?,updated_at=NOW() WHERE id=?`, req.Username, req.RealName, req.Email, req.IsAdmin, req.Status, req.ID)
				if err != nil {
					json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "更新失败：" + err.Error()})
					return
				}
				audit.Log(h.DB, u.ID, "update", "user", "user", int64(req.ID), "", "", r.RemoteAddr, r.UserAgent(), r.URL.Path, r.Method)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
			return
		}
		if r.Method == "GET" {
			if !h.hasFunctionPermission(u, "user:manage") {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "无权访问"})
				return
			}
			userID := r.URL.Query().Get("user_id")
			type Role struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
				Code string `json:"code"`
			}
			var roles []Role
			rows, err := h.DB.Query(`SELECT id,name,code FROM role WHERE status=1 ORDER BY id`)
			if err == nil {
				for rows.Next() {
					var x Role
					rows.Scan(&x.ID, &x.Name, &x.Code)
					roles = append(roles, x)
				}
				rows.Close()
			}
			userRoles := make([]int, 0)
			rows2, err2 := h.DB.Query(`SELECT role_id FROM user_role WHERE user_id=?`, userID)
			if err2 == nil {
				for rows2.Next() {
					var rid int
					rows2.Scan(&rid)
					userRoles = append(userRoles, rid)
				}
				rows2.Close()
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "roles": roles, "user_roles": userRoles})
			return
		}
		if r.Method == "PUT" {
			if !h.hasFunctionPermission(u, "user:manage") {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "无权分配用户角色，请联系管理员开通权限"})
				return
			}
			var req struct {
				UserID  int   `json:"user_id"`
				RoleIDs []int `json:"role_ids"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			if req.UserID == 0 {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "user_id 不能为空"})
				return
			}
			_, err := h.DB.Exec(`DELETE FROM user_role WHERE user_id=?`, req.UserID)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "删除旧角色失败：" + err.Error()})
				return
			}
			for _, rid := range req.RoleIDs {
				_, err := h.DB.Exec(`INSERT INTO user_role(user_id,role_id)VALUES(?,?)`, req.UserID, rid)
				if err != nil {
					json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "添加角色失败：" + err.Error()})
					return
				}
			}
			audit.Log(h.DB, u.ID, "assign_roles", "user", "user_role", int64(req.UserID), "", "", r.RemoteAddr, r.UserAgent(), r.URL.Path, r.Method)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
			return
		}
	}
	type U struct {
		ID, Status, IsAdmin int
		Username, RealName, Email, CreatedAt string
	}
	var us []U
	rows, err := h.DB.Query("SELECT id,username,real_name,email,is_admin,status,DATE_FORMAT(created_at,'%Y-%m-%d') FROM user ORDER BY created_at DESC")
	if err == nil {
		for rows.Next() {
			var x U
			rows.Scan(&x.ID, &x.Username, &x.RealName, &x.Email, &x.IsAdmin, &x.Status, &x.CreatedAt)
			us = append(us, x)
		}
		rows.Close()
	}
	menus := h.getUserMenus(u)
	funcs := h.getUserFunctions(u)
	tmpl, err := template.ParseFiles("templates/base.html", "templates/users.html")
	if err != nil {
		log.Printf("usersHandler template parse error: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	err = tmpl.ExecuteTemplate(w, "base.html", map[string]interface{}{"Title": "用户管理", "CurrentUser": u, "Users": us, "Menus": menus, "UserFunctions": funcs, "CurrentUrl": "/users"})
	if err != nil {
		log.Printf("usersHandler template exec error: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
}
