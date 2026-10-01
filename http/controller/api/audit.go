package api

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/lejianwen/rustdesk-api/v2/global"
	request "github.com/lejianwen/rustdesk-api/v2/http/request/api"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

type Audit struct {
}

// AuditConn salva nell'audit l'apertura, l'autenticazione e la chiusura
// delle connessioni al dispositivo.
// @Tags 审计
// @Summary 审计连接
// @Description 审计连接
// @Accept  json
// @Produce  json
// @Param body body request.AuditConnForm true "审计连接"
// @Success 200 {string} string ""
// @Failure 500 {object} response.Response
// @Router /audit/conn [post]
func (a *Audit) AuditConn(c *gin.Context) {
	af := &request.AuditConnForm{}
	err := c.ShouldBindBodyWith(af, binding.JSON)
	if err != nil {
		response.ErrorErr(c, "ParamsError", err)
		return
	}
	if af.Note != nil && af.ConnId == 0 {
		// La nota durante la sessione (golden audit-conn-nota) arriva dal PC
		// del tecnico con id, session_id e note, senza uuid ne' conn_id:
		// si attacca alla connessione gia' registrata e non crea righe
		// (ADR-0019, regola 6).
		notaDiSessione(c, af.Id, strconv.FormatUint(af.SessionId, 10), *af.Note)
		response.Success(c, "")
		return
	}
	if af.Uuid == "" && af.Action == "" && af.ConnId == 0 {
		// Senza nota, uuid, azione ne' conn_id non c'e' niente da scrivere,
		// come prima. Ogni altra richiesta senza uuid passa da
		// dalDispositivo, che la scarta con un warn.
		response.Success(c, "")
		return
	}
	if !dalDispositivo(c, af.Id, af.Uuid) {
		response.Success(c, "")
		return
	}
	ac := af.ToAuditConn()
	switch af.Action {
	case model.AuditActionNew:
		if err := service.AllService.AuditService.CreateAuditConn(ac); err != nil {
			auditNonSalvato(c, err)
		}
	case model.AuditActionClose:
		ex, err := connessioneAudit(c, af.Id, af.ConnId)
		if err == nil {
			ex.CloseTime = time.Now().Unix()
			if err := service.AllService.AuditService.UpdateAuditConn(ex); err != nil {
				auditNonSalvato(c, err)
			}
		}
	case "":
		ex, err := connessioneAudit(c, af.Id, af.ConnId)
		if err == nil {
			up := &model.AuditConn{
				IdModel:   model.IdModel{Id: ex.Id},
				FromPeer:  ac.FromPeer,
				FromName:  ac.FromName,
				SessionId: ac.SessionId,
				Type:      ac.Type,
			}
			if err := service.AllService.AuditService.UpdateAuditConn(up); err != nil {
				auditNonSalvato(c, err)
			}
		}
	}
	response.Success(c, "")
}

// notaDiSessione scrive la nota durante la sessione sulla connessione piu'
// recente del dispositivo peerId con quel sessionId. Se la connessione non
// c'e' scrive una riga di info; il testo della nota non va mai nel log.
func notaDiSessione(c *gin.Context, peerId, sessionId, nota string) {
	err := service.AllService.AuditService.NotaDiSessione(peerId, sessionId, tronca(c, nota))
	switch {
	case errors.Is(err, service.ErrNotFound):
		global.Logger.Per(c.Request.Context()).Infof("%s %s: nota di sessione senza connessione registrata del dispositivo %s, non salvata", c.Request.Method, c.FullPath(), peerId)
	case err != nil:
		auditNonSalvato(c, err)
	}
}

// tronca riduce la nota a model.NotaMax caratteri; se la taglia lo scrive
// nel log con la lunghezza, senza il testo, che e' libero del tecnico e puo'
// avere dati personali.
func tronca(c *gin.Context, nota string) string {
	n := utf8.RuneCountInString(nota)
	if n <= model.NotaMax {
		return nota
	}
	global.Logger.Per(c.Request.Context()).Warnf("%s %s: nota di %d caratteri troncata a %d", c.Request.Method, c.FullPath(), n, model.NotaMax)
	return string([]rune(nota)[:model.NotaMax])
}

