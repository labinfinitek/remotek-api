package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lejianwen/rustdesk-api/v2/http/response"
	apiResp "github.com/lejianwen/rustdesk-api/v2/http/response/api"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// Integrazione serve le rotte in sola lettura per l'integrazione (n8n),
// dietro IntegrazioneAuth (ADR-0026).
type Integrazione struct {
}

// perPaginaMax e' il numero massimo di righe di una pagina.
const perPaginaMax = 1000

// pagina legge pagina (da 1) e per_pagina (default 100, al massimo
// perPaginaMax) dalla query.
func pagina(c *gin.Context) (int, int, bool) {
	p, err1 := strconv.Atoi(c.DefaultQuery("pagina", "1"))
	n, err2 := strconv.Atoi(c.DefaultQuery("per_pagina", "100"))
	if err1 != nil || err2 != nil || p < 1 || n < 1 || n > perPaginaMax {
		return 0, 0, false
	}
	return p, n, true
}

// Pc restituisce una pagina dei PC: ID, nome (hostname), alias, ultimo
// contatto, versione e segno della copia (ADR-0024); mai uuid, chiavi o IP.
// @Tags Integrazione
// @Summary PC, per l'integrazione
// @Produce  json
// @Param pagina query int false "pagina, da 1"
// @Param per_pagina query int false "righe per pagina, da 1 a 1000 (100)"
// @Success 200 {object} apiResp.ElencoPcIntegrazione
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /integrazione/pc [get]
// @Security BearerAuth
func (i *Integrazione) Pc(c *gin.Context) {
	p, n, ok := pagina(c)
	if !ok {
		response.ErrorErr(c, "ParamsError", errors.New("pagina o per_pagina non valide"))
		return
	}
	lista, err := service.AllService.PeerService.List(uint(p), uint(n), func(tx *gorm.DB) { tx.Order("row_id") })
	if err != nil {
		response.ErrorStatusErr(c, http.StatusInternalServerError, "SystemError", err)
		return
	}
	res := apiResp.ElencoPcIntegrazione{Elenco: []apiResp.PcIntegrazione{}, Totale: lista.Total, Pagina: p, PerPagina: n}
	adesso := time.Now()
	for _, pc := range lista.Peers {
		r := apiResp.PcIntegrazione{Id: pc.Id, Nome: pc.Hostname, Alias: pc.Alias, Versione: pc.Version,
			PossibileCopia: service.PossibileCopia(pc.Id, adesso)}
		if pc.LastOnlineTime > 0 {
			u := time.Unix(pc.LastOnlineTime, 0).UTC().Format(time.RFC3339)
			r.UltimoContatto = &u
		}
		res.Elenco = append(res.Elenco, r)
	}
	c.JSON(http.StatusOK, res)
}

// Connessioni restituisce una pagina del registro delle connessioni, in
// ordine di id, del PC pc se c'e' e da da (RFC 3339) se c'e'; mai IP, uuid,
// guid o session_id.
// @Tags Integrazione
// @Summary registro delle connessioni, per l'integrazione
// @Produce  json
// @Param pc query string false "ID del PC"
// @Param da query string false "connessioni iniziate da quest'ora, RFC 3339"
// @Param pagina query int false "pagina, da 1"
// @Param per_pagina query int false "righe per pagina, da 1 a 1000 (100)"
// @Success 200 {object} apiResp.ElencoConnessioniIntegrazione
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /integrazione/connessioni [get]
// @Security BearerAuth
func (i *Integrazione) Connessioni(c *gin.Context) {
	p, n, ok := pagina(c)
	var da time.Time
	var err error
	if s := c.Query("da"); ok && s != "" {
		da, err = time.Parse(time.RFC3339, s)
		ok = err == nil
	}
	if !ok {
		response.ErrorErr(c, "ParamsError", errors.New("pagina, per_pagina o da non valide"))
		return
	}
	pc := c.Query("pc")
	lista, err := service.AllService.AuditService.AuditConnList(uint(p), uint(n), func(tx *gorm.DB) {
		if pc != "" {
			tx.Where("peer_id = ?", pc)
		}
		if !da.IsZero() {
			tx.Where("created_at >= ?", da)
		}
		tx.Order("id")
	})
	if err != nil {
		response.ErrorStatusErr(c, http.StatusInternalServerError, "SystemError", err)
		return
	}
	res := apiResp.ElencoConnessioniIntegrazione{Elenco: []apiResp.ConnessioneIntegrazione{}, Totale: lista.Total, Pagina: p, PerPagina: n}
	for _, conn := range lista.AuditConns {
		s := service.SessioneDi(conn)
		res.Elenco = append(res.Elenco, apiResp.ConnessioneIntegrazione{Id: conn.Id, Pc: conn.PeerId, ConnId: s.ConnId,
			Tipo: s.Tipo, Tecnico: s.Tecnico, Inizio: s.Inizio, Fine: s.Fine, DurataS: s.DurataS, Nota: s.Nota})
	}
	c.JSON(http.StatusOK, res)
}

// Verifica restituisce l'esito della verifica della catena della
// trascrizione della connessione id, come la verifica del pannello; 404 se
// la connessione non c'e' o non e' un terminale. Niente blocchi ne' dati.
// @Tags Integrazione
// @Summary verifica di una trascrizione, per l'integrazione
// @Produce  json
// @Param id path int true "id della connessione nel registro"
// @Success 200 {object} apiResp.VerificaIntegrazione
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /integrazione/connessioni/{id}/verifica [get]
// @Security BearerAuth
func (i *Integrazione) Verifica(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 0)
	if err != nil {
		response.ErrorStatusErr(c, http.StatusNotFound, "ItemNotFound", err)
		return
	}
	as := service.AllService.AuditService
	conn, err := as.ConnInfoById(uint(id))
	if err == nil && conn.Type != model.AuditConnTerminale {
		err = service.ErrNotFound
	}
	var v *model.AuditTerminalVerifica
	if err == nil {
		v, err = as.VerificaTerminale(conn.Id)
	}
	switch {
	case errors.Is(err, service.ErrNotFound):
		response.ErrorStatusErr(c, http.StatusNotFound, "ItemNotFound", err)
	case err != nil:
		response.ErrorStatusErr(c, http.StatusInternalServerError, "SystemError", err)
	default:
		c.JSON(http.StatusOK, apiResp.VerificaIntegrazione{Integra: v.Integra, Blocchi: v.Blocchi, Fine: v.Fine, Hash: v.Hash, PrimoErrato: v.PrimoErrato})
	}
}
