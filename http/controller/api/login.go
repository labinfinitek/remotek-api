package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/http/request/api"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	apiResp "github.com/lejianwen/rustdesk-api/v2/http/response/api"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

type Login struct {
}

// Login 登录
// @Tags 登录
// @Summary 登录
// @Description 登录
// @Accept  json
// @Produce  json
// @Param body body api.LoginForm true "登录表单"
// @Success 200 {object} apiResp.LoginRes
// @Failure 500 {object} response.ErrorResponse
// @Router /login [post]
func (l *Login) Login(c *gin.Context) {
	// 检查登录限制
	loginLimiter := global.LoginLimiter
	clientIp := c.ClientIP()

	f := &api.LoginForm{}
	err := c.ShouldBindJSON(f)
	if err != nil {
		loginLimiter.RecordFailedAttempt(clientIp)
		global.Logger.Warn(fmt.Sprintf("Login Fail: %s %s %s", "ParamsError", c.RemoteIP(), c.ClientIP()))
		response.ErrorErr(c, "ParamsError", err)
		return
	}

	errList := global.Validator.ValidStruct(c, f)
	if len(errList) > 0 {
		loginLimiter.RecordFailedAttempt(clientIp)
		global.Logger.Warn(fmt.Sprintf("Login Fail: %s %s %s", "ParamsError", c.RemoteIP(), c.ClientIP()))
		response.Error(c, errList[0])
		return
	}

	// Con il login con password spento entrano solo le eccezioni; agli
	// altri, e a un'eccezione con la password sbagliata, la stessa risposta
	// di prima, che non dice chi e' un'eccezione o se esiste.
	errore, motivo := "UsernameOrPasswordError", ""
	if global.Config.App.DisablePwdLogin {
		errore = "PwdLoginDisabled"
		motivo, err = service.AllService.UserService.EccezionePwdLogin(f.Username, f.Password, true)
		if err != nil {
			response.ErrorErr(c, "SystemError", err)
			return
		}
	}
	var u *model.User
	if !global.Config.App.DisablePwdLogin || motivo != "" {
		u, err = service.AllService.UserService.InfoByUsernamePassword(f.Username, f.Password)
	} else {
		err = service.ErrNotFound
	}

	if errors.Is(err, service.ErrNotFound) {
		loginLimiter.RecordFailedAttempt(clientIp)
		global.Logger.Warn(fmt.Sprintf("Login Fail: %s %s %s", errore, c.RemoteIP(), c.ClientIP()))
		response.Error(c, response.TranslateMsg(c, errore))
		return
	}
	if err != nil {
		// Non e' una password sbagliata: non conta per captcha e ban, che
		// bloccherebbero l'IP di tecnici con la password giusta.
		response.ErrorErr(c, "SystemError", err)
		return
	}

	if !service.AllService.UserService.CheckUserEnable(u) {
		response.Error(c, response.TranslateMsg(c, "UserDisabled"))
		return
	}

	ut, err := service.AllService.UserService.Login(u, &model.LoginLog{
		UserId:   u.Id,
		Client:   f.DeviceInfo.Type,
		DeviceId: f.Id,
		Uuid:     f.Uuid,
		Ip:       c.ClientIP(),
		Type:     model.LoginLogTypeAccount,
		Platform: f.DeviceInfo.Os,
	})
	if err != nil {
		response.ErrorErr(c, "OperationFailed", err)
		return
	}
	TracciaEccezione(c, u, motivo)

	c.JSON(http.StatusOK, apiResp.LoginRes{
		AccessToken: ut.Token,
		Type:        "access_token",
		User:        *(&apiResp.UserPayload{}).FromUser(u),
	})
}

// TracciaEccezione scrive nel log il login con password riuscito di u per
// l'eccezione motivo (service.EccezionePwdLogin); niente se motivo e' vuoto.
// Per un amministratore e' un warn: e' l'accesso di emergenza al pannello.
func TracciaEccezione(c *gin.Context, u *model.User, motivo string) {
	if motivo == "" {
		return
	}
	l := global.Logger.Per(c.Request.Context())
	if service.AllService.UserService.IsAdmin(u) {
		l.Warnf("login con password per eccezione: utente %s (amministratore), motivo %s", u.Username, motivo)
		return
	}
	l.Infof("login con password per eccezione: utente %s, motivo %s", u.Username, motivo)
}

// LoginOptions restituisce i modi di login che il client offre: i provider
// OIDC e, con app.web-sso, webauth.
// @Tags 登录
// @Summary 登录选项
// @Description 登录选项
// @Accept  json
// @Produce  json
// @Success 200 {object} []string
// @Failure 500 {object} response.ErrorResponse
// @Router /login-options [get]
func (l *Login) LoginOptions(c *gin.Context) {
	ops, err := service.AllService.OauthService.GetOauthProviders()
	if err != nil {
		response.ErrorErr(c, "SystemError", err)
		return
	}
	if global.Config.App.WebSso {
		ops = append(ops, model.OauthTypeWebauth)
	}
	// Vuoto e non nil: senza opzioni "common-oidc/[]", non "common-oidc/null".
	oidcItems := make([]map[string]string, 0, len(ops))
	for _, v := range ops {
		oidcItems = append(oidcItems, map[string]string{"name": v})
	}
	common, err := json.Marshal(oidcItems)
	if err != nil {
		response.ErrorErr(c, "SystemError", err)
		return
	}
	var res []string
	res = append(res, "common-oidc/"+string(common))
	for _, v := range ops {
		res = append(res, "oidc/"+v)
	}
	c.JSON(http.StatusOK, res)
}

// Logout cancella il token di sessione del client e scollega il suo
// dispositivo dall'utente.
// @Tags 登录
// @Summary 登出
// @Description 登出
// @Accept  json
// @Produce  json
// @Success 200 {string} string
// @Failure 500 {object} response.ErrorResponse
// @Router /logout [post]
func (l *Login) Logout(c *gin.Context) {
	u := service.AllService.UserService.CurUser(c)
	token, ok := c.Get("token")
	if ok {
		if err := service.AllService.UserService.Logout(u, token.(string)); err != nil {
			// Il client 1.4.9 non legge la risposta ed esce comunque
			// (user_model.dart, logOut): l'errore dice il vero, il token vale ancora.
			response.ErrorErr(c, "OperationFailed", err)
			return
		}
	}
	c.JSON(http.StatusOK, nil)

}
