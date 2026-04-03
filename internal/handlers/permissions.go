package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"pms/internal/models"
)

func (h *Handlers) getUserMenus(u *models.User) []map[string]interface{} {
	var menus []map[string]interface{}
	var rows *sql.Rows
	var err error
	if u.IsAdmin == 1 {
		rows, err = h.DB.Query(`SELECT id,name,icon,url,parent_id,order_num FROM menu WHERE status=1 ORDER BY parent_id,order_num`)
	} else {
		rows, err = h.DB.Query(`SELECT DISTINCT m.id,m.name,m.icon,m.url,m.parent_id,m.order_num FROM menu m INNER JOIN role_menu rm ON m.id=rm.menu_id INNER JOIN user_role ur ON rm.role_id=ur.role_id WHERE ur.user_id=? AND m.status=1 ORDER BY m.parent_id,m.order_num`, u.ID)
	}
	if err != nil {
		return menus
	}
	defer rows.Close()
	for rows.Next() {
		var id, parentID, orderNum int
		var name, icon, url string
		rows.Scan(&id, &name, &icon, &url, &parentID, &orderNum)
		menus = append(menus, map[string]interface{}{
			"ID": id, "Name": name, "Icon": icon, "Url": url, "ParentID": parentID, "OrderNum": orderNum,
		})
	}
	return menus
}

func (h *Handlers) getUserFunctions(u *models.User) map[string]bool {
	funcs := make(map[string]bool)
	var rows *sql.Rows
	var err error
	if u.IsAdmin == 1 {
		rows, err = h.DB.Query(`SELECT code FROM system_function WHERE status=1`)
	} else {
		rows, err = h.DB.Query(`SELECT DISTINCT f.code FROM system_function f INNER JOIN role_function rf ON f.id=rf.function_id INNER JOIN user_role ur ON rf.role_id=ur.role_id WHERE ur.user_id=? AND f.status=1`, u.ID)
	}
	if err != nil {
		return funcs
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		rows.Scan(&code)
		funcs[code] = true
	}
	return funcs
}

func (h *Handlers) hasFunctionPermission(u *models.User, funcCode string) bool {
	if u.IsAdmin == 1 {
		return true
	}
	var count int
	h.DB.QueryRow(`SELECT COUNT(*) FROM system_function f INNER JOIN role_function rf ON f.id=rf.function_id INNER JOIN user_role ur ON rf.role_id=ur.role_id WHERE ur.user_id=? AND f.code=? AND f.status=1`, u.ID, funcCode).Scan(&count)
	return count > 0
}

// CheckPermission 与原 checkPermission 一致
func (h *Handlers) CheckPermission(funcCode string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			u := h.Session.Get(r)
			if u == nil {
				http.Redirect(w, r, "/login", 303)
				return
			}
			if !h.hasFunctionPermission(u, funcCode) {
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
