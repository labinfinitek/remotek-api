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
	pe, esito, err := service.AllService.PeerService.Riconosci(f.Id, f.Uuid)
	switch {
	case err != nil:
		// Un dispositivo che non si legge non e' nuovo: crearlo farebbe un
		// doppione, perche' peers.id non e' unico. Il client riprova piu' tardi.
		response.ErrorErr(c, "SystemError", err)
		return
	case esito == service.PcSconosciuto:
		// Il primo sysinfo di un ID crea il PC e lo lega al suo uuid.
		pe = f.ToPeer()
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
	}
	// SYSINFO_UPDATED 上传成功
	// ID_NOT_FOUND 下次心跳会上传
	// 直接响应文本
	c.String(http.StatusOK, "SYSINFO_UPDATED")
}

// ultimoUtente restituisce l'utente dell'ultimo login del dispositivo, 0 se
// non ce n'e'. Un errore del database va nel log e vale 0: il dispositivo si
// salva senza utente, come prima, e la risposta non cambia.
func ultimoUtente(c *gin.Context, uuid, id string) uint {
	userId, err := service.AllService.UserService.FindLatestUserIdFromLoginLogByUuid(uuid, id)
	if err != nil && !errors.Is(err, service.ErrNotFound) {
		global.Logger.Warnf("%s %s: dispositivo salvato senza utente: %v", c.Request.Method, c.FullPath(), err)
	}
	return userId
}

// dispositivoDiverso scrive nel log, a livello warn, la richiesta scartata
// perche' l'ID id non e' un PC salvato con l'uuid arrivato. Nel log vanno
// rotta e ID, non gli uuid.
func dispositivoDiverso(c *gin.Context, id, perche string) {
	global.Logger.Warnf("%s %s: dispositivo %s: %s, niente salvato", c.Request.Method, c.FullPath(), id, perche)
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