// ConnAttiva restituisce al tecnico, come stringa JSON, il guid della
// connessione aperta al dispositivo id con quel session_id e di tipo
// conn_type, con cui manda la nota di fine connessione; "" se non c'e',
// e il client riprova.
// @Tags 审计
// @Summary guid della connessione aperta
// @Produce  json
// @Param id query string true "ID del dispositivo"
// @Param session_id query string true "session_id"
// @Param conn_type query int true "tipo della connessione"
// @Success 200 {string} string ""
// @Failure 400 {object} response.ErrorResponse
// @Router /audit/conn/active [get]
// @Security token
func (a *Audit) ConnAttiva(c *gin.Context) {
	sid, err := strconv.ParseUint(c.Query("session_id"), 10, 64)
	var tipo int
	if err == nil {
		tipo, err = strconv.Atoi(c.Query("conn_type"))
	}
	if err != nil {
		response.ErrorErr(c, "ParamsError", err)
		return
	}
	guid, err := service.AllService.AuditService.GuidConnessioneAperta(c.Query("id"), strconv.FormatUint(sid, 10), tipo)
	if err != nil && !errors.Is(err, service.ErrNotFound) {
		response.ErrorErr(c, "SystemError", err)
		return
	}
	c.JSON(http.StatusOK, guid)
}

// Nota scrive la nota di fine connessione del tecnico sulla connessione col
// guid della richiesta; guid sconosciuto: 400 ItemNotFound.
// @Tags 审计
// @Summary nota di fine connessione
// @Accept  json
// @Produce  json
// @Param body body request.AuditNotaForm true "guid e nota"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse
// @Router /audit [put]
// @Security token
func (a *Audit) Nota(c *gin.Context) {
	f := &request.AuditNotaForm{}
	if err := c.ShouldBindBodyWith(f, binding.JSON); err != nil {
		response.ErrorErr(c, "ParamsError", err)
		return
	}
	if err := service.AllService.AuditService.NotaPerGuid(f.Guid, tronca(c, f.Note)); err != nil {
		response.ErrorErr(c, "SystemError", err)
		return
	}
	response.Success(c, "")
}

// connessioneAudit legge la connessione connId del dispositivo peerId. Se il
// database non la legge scrive l'errore nel log con auditNonSalvato; una
// connessione che non c'e' non va nel log, come prima: il client puo'
// chiudere o annotare una connessione di cui l'audit non ha l'apertura.
func connessioneAudit(c *gin.Context, peerId string, connId int64) (*model.AuditConn, error) {
	ex, err := service.AllService.AuditService.InfoByPeerIdAndConnId(peerId, connId)
	if err != nil && !errors.Is(err, service.ErrNotFound) {
		auditNonSalvato(c, err)
	}
	return ex, err
}

// dalDispositivo dice se l'audit arriva dal PC salvato con l'ID id e
// l'uuid uuid, l'unico che puo' scriverne il registro (REM-2026-002).
// Altrimenti scrive nel log perche' l'audit e' scartato: warn con rotta e
// ID, niente uuid, o l'errore del database con auditNonSalvato. Al client
// va successo lo stesso: ignora la risposta.
func dalDispositivo(c *gin.Context, id, uuid string) bool {
	peer, esito, err := service.AllService.PeerService.Riconosci(id, uuid)
	switch {
	case err != nil:
		auditNonSalvato(c, err)
		return false
	case esito == service.PcSconosciuto:
		dispositivoDiverso(c, id, "nessun PC salvato con questo ID")
		return false
	case esito == service.UuidDiverso:
		// Come nell'heartbeat: un PC creato dal pannello senza uuid si lega
		// al primo sysinfo (ADR-0019), e il motivo nel log lo dice.
		perche := "uuid diverso da quello salvato"
		if peer.Uuid == "" {
			perche = "PC senza uuid, in attesa del primo sysinfo"
		}
		dispositivoDiverso(c, id, perche)
		return false
	}
	return true
}

// auditNonSalvato scrive nel log, a livello error, l'audit che il database
// non ha salvato. Al client va successo lo stesso: ignora la risposta, e un
// errore non gli farebbe rimandare niente.
func auditNonSalvato(c *gin.Context, err error) {
	global.Logger.Per(c.Request.Context()).Errorf("%s %s: audit non salvato: %v", c.Request.Method, c.FullPath(), err)
}

// AuditFile salva nell'audit un trasferimento di file.
// @Tags 审计
// @Summary 审计文件
// @Description 审计文件
// @Accept  json
// @Produce  json
// @Param body body request.AuditFileForm true "审计文件"
// @Success 200 {string} string ""
// @Failure 500 {object} response.Response
// @Router /audit/file [post]
func (a *Audit) AuditFile(c *gin.Context) {
	aff := &request.AuditFileForm{}
	err := c.ShouldBindBodyWith(aff, binding.JSON)
	if err != nil {
		response.ErrorErr(c, "ParamsError", err)
		return
	}
	if !dalDispositivo(c, aff.Id, aff.Uuid) {
		response.Success(c, "")
		return
	}
	af := aff.ToAuditFile()
	if err := service.AllService.AuditService.CreateAuditFile(af); err != nil {
		auditNonSalvato(c, err)
	}
	response.Success(c, "")
}

