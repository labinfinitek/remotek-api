package admin

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/http/controller/api"
	"github.com/lejianwen/rustdesk-api/v2/http/request/admin"
	apiReq "github.com/lejianwen/rustdesk-api/v2/http/request/api"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	adResp "github.com/lejianwen/rustdesk-api/v2/http/response/admin"
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
// @Param body body admin.Login true "登录信息"
// @Success 200 {object} response.Response{data=adResp.LoginPayload}
// @Failure 500 {object} response.Response
// @Router /admin/login [post]
// @Security token
func (ct *Login) Login(c *gin.Context) {
	// 检查登录限制
	loginLimiter := global.LoginLimiter
	clientIp := c.ClientIP()
	_, needCaptcha := loginLimiter.CheckSecurityStatus(clientIp)

	f := &admin.Login{}
	err := c.ShouldBindJSON(f)
	if err != nil {
		loginLimiter.RecordFailedAttempt(clientIp)
		global.Logger.Warn(fmt.Sprintf("Login Fail: %s %s %s", "ParamsError", c.RemoteIP(), clientIp))
		response.FailErr(c, 101, "ParamsError", err)
		return
	}

	errList := global.Validator.ValidStruct(c, f)
	if len(errList) > 0 {
		loginLimiter.RecordFailedAttempt(clientIp)
		global.Logger.Warn(fmt.Sprintf("Login Fail: %s %s %s", "ParamsError", c.RemoteIP(), clientIp))
		response.Fail(c, 101, errList[0])
		return
	}

	// 检查是否需要验证码
	if needCaptcha {
		if f.CaptchaId == "" || f.Captcha == "" || !loginLimiter.VerifyCaptcha(f.CaptchaId, f.Captcha) {
			response.Fail(c, 101, response.TranslateMsg(c, "CaptchaError"))
			return
		}
	}

	// Con il login con password spento entrano solo i nomi dell'elenco,
	// non gli agenti; gli altri come in /api/login.
	errore, motivo := "UsernameOrPasswordError", ""
	if global.Config.App.DisablePwdLogin {
		errore = "PwdLoginDisabled"
		motivo, err = service.AllService.UserService.EccezionePwdLogin(f.Username, f.Password, false)
		if err != nil {
			response.FailErr(c, 101, "SystemError", err)
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
		global.Logger.Warn(fmt.Sprintf("Login Fail: %s %s %s", errore, c.RemoteIP(), clientIp))
		loginLimiter.RecordFailedAttempt(clientIp)
		if _, needCaptcha = loginLimiter.CheckSecurityStatus(clientIp); needCaptcha {
			response.Fail(c, 110, response.TranslateMsg(c, errore))
		} else {
			response.Fail(c, 101, response.TranslateMsg(c, errore))
		}
		return
	}
	if err != nil {
		// Non e' una password sbagliata: non conta per captcha e ban.
		response.FailErr(c, 101, "SystemError", err)
		return
	}

	if !service.AllService.UserService.CheckUserEnable(u) {
		if needCaptcha {
			response.Fail(c, 110, response.TranslateMsg(c, "UserDisabled"))
			return
		}
		response.Fail(c, 101, response.TranslateMsg(c, "UserDisabled"))
		return
	}
	// Un agente AI usa solo il client CLI (ADR-0028): niente
	// token del pannello. Non conta come password sbagliata.
	if u.AgenteDi != 0 {
		global.Logger.Per(c.Request.Context()).Warnf("login al pannello rifiutato all'agente AI %d", u.Id)
		response.Fail(c, 101, response.TranslateMsg(c, "AgentCliOnly"))
		return
	}

	ut, err := service.AllService.UserService.Login(u, &model.LoginLog{
		UserId:   u.Id,
		Client:   model.LoginLogClientWebAdmin,
		Uuid:     "", // must be empty
		Ip:       clientIp,
		Type:     model.LoginLogTypeAccount,
		Platform: f.Platform,
	})
	if err != nil {
		response.FailErr(c, 101, "OperationFailed", err)
		return
	}
	api.TracciaEccezione(c, u, motivo)

	// 登录成功，清除登录限制
	loginLimiter.RemoveAttempts(clientIp)
	responseLoginSuccess(c, u, ut.Token)
}
func (ct *Login) Captcha(c *gin.Context) {
	loginLimiter := global.LoginLimiter
	clientIp := c.ClientIP()
	banned, needCaptcha := loginLimiter.CheckSecurityStatus(clientIp)
	if banned {
		response.Fail(c, 101, response.TranslateMsg(c, "LoginBanned"))
		return
	}
	if !needCaptcha {
		response.Fail(c, 101, response.TranslateMsg(c, "NoCaptchaRequired"))
		return
	}
	captcha, err := loginLimiter.RequireCaptcha()
	if err != nil {
		response.FailErr(c, 101, "CaptchaError", err)
		return
	}
	b64, err := loginLimiter.DrawCaptcha(captcha.Content)
	if err != nil {
		response.FailErr(c, 101, "CaptchaError", err)
		return
	}
	response.Success(c, gin.H{
		"captcha": gin.H{
			"id":  captcha.Id,
			"b64": b64,
		},
	})
}

// Logout 登出
// @Tags 登录
// @Summary 登出
// @Description 登出
// @Accept  json
// @Produce  json
// @Success 200 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /admin/logout [post]
func (ct *Login) Logout(c *gin.Context) {
	u := service.AllService.UserService.CurUser(c)
	token, ok := c.Get("token")
	if ok {
		if err := service.AllService.UserService.Logout(u, token.(string)); err != nil {
			response.FailErr(c, 101, "OperationFailed", err)
			return
		}
	}
	response.Success(c, nil)
}

// LoginOptions risponde con le opzioni della pagina di login: provider
// OAuth, registrazione, captcha, login con password e OIDC come unico accesso.
// @Tags 登录
// @Summary 登录选项
// @Description 登录选项
// @Accept  json
// @Produce  json
// @Success 200 {object} []string
// @Failure 500 {object} response.ErrorResponse
// @Router /admin/login-options [get]
func (ct *Login) LoginOptions(c *gin.Context) {
	loginLimiter := global.LoginLimiter
	clientIp := c.ClientIP()
	banned, needCaptcha := loginLimiter.CheckSecurityStatus(clientIp)
	if banned {
		response.Fail(c, 101, response.TranslateMsg(c, "LoginBanned"))
		return
	}
	ops, err := service.AllService.OauthService.GetOauthProviders()
	if err != nil {
		response.FailErr(c, 101, "SystemError", err)
		return
	}
	response.Success(c, gin.H{
		"ops":          ops,
		"register":     global.Config.App.Register,
		"need_captcha": needCaptcha,
		"disable_pwd":  global.Config.App.DisablePwdLogin,
		"auto_oidc":    global.Config.App.DisablePwdLogin && len(ops) == 1,
	})
}

// OidcAuth avvia il login OIDC del pannello e risponde col codice da
// interrogare e l'indirizzo del provider.
// @Tags Oauth
// @Summary OidcAuth
// @Description OidcAuth
// @Accept  json
// @Produce  json
// @Router /admin/oidc/auth [post]
func (ct *Login) OidcAuth(c *gin.Context) {
	// o := &api.Oauth{}
	// o.OidcAuth(c)
	f := &apiReq.OidcAuthRequest{}
	err := c.ShouldBindJSON(f)
	if err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}

	state, verifier, nonce, url, err := service.AllService.OauthService.BeginAuth(f.Op)
	if err != nil {
		response.ErrorErr(c, "SystemError", err)
		return
	}

	service.AllService.OauthService.SetOauthCache(state, &service.OauthCacheItem{
		Action:     service.OauthActionTypeLogin,
		Op:         f.Op,
		Id:         f.Id,
		DeviceType: "webadmin",
		// DeviceOs: ct.Platform(c),
		DeviceOs: f.DeviceInfo.Os,
		Uuid:     f.Uuid,
		Verifier: verifier,
		Nonce:    nonce,
	}, 5*60)

	response.Success(c, gin.H{
		"code": state,
		"url":  url,
	})
}

