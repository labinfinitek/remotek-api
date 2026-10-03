package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/lejianwen/rustdesk-api/v2/global"
	requestform "github.com/lejianwen/rustdesk-api/v2/http/request/api"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

type Peer struct {
}

// SysInfo salva le informazioni di sistema che il dispositivo manda.
// @Tags System
// @Summary 提交系统信息
// @Description 提交系统信息
// @Accept  json
// @Produce  json
// @Param body body requestform.PeerForm true "系统信息表单"
// @Success 200 {string} string "SYSINFO_UPDATED,ID_NOT_FOUND"
// @Failure 500 {object} response.ErrorResponse
// @Router /sysinfo [post]
func (p *Peer) SysInfo(c *gin.Context) {
	f := &requestform.PeerForm{}
	err := c.ShouldBindBodyWith(f, binding.JSON)
	if err != nil {
		response.ErrorErr(c, "ParamsError", err)
		return
	}
	if f.Uuid == "" {
		// Senza uuid il dispositivo non si riconosce: niente da creare o
		// aggiornare. Il client 1.4.9 lo manda sempre.
		response.ErrorErr(c, "ParamsError", fmt.Errorf("dispositivo %s: sysinfo senza uuid", f.Id))
		return
	}
	fpe := f.ToPeer()
	ric, err := service.AllService.PeerService.RiconosciFirmato(f.Id, f.Uuid, richiestaFirmata(c, f.Pk))
	pe, esito := ric.Peer, ric.Esito
	switch {
	case err != nil:
		// Un dispositivo che non si legge non e' nuovo: crearlo farebbe un
		// doppione su un database senza indice unico su peers.id. Il client
		// riprova piu' tardi.
		response.ErrorErr(c, "SystemError", err)
		return
	case ric.ChiaveDiversa:
		// Error e non warn: PC reinstallato, o qualcuno che conosceva
		// l'uuid ha messo la sua chiave prima del PC vero (README).
		global.Logger.Per(c.Request.Context()).Errorf("%s %s: dispositivo %s: uuid giusto ma firma con una chiave diversa da quella registrata, niente salvato", c.Request.Method, c.FullPath(), f.Id)
		response.Error(c, response.TranslateMsg(c, "DeviceMismatch"))
		return
	case ric.Rifiuto != "":
		response.ErrorErr(c, "DeviceMismatch", fmt.Errorf("dispositivo %s: %s: %w", f.Id, ric.Rifiuto, service.ErrDispositivoDiverso))
		return
	case esito == service.PcSconosciuto:
		// Il primo sysinfo di un ID crea il PC e lo lega al suo uuid, e alla
		// sua chiave se firmato. Di due sysinfo insieme di un ID nuovo,
		// quello che crea per secondo trova l'indice unico e risponde
		// OperationFailed: il client riprova dopo 120 s e trova il PC.
		pe = f.ToPeer()
		pe.ChiavePubblica = ric.Chiave
		pe.UserId = ultimoUtente(c, pe.Uuid, pe.Id)
		err = service.AllService.PeerService.Create(pe)
		if err != nil {
			response.ErrorErr(c, "OperationFailed", err)
			return
		}
	case esito == service.UuidDiverso && pe.Uuid != "":
		// Un altro dispositivo con lo stesso ID: la scheda non cambia. Il
		// warn di ErrorErr ha rotta e ID del PC, non gli uuid. Il legame si
		// riapre cancellando il PC dal pannello.
		response.ErrorErr(c, "DeviceMismatch", fmt.Errorf("dispositivo %s: %w", f.Id, service.ErrDispositivoDiverso))
		return
	default:
		// Lo stesso dispositivo, o un PC creato dal pannello senza uuid,
		// che si lega al primo uuid che arriva: fpe lo porta con se'.
		if pe.UserId == 0 {
			pe.UserId = ultimoUtente(c, fpe.Uuid, pe.Id)
		}
		fpe.RowId = pe.RowId
		fpe.UserId = pe.UserId
		err = service.AllService.PeerService.Update(fpe)
		if err != nil {
			response.ErrorErr(c, "OperationFailed", err)
			return
		}
		if ric.Chiave != "" {
			registraChiave(c, pe.RowId, f.Id, ric.Chiave)
		}
	}
	if ric.NonRegistrata != "" {
		dispositivoDiverso(c, f.Id, "chiave non registrata: "+ric.NonRegistrata)
	}
	// SYSINFO_UPDATED 上传成功
	// ID_NOT_FOUND 下次心跳会上传
	// 直接响应文本
	c.String(http.StatusOK, "SYSINFO_UPDATED")
}

