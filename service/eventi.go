package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/lejianwen/rustdesk-api/v2/model"
)

// Tipi degli eventi delle connessioni (README, Integrazione).
const (
	EventoConnessioneAperta = "connessione.aperta"
	EventoConnessioneChiusa = "connessione.chiusa"
	EventoConnessioneNota   = "connessione.nota"
)

// schemaEventi e' la versione del corpo degli eventi.
const schemaEventi = 1

// PcEvento e' il PC di un evento: ID, hostname e alias, mai uuid ne' chiavi.
type PcEvento struct {
	Id    string `json:"id"`
	Nome  string `json:"nome"`
	Alias string `json:"alias"`
}

// SessioneEvento e' la connessione di un evento di sessione: Tipo e' il
// type del registro, Tecnico il from_name; Fine e DurataS sono null finche'
// la connessione e' aperta. Mai IP, uuid, guid o session_id.
type SessioneEvento struct {
	ConnId  int64   `json:"conn_id"`
	Tipo    int     `json:"tipo"`
	Tecnico string  `json:"tecnico"`
	Inizio  string  `json:"inizio"`
	Fine    *string `json:"fine"`
	DurataS *int64  `json:"durata_s"`
	Nota    string  `json:"nota"`
}

// corpoEvento e' il corpo comune degli eventi.
type corpoEvento struct {
	Schema   int             `json:"schema"`
	Id       string          `json:"id"`
	Tipo     string          `json:"tipo"`
	Ora      string          `json:"ora"`
	Pc       PcEvento        `json:"pc"`
	Sessione *SessioneEvento `json:"sessione,omitempty"`
}

type EventiService struct {
}

// AccodaSessione accoda l'evento tipo della connessione conn, col PC letto
// dal database; con l'integrazione spenta non fa niente.
func (es *EventiService) AccodaSessione(tipo string, conn *model.AuditConn) error {
	if !Config.Integrazione.Attiva() {
		return nil
	}
	pc, err := pcEvento(conn.PeerId)
	if err != nil {
		return err
	}
	return accoda(tipo, pc, sessioneEvento(conn), time.Now())
}

// pcEvento legge ID, hostname e alias del PC id; un PC che non c'e' ha solo
// l'ID.
func pcEvento(id string) (PcEvento, error) {
	pc := PcEvento{Id: id}
	p, err := AllService.PeerService.FindById(id)
	switch {
	case errors.Is(err, ErrNotFound):
		return pc, nil
	case err != nil:
		return pc, fmt.Errorf("PC dell'evento: %w", err)
	}
	pc.Nome, pc.Alias = p.Hostname, p.Alias
	return pc, nil
}

// sessioneEvento riduce la connessione conn ai campi che escono.
func sessioneEvento(conn *model.AuditConn) *SessioneEvento {
	inizio := time.Time(conn.CreatedAt)
	s := &SessioneEvento{ConnId: conn.ConnId, Tipo: conn.Type, Tecnico: conn.FromName,
		Inizio: ora(inizio), Nota: conn.Note}
	if conn.CloseTime > 0 {
		fine := time.Unix(conn.CloseTime, 0)
		f, d := ora(fine), max(0, conn.CloseTime-inizio.Unix())
		s.Fine, s.DurataS = &f, &d
	}
	return s
}

// ora scrive t in RFC 3339, UTC, al secondo.
func ora(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// accoda salva in coda l'evento tipo, creato all'ora adesso, con un id
// casuale.
func accoda(tipo string, pc PcEvento, sessione *SessioneEvento, adesso time.Time) error {
	id, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("id dell'evento: %w", err)
	}
	corpo, err := json.Marshal(corpoEvento{Schema: schemaEventi, Id: id.String(), Tipo: tipo,
		Ora: ora(adesso), Pc: pc, Sessione: sessione})
	if err != nil {
		return fmt.Errorf("corpo dell'evento %s: %w", tipo, err)
	}
	e := &model.Evento{EventoId: id.String(), Tipo: tipo, Corpo: string(corpo),
		Creato: adesso.Unix(), Prossimo: adesso.Unix()}
	if err := DB.Create(e).Error; err != nil {
		return fmt.Errorf("evento %s in coda: %w", tipo, err)
	}
	return nil
}

// Firma restituisce l'header X-Remotek-Evento-Firma del corpo, mandato al
// secondo UNIX t: "t=<t>,v1=<hex>", con v1 l'HMAC-SHA256 col segreto di
// "<t>." seguito dal corpo esatto.
func Firma(segreto string, t int64, corpo []byte) string {
	ts := strconv.FormatInt(t, 10)
	m := hmac.New(sha256.New, []byte(segreto))
	m.Write([]byte(ts + "."))
	m.Write(corpo)
	return "t=" + ts + ",v1=" + hex.EncodeToString(m.Sum(nil))
}
