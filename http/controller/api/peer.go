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
	fpe := f.ToPeer()
	pe := service.AllService.PeerService.FindById(f.Id)
	if pe.RowId == 0 {
		pe = f.ToPeer()
		pe.UserId = ultimoUtente(c, pe.Uuid, pe.Id)
		err = service.AllService.PeerService.Create(pe)
		if err != nil {
			response.ErrorErr(c, "OperationFailed", err)
			return
		}
	} else {
		if pe.UserId == 0 {
			pe.UserId = ultimoUtente(c, pe.Uuid, pe.Id)
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
