package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"

	"pms/internal/audit"
	"pms/internal/models"

	"golang.org/x/crypto/bcrypt"
)

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
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
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "无效请求"})
		return
	}
	if req.Username == "" || req.Password == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "用户名和密码不能为空"})
		return
	}
	var u models.User
	var hash string
	err := h.DB.QueryRow("SELECT id,username,password_hash,real_name,email,is_admin,status FROM user WHERE username=?", req.Username).
		Scan(&u.ID, &u.Username, &hash, &u.RealName, &u.Email, &u.IsAdmin, &u.Status)
	if err != nil || u.Status != 1 || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "用户名或密码错误"})
		return
	}
	h.Session.Set(w, &u)
	audit.Log(h.DB, u.ID, "login", "system", "", 0, "", "", r.RemoteAddr, r.UserAgent(), r.URL.Path, r.Method)
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "user": u})
}

func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	h.Session.Clear(w)
	http.Redirect(w, r, "/login", 303)
}

func (h *Handlers) Home(w http.ResponseWriter, r *http.Request) {
	u := h.Session.Get(r)
	if u == nil {
		http.Redirect(w, r, "/login", 303)
		return
	}
	var tp, ap, td, tu int
	h.DB.QueryRow("SELECT COUNT(*) FROM project WHERE status != 4").Scan(&tp)
	h.DB.QueryRow("SELECT COUNT(*) FROM project WHERE status IN(1,2)").Scan(&ap)
	h.DB.QueryRow("SELECT COUNT(*) FROM document WHERE status=1").Scan(&td)
	h.DB.QueryRow("SELECT COUNT(*) FROM user WHERE status=1").Scan(&tu)
	stats := map[string]int{"TotalProjects": tp, "ActiveProjects": ap, "TotalDocuments": td, "TotalUsers": tu}
	var ps []models.Project
	rows, err := h.DB.Query("SELECT id,name,code,status,progress,created_at FROM project WHERE status != 4 ORDER BY created_at DESC LIMIT 5")
	if err == nil {
		for rows.Next() {
			var p models.Project
			rows.Scan(&p.ID, &p.Name, &p.Code, &p.Status, &p.Progress, &p.CreatedAt)
			ps = append(ps, p)
		}
		rows.Close()
	}
	menus := h.getUserMenus(u)
	tmpl := template.Must(template.ParseFiles("templates/base.html", "templates/home.html"))
	tmpl.ExecuteTemplate(w, "base.html", map[string]interface{}{"Title": "首页", "CurrentUser": u, "Stats": stats, "RecentProjects": ps, "Menus": menus, "CurrentUrl": "/"})
}
