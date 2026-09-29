package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/global"
	requestform "github.com/lejianwen/rustdesk-api/v2/http/request/api"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

type Index struct {
}

// Index 首页
// @Tags 首页
// @Summary 首页
// @Description 首页
// @Accept  json
// @Produce  json
// @Success 200 {object} response.Response
// @Failure 500 {object} response.Response
// @Router / [get]
func (i *Index) Index(c *gin.Context) {
	response.Success(
		c,
		"Hello Gwen",
	)
}

// Heartbeat 心跳
// @Tags 首页
// @Summary 心跳
// @Description 心跳
// @Accept  json
// @Produce  json
// @Success 200 {object} nil
// @Failure 500 {object} response.Response
// @Router /heartbeat [post]
func (i *Index) Heartbeat(c *gin.Context) {
	info := &requestform.PeerInfoInHeartbeat{}
	err := c.ShouldBindJSON(info)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	if info.Uuid == "" {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	peer, esito, err := service.AllService.PeerService.Riconosci(info.Id, info.Uuid)
	if err != nil {
		// Il client non legge l'errore: l'ultimo contatto resta quello di prima.
		global.Logger.Per(c.Request.Context()).Warnf("%s %s: ultimo contatto del dispositivo non aggiornato: %v", c.Request.Method, c.FullPath(), err)
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	switch esito {
	case service.PcSconosciuto:
		c.JSON(http.StatusOK, gin.H{})
		return
	case service.UuidDiverso:
		// Ultimo contatto e IP sono del dispositivo salvato, non di chi
		// ne conosce l'ID. Un PC creato dal pannello senza uuid si lega al
		// primo sysinfo (ADR-0019), non all'heartbeat.
		perche := "uuid diverso da quello salvato"
		if peer.Uuid == "" {
			perche = "PC senza uuid, in attesa del primo sysinfo"
		}
		dispositivoDiverso(c, info.Id, perche)
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	// 如果在40s以内则不更新
	if time.Now().Unix()-peer.LastOnlineTime >= 30 {
		upp := &model.Peer{RowId: peer.RowId, LastOnlineTime: time.Now().Unix(), LastOnlineIp: c.ClientIP()}
		if err := service.AllService.PeerService.Update(upp); err != nil {
			// Il client non legge l'errore: l'ultimo contatto resta quello di prima.
			global.Logger.Per(c.Request.Context()).Warnf("%s %s: ultimo contatto del dispositivo non salvato: %v", c.Request.Method, c.FullPath(), err)
		}
	}
	c.JSON(http.StatusOK, gin.H{})
}

// Version 版本
// @Tags 首页
// @Summary 版本
// @Description 版本
// @Accept  json
// @Produce  json
// @Success 200 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /version [get]
func (i *Index) Version(c *gin.Context) {
	// 读取resources/version文件
	v := service.AllService.AppService.GetAppVersion()
	response.Success(
		c,
		v,
	)
}