// registraChiave salva la chiave del PC rowId, se non ne ha gia' una. Il
// sysinfo e' salvato lo stesso: una chiave che non si salva va nel log e il
// prossimo sysinfo firmato riprova.
func registraChiave(c *gin.Context, rowId uint, id, chiave string) {
	registrata, err := service.AllService.PeerService.RegistraChiave(rowId, chiave)
	switch {
	case err != nil:
		global.Logger.Per(c.Request.Context()).Errorf("%s %s: chiave del dispositivo non salvata: %v", c.Request.Method, c.FullPath(), err)
	case !registrata:
		global.Logger.Per(c.Request.Context()).Errorf("%s %s: dispositivo %s: chiave non registrata, nel frattempo ne e' stata registrata un'altra", c.Request.Method, c.FullPath(), id)
	}
}

// richiestaFirmata raccoglie la firma del dispositivo di c per
// RiconosciFirmato. Il corpo e' quello letto, una volta sola ed entro
// middleware.CorpoMax, dal bind con ShouldBindBodyWith, che lo tiene nel
// contesto: la firma si verifica sui byte arrivati.
func richiestaFirmata(c *gin.Context, pk string) service.RichiestaFirmata {
	corpo, _ := c.Get(gin.BodyBytesKey)
	b, _ := corpo.([]byte)
	return service.RichiestaFirmata{Metodo: c.Request.Method, Percorso: c.Request.URL.Path,
		Intestazione: c.GetHeader(service.IntestazioneFirma), Corpo: b, Pk: pk}
}

// ultimoUtente restituisce l'utente dell'ultimo login del dispositivo, 0 se
// non ce n'e'. Un errore del database va nel log e vale 0: il dispositivo si
// salva senza utente, come prima, e la risposta non cambia.
func ultimoUtente(c *gin.Context, uuid, id string) uint {
	userId, err := service.AllService.UserService.FindLatestUserIdFromLoginLogByUuid(uuid, id)
	if err != nil && !errors.Is(err, service.ErrNotFound) {
		global.Logger.Per(c.Request.Context()).Warnf("%s %s: dispositivo salvato senza utente: %v", c.Request.Method, c.FullPath(), err)
	}
	return userId
}

// dispositivoDiverso scrive nel log, a livello warn, la richiesta del
// dispositivo id scartata per il motivo perche': l'ID non e' un PC salvato
// con l'uuid arrivato, o un blocco della trascrizione del terminale non
// passa i controlli. Nel log vanno rotta e ID, non gli uuid ne' contenuti.
func dispositivoDiverso(c *gin.Context, id, perche string) {
	global.Logger.Per(c.Request.Context()).Warnf("%s %s: dispositivo %s: %s, niente salvato", c.Request.Method, c.FullPath(), id, perche)
}

// SysInfoVer restituisce la versione dell'API e l'ora di avvio.
// @Tags System
// @Summary 获取系统版本信息
// @Description 获取系统版本信息
// @Accept  json
// @Produce  json
// @Success 200 {string} string ""
// @Failure 500 {object} response.ErrorResponse
// @Router /sysinfo_ver [post]
func (p *Peer) SysInfoVer(c *gin.Context) {
	// 读取resources/version文件
	v := service.AllService.AppService.GetAppVersion()
	// 加上启动时间，方便client上传信息
	v = fmt.Sprintf("%s\n%s", v, service.AllService.AppService.GetStartTime())
	c.String(http.StatusOK, v)
}