// AuditTerminal salva un blocco della trascrizione di una sessione
// terminale, mandato dal PC controllato. Un blocco che non passa i
// controlli non si salva e va nel log a livello warn, con rotta e ID del PC
// e senza contenuto; al client va successo lo stesso, come nelle altre
// rotte dell'audit.
// @Tags 审计
// @Summary blocco della trascrizione del terminale
// @Accept  json
// @Produce  json
// @Param body body request.AuditTerminalForm true "blocco"
// @Success 200 {string} string ""
// @Failure 400 {object} response.ErrorResponse
// @Router /audit/terminal [post]
func (a *Audit) AuditTerminal(c *gin.Context) {
	f := &request.AuditTerminalForm{}
	if err := c.ShouldBindBodyWith(f, binding.JSON); err != nil {
		response.ErrorErr(c, "ParamsError", err)
		return
	}
	if dalDispositivo(c, f.Id, f.Uuid) {
		salvaBlocco(c, f)
	}
	response.Success(c, "")
}

// salvaBlocco controlla il blocco f, nell'ordine del README, e lo salva.
func salvaBlocco(c *gin.Context, f *request.AuditTerminalForm) {
	scarta := func(perche string) { dispositivoDiverso(c, f.Id, perche) }
	as := service.AllService.AuditService
	conn, err := as.ConnTerminale(f.Id, f.ConnId)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			scarta("nessuna connessione terminale con questo conn_id")
		} else {
			auditNonSalvato(c, err)
		}
		return
	}
	if f.Dir != "in" && f.Dir != "out" {
		scarta("dir sconosciuto")
		return
	}
	// Prima di decodificare si scarta cio' che non puo' stare nel limite;
	// EncodedLen arrotonda a gruppi di 3 byte, quindi dopo si ricontrolla.
	if len(f.Data) > base64.StdEncoding.EncodedLen(model.BloccoTerminaleMax) {
		scarta("blocco oltre 64 KiB")
		return
	}
	data, err := base64.StdEncoding.DecodeString(f.Data)
	if err != nil {
		scarta("data non e' base64")
		return
	}
	if len(data) > model.BloccoTerminaleMax {
		scarta("blocco oltre 64 KiB")
		return
	}
	if len(data) == 0 && !f.Fine {
		scarta("blocco vuoto senza fine")
		return
	}
	// Con seq consecutivo da 1, seq e' il numero dei blocchi: il tetto si
	// controlla senza leggere il database.
	if f.Seq > model.BlocchiTerminaleMax {
		scarta("trascrizione oltre 100000 blocchi")
		return
	}
	ultimo, totale, err := as.UltimoBlocco(conn.Id)
	if err != nil {
		auditNonSalvato(c, err)
		return
	}
	prec := make([]byte, sha256.Size)
	atteso := int64(1)
	if ultimo != nil {
		atteso = ultimo.Seq + 1
		if prec, err = hex.DecodeString(ultimo.Hash); err != nil {
			auditNonSalvato(c, fmt.Errorf("hash del blocco %d: %w", ultimo.Seq, err))
			return
		}
	}
	switch {
	case f.Seq != atteso:
		scarta(fmt.Sprintf("seq %d invece di %d", f.Seq, atteso))
	case hex.EncodeToString(service.HashBlocco(prec, f.Dir, data)) != f.Hash:
		scarta(fmt.Sprintf("hash del blocco %d sbagliato", f.Seq))
	case ultimo != nil && ultimo.Fine:
		scarta(fmt.Sprintf("blocco %d dopo la fine della sessione", f.Seq))
	case totale+int64(len(data)) > model.TrascrizioneTerminale:
		scarta("trascrizione oltre 20 MiB")
	default:
		totale += int64(len(data))
		b := &model.AuditTerminal{AuditConnId: conn.Id, PeerId: f.Id, ConnId: f.ConnId, Seq: f.Seq, Dir: f.Dir, Data: data, Hash: f.Hash, Fine: f.Fine, Totale: &totale}
		if err := as.CreateBlocco(b); err != nil {
			auditNonSalvato(c, err)
		}
	}
}
