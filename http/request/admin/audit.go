package admin

type AuditQuery struct {
	PeerId   string `form:"peer_id"`
	FromPeer string `form:"from_peer"`
	PageQuery
}

// AuditTerminalQuery sceglie la trascrizione della connessione con l'id
// audit_conn_id, quello della riga nel registro delle connessioni.
type AuditTerminalQuery struct {
	AuditConnId uint `form:"audit_conn_id" binding:"required"`
	PageQuery
}

type AuditConnLogIds struct {
	Ids []uint `json:"ids" validate:"required"`
}
type AuditFileLogIds struct {
	Ids []uint `json:"ids" validate:"required"`
}
