package audit

import (
	"database/sql"
	"log"
)

// Log 写入审计日志，行为与原先 auditLog 一致
func Log(database *sql.DB, uid int, atype, acat, tbl string, rid int64, old, nw, ip, ua, url, meth string) {
	if database == nil {
		log.Println("auditLog: db is nil")
		return
	}
	var oldData, newData interface{}
	if old == "" {
		oldData = nil
	} else {
		oldData = old
	}
	if nw == "" {
		newData = nil
	} else {
		newData = nw
	}
	_, err := database.Exec(`INSERT INTO audit_log (user_id,action_type,action_category,table_name,record_id,old_data,new_data,ip_address,user_agent,request_url,request_method,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,NOW())`,
		uid, atype, acat, tbl, rid, oldData, newData, ip, ua, url, meth)
	if err != nil {
		log.Printf("auditLog error: %v\n", err)
	}
}
