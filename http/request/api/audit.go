package api

import (
	"encoding/json"
	"strconv"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/model"
)

type AuditConnForm struct {
	Action    string   `json:"action"`
	ConnId    int64    `json:"conn_id"`
	Id        string   `json:"id"`
	Peer      []string `json:"peer"`
	Ip        string   `json:"ip"`
	SessionId uint64   `json:"session_id"` // u64 casuale nel client: un float64 lo arrotonda sopra 2^53
	Type      int      `json:"type"`
	Uuid      string   `json:"uuid"`
	Note      *string  `json:"note"` // c'e' solo nella nota durante la sessione
}

// AuditNotaForm e' la nota di fine connessione del tecnico (PUT /api/audit).
type AuditNotaForm struct {
	Guid string `json:"guid"`
	Note string `json:"note"`
}

func (a *AuditConnForm) ToAuditConn() *model.AuditConn {
	fp := ""
	fn := ""
	if len(a.Peer) >= 1 {
		fp = a.Peer[0]
		if len(a.Peer) == 2 {
			fn = a.Peer[1]
		}
	}
	ssid := strconv.FormatUint(a.SessionId, 10)
	return &model.AuditConn{
		Action:    a.Action,
		ConnId:    a.ConnId,
		PeerId:    a.Id,
		FromPeer:  fp,
		FromName:  fn,
		Ip:        a.Ip,
		SessionId: ssid,
		Type:      a.Type,
		Uuid:      a.Uuid,
	}
}

type AuditFileForm struct {
	Id     string `json:"id"`
	Info   string `json:"info"`
	IsFile bool   `json:"is_file"`
	Path   string `json:"path"`
	PeerId string `json:"peer_id"`
	Type   int    `json:"type"`
	Uuid   string `json:"uuid"`
}
type AuditFileInfo struct {
	Ip   string `json:"ip"`
	Name string `json:"name"`
	Num  int    `json:"num"`
}

func (a *AuditFileForm) ToAuditFile() *model.AuditFile {
	fi := &AuditFileInfo{}
	err := json.Unmarshal([]byte(a.Info), fi)
	if err != nil {
		global.Logger.Warn("ToAuditFile", err)
	}

	return &model.AuditFile{
		PeerId:   a.Id,
		Info:     a.Info,
		IsFile:   a.IsFile,
		FromPeer: a.PeerId,
		Path:     a.Path,
		Type:     a.Type,
		Uuid:     a.Uuid,
		FromName: fi.Name,
		Ip:       fi.Ip,
		Num:      fi.Num,
	}
}

// AuditTerminalForm e' un blocco della trascrizione di una sessione
// terminale (POST /api/audit/terminal); il formato e' nel README.
type AuditTerminalForm struct {
	Id     string `json:"id"`
	Uuid   string `json:"uuid"`
	ConnId int64  `json:"conn_id"`
	Seq    int64  `json:"seq"`
	Dir    string `json:"dir"`
	Data   string `json:"data"` // base64 standard
	Hash   string `json:"hash"`
	Fine   bool   `json:"fine"`
}
