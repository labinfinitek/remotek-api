package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/lejianwen/rustdesk-api/v2/global"
	apiReq "github.com/lejianwen/rustdesk-api/v2/http/request/api"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	apiResp "github.com/lejianwen/rustdesk-api/v2/http/response/api"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

type User struct {
}

// currentUser 当前用户
// @Tags 用户
// @Summary 用户信息
// @Description 用户信息
// @Accept  json
// @Produce  json
// @Success 200 {object} apiResp.UserPayload
// @Failure 500 {object} response.Response
// @Router /currentUser [get]
// @Security token
// func (u *User) currentUser(c *gin.Context) {
//	user := service.AllService.UserService.CurUser(c)
//	up := (&apiResp.UserPayload{}).FromName(user)
//	c.JSON(http.StatusOK, up)
//}

// Info 用户信息
// @Tags 用户
// @Summary 用户信息
// @Description 用户信息
// @Accept  json
// @Produce  json
// @Success 200 {object} apiResp.UserPayload
// @Failure 500 {object} response.Response
// @Router /currentUser [post]
// @Router /user/info [get]
// @Security token
func (u *User) Info(c *gin.Context) {
	user := service.AllService.UserService.CurUser(c)
	up := (&apiResp.UserPayload{}).FromUser(user)
	c.JSON(http.StatusOK, up)
}

// Agente dice al client CLI se l'utente del token e' un
// agente AI e il nome del tecnico che ne risponde: il nickname, o lo
// username se il nickname e' vuoto.
// @Tags 用户
// @Summary agente AI dell'utente
// @Produce  json
// @Success 200 {object} apiResp.AgentePayload
// @Failure 400 {object} response.ErrorResponse
// @Router /agente [get]
// @Security token
func (u *User) Agente(c *gin.Context) {
	user := service.AllService.UserService.CurUser(c)
	res := apiResp.AgentePayload{}
	if user.AgenteDi != 0 {
		t, err := service.AllService.UserService.InfoById(user.AgenteDi)
		if err != nil {
			response.ErrorErr(c, "SystemError", err)
			return
		}
		res.Agente, res.Tecnico = true, t.Nickname
		if res.Tecnico == "" {
			res.Tecnico = t.Username
		}
	}
	c.JSON(http.StatusOK, res)
}

// Lasciapassare rilascia all'agente AI del token il lasciapassare firmato
// per aprire, entro 60 secondi e una volta sola, una sessione sul PC id
// (ADR-0028), da amministratore o, con modo utente, senza, per la capacita'
// chiesta (ADR-0033: terminale se assente). Rifiuta se la funzione e'
// spenta, se l'utente non e' un agente attivo, se il suo tecnico non e'
// attivo, se il PC non e' in peers o non e' nella rubrica del tecnico. Nel
// log mai il lasciapassare ne' il nonce.
// @Tags 用户
// @Summary lasciapassare dell'agente AI
// @Accept  json
// @Produce  json
// @Param body body apiReq.LasciapassareForm true "ID del PC"
// @Success 200 {object} apiResp.LasciapassarePayload
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /agente/lasciapassare [post]
// @Security token
func (u *User) Lasciapassare(c *gin.Context) {
	f := &apiReq.LasciapassareForm{}
	if err := c.ShouldBindBodyWith(f, binding.JSON); err != nil {
		response.ErrorErr(c, "ParamsError", err)
		return
	}
	if err := f.Valida(); err != nil {
		response.ErrorErr(c, "ParamsError", err)
		return
	}
	modo, capacita := f.Modo, f.Capacita
	if modo == "" { // i client -4 e -5 non mandano il campo
		modo = service.ModoAmministratore
	}
	if capacita == "" { // ne' questo
		capacita = service.CapacitaTerminale
	}
	durata := f.Durata
	if capacita == service.CapacitaTunnel && durata == 0 {
		durata = service.DurataTunnel
	}
	agente := service.AllService.UserService.CurUser(c)
	lp, tecnico, err := service.AllService.LasciapassareService.Rilascia(agente, f.Id, modo, capacita, f.Porte, durata, time.Now())
	if err != nil {
		response.ErrorErr(c, "SystemError", err)
		return
	}
	global.Logger.Per(c.Request.Context()).Infof("lasciapassare rilasciato: agente %s, tecnico %s, PC %s, modo %s, capacita' %s, porte %v, durata %d",
		agente.Username, tecnico.Username, f.Id, modo, capacita, f.Porte, durata)
	if err := service.AllService.EventiService.AccodaLasciapassare(f.Id, agente, tecnico, modo == service.ModoAmministratore, capacita, f.Porte, durata); err != nil {
		eventoNonAccodato(c, service.EventoAgenteLasciapassare, err)
	}
	c.JSON(http.StatusOK, apiResp.LasciapassarePayload{Lasciapassare: lp})
}

// LasciapassareTecnico rilascia al tecnico del token il lasciapassare
// firmato per aprire, entro 60 secondi e una volta sola, la sua sessione sul
// PC id (ADR-0032), senza la rubrica: a chi e' entrato con OIDC, o all'admin
// di emergenza (amministratore in app.pwd-login-eccezioni) con un warn.
// Rifiuta gli altri entrati con la password, un PC che non e' in peers e la
// funzione spenta; un agente AI ha la sua rotta. Nessun evento verso
// l'integrazione; nel log mai il lasciapassare ne' il nonce.
// @Tags 用户
// @Summary lasciapassare del tecnico
// @Accept  json
// @Produce  json
// @Param body body apiReq.LasciapassareTecnicoForm true "ID del PC"
// @Success 200 {object} apiResp.LasciapassarePayload
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /lasciapassare [post]
// @Security token
func (u *User) LasciapassareTecnico(c *gin.Context) {
	f := &apiReq.LasciapassareTecnicoForm{}
	if err := c.ShouldBindBodyWith(f, binding.JSON); err != nil {
		response.ErrorErr(c, "ParamsError", err)
		return
	}
	tecnico := service.AllService.UserService.CurUser(c)
	daOidc := service.AllService.UserService.LoginDaOidc(c)
	lp, emergenza, err := service.AllService.LasciapassareService.RilasciaTecnico(tecnico, daOidc, f.Id, time.Now())
	if err != nil {
		response.ErrorErr(c, "SystemError", err)
		return
	}
	if emergenza {
		global.Logger.Per(c.Request.Context()).Warnf("lasciapassare all'admin di emergenza: utente %s, PC %s", tecnico.Username, f.Id)
	} else {
		global.Logger.Per(c.Request.Context()).Infof("lasciapassare rilasciato al tecnico %s, PC %s", tecnico.Username, f.Id)
	}
	c.JSON(http.StatusOK, apiResp.LasciapassarePayload{Lasciapassare: lp})
}