// OidcAuthQuery chiude il login OIDC del pannello: a login riuscito risponde
// come Login.
// @Tags Oauth
// @Summary OidcAuthQuery
// @Description OidcAuthQuery
// @Accept  json
// @Produce  json
// @Success 200 {object} response.Response{data=adResp.LoginPayload}
// @Failure 500 {object} response.Response
// @Router /admin/oidc/auth-query [get]
func (ct *Login) OidcAuthQuery(c *gin.Context) {
	o := &api.Oauth{}
	u, ut := o.OidcAuthQueryPre(c)
	if ut == nil {
		return
	}
	// Come nel login con password: un agente AI non riceve il token del
	// pannello. OidcAuthQueryPre l'ha gia' creato, e varrebbe sulle rotte
	// del CLI fino alla scadenza: si revoca prima di rispondere.
	if u.AgenteDi != 0 {
		if err := service.AllService.UserService.Logout(u, ut.Token); err != nil {
			response.FailErr(c, 101, "SystemError", err)
			return
		}
		global.Logger.Per(c.Request.Context()).Warnf("login OIDC al pannello rifiutato all'agente AI %d", u.Id)
		response.Fail(c, 101, response.TranslateMsg(c, "AgentCliOnly"))
		return
	}
	responseLoginSuccess(c, u, ut.Token)
}

// responseLoginSuccess risponde al pannello con l'utente u e il suo token. Il
// nome del gruppo serve solo al Profilo: se non si legge, il login vale lo
// stesso, con il nome vuoto e l'errore nel log.
func responseLoginSuccess(c *gin.Context, u *model.User, token string) {
	lp := &adResp.LoginPayload{}
	lp.FromUser(u)
	lp.Token = token
	lp.RouteNames = service.AllService.UserService.RouteNames(u)
	switch g, err := service.AllService.GroupService.InfoById(u.GroupId); {
	case err == nil:
		lp.GroupName = g.Name
	case !errors.Is(err, service.ErrNotFound):
		global.Logger.Per(c.Request.Context()).Warnf("nome del gruppo dell'utente %d non letto: %v", u.Id, err)
	}
	response.Success(c, lp)
}
