package api

import (
	"errors"
	"time"

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
	if af.Uuid == "" && af.Action == "" && af.ConnId == 0 {
		// La nota durante la sessione (golden audit-conn-nota: id,
		// session_id e note, senza uuid, azione ne' conn_id) non scrive
		// niente, come prima; e' un altro compito. Ogni altra richiesta
		// senza uuid passa da dalDispositivo, che la scarta con un warn.
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
	_, esito, err := service.AllService.PeerService.Riconosci(id, uuid)
	switch {
	case err != nil:
		auditNonSalvato(c, err)
		return false
	case esito == service.PcSconosciuto:
		dispositivoDiverso(c, id, "nessun PC salvato con questo ID")
		return false
	case esito == service.UuidDiverso:
		dispositivoDiverso(c, id, "uuid diverso da quello salvato")
		return false
	}
	return true
}

// auditNonSalvato scrive nel log, a livello error, l'audit che il database
// non ha salvato. Al client va successo lo stesso: ignora la risposta, e un
// errore non gli farebbe rimandare niente.
func auditNonSalvato(c *gin.Context, err error) {
	global.Logger.Errorf("%s %s: audit non salvato: %v", c.Request.Method, c.FullPath(), err)
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
