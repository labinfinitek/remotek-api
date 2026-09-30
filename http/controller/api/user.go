package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

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
