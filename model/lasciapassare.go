package model

// Lasciapassare e' il registro dei lasciapassare rilasciati agli agenti AI
// (ADR-0028, versione del database 273). Mai il lasciapassare intero ne' la
// firma: solo il nonce, che il PC rimanda quando apre la sessione. Le ore
// sono secondi UNIX. ConnId (il conn_id del client, che si ripete a ogni
// riavvio del PC) e AuditConnId (l'id della riga di audit_conns, versione
// del database 275) sono 0 finche' il lasciapassare non apre una
// connessione: il legame che vale e' AuditConnId.
type Lasciapassare struct {
	IdModel
	Nonce       string `gorm:"size:32;not null;uniqueIndex"`
	Pc          string `gorm:"size:100;not null;index"`
	AgenteId    uint   `gorm:"not null;index"`
	TecnicoId   uint   `gorm:"not null"`
	Emesso      int64  `gorm:"not null"`
	Scade       int64  `gorm:"not null"`
	ConnId      int64  `gorm:"not null;default:0"`
	AuditConnId uint   `gorm:"not null;default:0;index"`
	Creato      int64  `gorm:"not null"`
}

// TableName e' lasciapassare.
func (Lasciapassare) TableName() string { return "lasciapassare" }
