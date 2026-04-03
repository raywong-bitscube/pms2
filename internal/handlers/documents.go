package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"pms/internal/audit"
	"pms/internal/models"
)

func (h *Handlers) Documents(w http.ResponseWriter, r *http.Request) {
	u := h.Session.Get(r)
	if u == nil {
		http.Redirect(w, r, "/login", 303)
		return
	}
	if r.URL.Path == "/documents/api" {
		if r.Method == "POST" {
			r.ParseMultipartForm(32 << 20)
			f, header, e := r.FormFile("file")
			if e != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "上传失败"})
				return
			}
			defer f.Close()
			pid, _ := strconv.Atoi(r.FormValue("project_id"))
			if pid <= 0 {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "project_id 无效"})
				return
			}
			sidStr := r.FormValue("stage_id")
			var sid int
			if sidStr != "" {
				sid, _ = strconv.Atoi(sidStr)
			}
			os.MkdirAll(fmt.Sprintf("%s/%d", h.Config.UploadDir, pid), 0755)
			safeName := filepath.Base(header.Filename)
			fp := fmt.Sprintf("%s/%d/%s", h.Config.UploadDir, pid, safeName)
			dst, err := os.Create(fp)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "创建文件失败"})
				return
			}
			defer dst.Close()
			_, err = io.Copy(dst, f)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "写入文件失败"})
				return
			}
			if sid > 0 {
				_, err = h.DB.Exec(`INSERT INTO document(project_id,stage_id,name,type,file_path,file_name,file_size,status,uploaded_by,created_at)VALUES(?,?,?,?,?,?,?,1,?,NOW())`,
					pid, sid, r.FormValue("name"), r.FormValue("type"), fp, header.Filename, header.Size, u.ID)
			} else {
				_, err = h.DB.Exec(`INSERT INTO document(project_id,name,type,file_path,file_name,file_size,status,uploaded_by,created_at)VALUES(?,?,?,?,?,?,1,?,NOW())`,
					pid, r.FormValue("name"), r.FormValue("type"), fp, header.Filename, header.Size, u.ID)
			}
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "上传失败：" + err.Error()})
				return
			}
			go func(uid int, atype, acat, tbl string, rid int64, old, nw, ip, ua, url, meth string) {
				audit.Log(h.DB, uid, atype, acat, tbl, rid, old, nw, ip, ua, url, meth)
			}(u.ID, "upload", "document", "document", 0, "", "", r.RemoteAddr, r.UserAgent(), r.URL.Path, r.Method)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
		} else if r.Method == "DELETE" {
			var req struct {
				ID int `json:"id"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			if req.ID == 0 {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "缺少 id"})
				return
			}
			var docProjectID, docStageID, docFileSize, docStatus, docUploadedBy int
			var docName, docType, docFileName, docFilePath, docVersion string
			err := h.DB.QueryRow(`SELECT project_id,stage_id,name,type,file_path,file_name,file_size,version,status,uploaded_by FROM document WHERE id=?`, req.ID).
				Scan(&docProjectID, &docStageID, &docName, &docType, &docFilePath, &docFileName, &docFileSize, &docVersion, &docStatus, &docUploadedBy)
			var oldJSON string
			if err == nil {
				oldData := map[string]interface{}{
					"id": req.ID, "project_id": docProjectID, "stage_id": docStageID,
					"name": docName, "type": docType, "file_name": docFileName,
					"file_path": docFilePath, "file_size": docFileSize,
					"version": docVersion, "status": docStatus,
				}
				oldJSONBytes, _ := json.Marshal(oldData)
				oldJSON = string(oldJSONBytes)
			} else {
				oldJSON = fmt.Sprintf("query error:%v", err)
			}
			_, err = h.DB.Exec("UPDATE document SET status=0 WHERE id=?", req.ID)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": err.Error()})
				return
			}
			go func(uid int, atype, acat, tbl string, rid int64, old, nw, ip, ua, url, meth string) {
				audit.Log(h.DB, uid, atype, acat, tbl, rid, old, nw, ip, ua, url, meth)
			}(u.ID, "delete", "document", "document", int64(req.ID), oldJSON, "", r.RemoteAddr, r.UserAgent(), r.URL.Path, r.Method)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
		}
		return
	}
	var ds []models.Document
	rows, err := h.DB.Query(`SELECT d.id,d.project_id,p.name,d.stage_id,IFNULL(s.name,'-'),d.name,d.type,d.file_name,d.file_size,d.version,d.status,d.uploaded_by,IFNULL(u.real_name,u.username),d.created_at FROM document d LEFT JOIN project p ON d.project_id=p.id LEFT JOIN project_stage s ON d.stage_id=s.id LEFT JOIN user u ON d.uploaded_by=u.id WHERE d.status=1 ORDER BY d.created_at DESC`)
	if err == nil {
		for rows.Next() {
			var d models.Document
			rows.Scan(&d.ID, &d.ProjectID, &d.ProjectName, &d.StageID, &d.StageName, &d.Name, &d.Type, &d.FileName, &d.FileSize, &d.Version, &d.Status, &d.UploadedBy, &d.UploadedByName, &d.CreatedAt)
			ds = append(ds, d)
		}
		rows.Close()
	}
	menus := h.getUserMenus(u)
	tmpl := template.Must(template.ParseFiles("templates/base.html", "templates/documents.html"))
	tmpl.ExecuteTemplate(w, "base.html", map[string]interface{}{"Title": "文档管理", "CurrentUser": u, "Documents": ds, "Menus": menus, "CurrentUrl": "/documents"})
}
