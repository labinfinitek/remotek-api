package model

import "github.com/lejianwen/rustdesk-api/v2/model/custom_types"

const (
	AuditActionNew   = "new"
	AuditActionClose = "close"
)

type AuditConn struct {
	IdModel
	Action    string `json:"action" gorm:"default:'';not null;"`
	ConnId    int64  `json:"conn_id" gorm:"default:0;not null;index"`
	PeerId    string `json:"peer_id" gorm:"default:'';not null;index"`
	FromPeer  string `json:"from_peer" gorm:"default:'';not null;"`
	FromName  string `json:"from_name" gorm:"default:'';not null;"`
	Ip        string `json:"ip" gorm:"default:'';not null;"`
	SessionId string `json:"session_id" gorm:"default:'';not null;"`
	Type      int    `json:"type" gorm:"default:0;not null;"`
	Uuid      string `json:"uuid" gorm:"default:'';not null;"`
	CloseTime int64  `json:"close_time" gorm:"default:0;not null;"`
	Note      string `json:"note" gorm:"default:'';not null;"`
	// Guid e' il segreto con cui il tecnico scrive la nota di fine
	// connessione (PUT /api/audit): nel pannello non si vede.
	Guid string `json:"-" gorm:"default:'';not null;index"`
	TimeModel
}

// NotaMax e' la lunghezza massima della nota di sessione, in caratteri.
const NotaMax = 2000

type AuditConnList struct {
	AuditConns []*AuditConn `json:"list"`
	Pagination
}

type AuditFile struct {
	IdModel
	FromPeer string `json:"from_peer" gorm:"default:'';not null;index"`
	Info     string `json:"info" gorm:"default:'';not null;"`
	IsFile   bool   `json:"is_file" gorm:"default:0;not null;"`
	Path     string `json:"path" gorm:"default:'';not null;"`
	PeerId   string `json:"peer_id" gorm:"default:'';not null;index"`
	Type     int    `json:"type" gorm:"default:0;not null;"`
	Uuid     string `json:"uuid" gorm:"default:'';not null;"`
	Ip       string `json:"ip" gorm:"default:'';not null;"`
	Num      int    `json:"num" gorm:"default:0;not null;"`
	FromName string `json:"from_name" gorm:"default:'';not null;"`
	TimeModel
}

type AuditFileList struct {
	AuditFiles []*AuditFile `json:"list"`
	Pagination
}

// AuditConnTerminale e' il type delle connessioni al terminale del PC.
const AuditConnTerminale = 4

// Limiti della trascrizione del terminale: byte di un blocco e di una
// sessione.
const (
	BloccoTerminaleMax    = 64 << 10
	TrascrizioneTerminale = 20 << 20
)

// AuditTerminal e' un blocco della trascrizione di una sessione terminale,
// mandato dal PC controllato (POST /api/audit/terminal). AuditConnId e' la
// riga di audit_conns della sessione: conn_id si ripete dopo un riavvio del
// servizio sul PC e da solo non la identifica. Hash e' l'hash esadecimale
// del blocco, concatenato a quello del blocco prima.
type AuditTerminal struct {
	IdModel
	AuditConnId uint                  `json:"audit_conn_id" gorm:"not null;uniqueIndex:idx_audit_terminal_blocco,priority:1"`
	PeerId      string                `json:"peer_id" gorm:"size:100;not null"`
	ConnId      int64                 `json:"conn_id" gorm:"not null"`
	Seq         int64                 `json:"seq" gorm:"not null;uniqueIndex:idx_audit_terminal_blocco,priority:2"`
	Dir         string                `json:"dir" gorm:"size:3;not null"`
	Data        []byte                `json:"data" gorm:"not null" swaggertype:"string" format:"base64"`
	Hash        string                `json:"hash" gorm:"size:64;not null"`
	Fine        bool                  `json:"fine" gorm:"not null;default:false"`
	CreatedAt   custom_types.AutoTime `json:"created_at" gorm:"type:timestamp;"`
}

type AuditTerminalList struct {
	Blocchi []*AuditTerminal `json:"list"`
	Pagination
}

// AuditTerminalVerifica e' l'esito del ricalcolo della catena di una
// trascrizione: Hash e' quello salvato dell'ultimo blocco, PrimoErrato il
// primo seq che non torna (0 se la catena e' integra).
type AuditTerminalVerifica struct {
	Integra     bool   `json:"integra"`
	Blocchi     int64  `json:"blocchi"`
	Fine        bool   `json:"fine"`
	Hash        string `json:"hash"`
	PrimoErrato int64  `json:"primo_errato"`
}
