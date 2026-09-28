package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/http/request/api"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	apiResp "github.com/lejianwen/rustdesk-api/v2/http/response/api"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

type Oauth struct {
}

// OidcAuth avvia il login OIDC del client e restituisce il codice del
// login e l'indirizzo del provider.
// @Tags Oauth
// @Summary OidcAuth
// @Description OidcAuth
// @Accept  json
// @Produce  json
// @Success 200 {object} apiResp.LoginRes
// @Failure 500 {object} response.ErrorResponse
// @Router /oidc/auth [post]
func (o *Oauth) OidcAuth(c *gin.Context) {
	f := &api.OidcAuthRequest{}
	err := c.ShouldBindJSON(&f)
	if err != nil {
		response.ErrorErr(c, "ParamsError", err)
		return
	}

	oauthService := service.AllService.OauthService

	state, verifier, nonce, url, err := oauthService.BeginAuth(f.Op)
	if err != nil {
		response.ErrorErr(c, "SystemError", err)
		return
	}

	service.AllService.OauthService.SetOauthCache(state, &service.OauthCacheItem{
		Action:     service.OauthActionTypeLogin,
		Id:         f.Id,
		Op:         f.Op,
		Uuid:       f.Uuid,
		DeviceName: f.DeviceInfo.Name,
		DeviceOs:   f.DeviceInfo.Os,
		DeviceType: f.DeviceInfo.Type,
		Verifier:   verifier,
		Nonce:      nonce,
	}, 5*60)
	// fmt.Println("code url", code, url)
	c.JSON(http.StatusOK, gin.H{
		"code": state,
		"url":  url,
	})
}

func (o *Oauth) OidcAuthQueryPre(c *gin.Context) (*model.User, *model.UserToken) {
	var u *model.User
	var ut *model.UserToken
	q := &api.OidcAuthQuery{}

	// 解析查询参数并处理错误
	if err := c.ShouldBindQuery(q); err != nil {
		response.ErrorErr(c, "ParamsError", err)
		return nil, nil
	}

	// 获取 OAuth 缓存
	v := service.AllService.OauthService.GetOauthCache(q.Code)
	if v == nil {
		response.Error(c, response.TranslateMsg(c, "OauthExpired"))
		return nil, nil
	}

	// 如果 UserId 为 0，说明还在授权中
	if v.UserId == 0 {
		// Risposta che il client nativo 1.4.9 interroga ogni secondo: cerca
		// alla lettera "No authed oidc is found" in error e continua
		// (src/hbbs_http/account.rs:291). Golden oidc-auth-query-in-attesa.
		c.JSON(http.StatusOK, gin.H{"message": "Authorization in progress, please login and bind", "error": "No authed oidc is found"})
		return nil, nil
	}

	// 获取用户信息
	u, err := service.AllService.UserService.InfoById(v.UserId)
	if errors.Is(err, service.ErrNotFound) {
		response.Error(c, response.TranslateMsg(c, "UserNotFound"))
		return nil, nil
	}
	if err != nil {
		response.ErrorErr(c, "SystemError", err)
		return nil, nil
	}

	// 删除 OAuth 缓存
	service.AllService.OauthService.DeleteOauthCache(q.Code)

	// 创建登录日志并生成用户令牌
	ut, err = service.AllService.UserService.Login(u, &model.LoginLog{
		UserId:   u.Id,
		Client:   v.DeviceType,
		DeviceId: v.Id,
		Uuid:     v.Uuid,
		Ip:       c.ClientIP(),
		Type:     model.LoginLogTypeOauth,
		Platform: v.DeviceOs,
	})
	if err != nil {
		response.ErrorErr(c, "LoginFailed", err)
		return nil, nil
	}

	// 返回用户令牌
	return u, ut
}

// OidcAuthQuery restituisce al client il token del login OIDC, quando il
// provider ha risposto.
// @Tags Oauth
// @Summary OidcAuthQuery
// @Description OidcAuthQuery
// @Accept  json
// @Produce  json
// @Success 200 {object} apiResp.LoginRes
// @Failure 500 {object} response.ErrorResponse
// @Router /oidc/auth-query [get]
func (o *Oauth) OidcAuthQuery(c *gin.Context) {
	u, ut := o.OidcAuthQueryPre(c)
	if u == nil || ut == nil {
		return
	}
	c.JSON(http.StatusOK, apiResp.LoginRes{
		AccessToken: ut.Token,
		Type:        "access_token",
		User:        *(&apiResp.UserPayload{}).FromUser(u),
	})
}

// OauthCallback 回调
// @Tags Oauth
// @Summary OauthCallback
// @Description OauthCallback
// @Accept  json
// @Produce  json
// @Success 200 {object} apiResp.LoginRes
// @Failure 500 {object} response.ErrorResponse
// @Router /oidc/callback [get]
func (o *Oauth) OauthCallback(c *gin.Context) {
	state := c.Query("state")
	if state == "" {
		// ParamIsEmpty ha il segnaposto del campo, che /api/oidc/msg non
		// riempie: la pagina mostrava "Il campo <no value> è vuoto.".
		c.HTML(http.StatusOK, "oauth_fail.html", gin.H{
			"message": "OauthStateMissing",
		})
		return
	}
	cacheKey := state
	oauthService := service.AllService.OauthService
	// 从缓存中获取
	oauthCache := oauthService.GetOauthCache(cacheKey)
	if oauthCache == nil {
		c.HTML(http.StatusOK, "oauth_fail.html", gin.H{
			"message": "OauthExpired",
		})
		return
	}
	nonce := oauthCache.Nonce
	op := oauthCache.Op
	action := oauthCache.Action
	verifier := oauthCache.Verifier
	var user *model.User
	// 获取用户信息
	code := c.Query("code")
	oauthUser, err := oauthService.Callback(code, verifier, op, nonce)
	if err != nil {
		// L'errore del provider va solo nel log, la pagina dice OauthFailed.
		global.Logger.Warnf("%s %s: alla pagina va OauthFailed, errore %q", c.Request.Method, c.FullPath(), err)
		c.HTML(http.StatusOK, "oauth_fail.html", gin.H{
			"message": "OauthFailed",
		})
		return
	}
	userId := oauthCache.UserId
	openid := oauthUser.OpenId
	switch action {
	case service.OauthActionTypeBind:
		// fmt.Println("bind", ty, userData)
		// 检查此openid是否已经绑定过
		utr, err := oauthService.UserThirdInfo(op, openid)
		if err != nil && !errors.Is(err, service.ErrNotFound) {
			c.HTML(http.StatusOK, "oauth_fail.html", gin.H{
				"message": response.IDErr(c, "OauthFailed", err),
			})
			return
		}
		if err == nil && utr.UserId > 0 {
			c.HTML(http.StatusOK, "oauth_fail.html", gin.H{
				"message": "OauthHasBindOtherUser",
			})
			return
		}
		// 绑定: ItemNotFound se l'utente non c'e', OauthFailed se non si legge
		if _, err := service.AllService.UserService.InfoById(userId); err != nil {
			c.HTML(http.StatusOK, "oauth_fail.html", gin.H{
				"message": response.IDErr(c, "OauthFailed", err),
			})
			return
		}
		// 绑定
		err = oauthService.BindOauthUser(userId, oauthUser, op)
		if err != nil {
			c.HTML(http.StatusOK, "oauth_fail.html", gin.H{
				"message": "BindFail",
			})
			return
		}
		c.HTML(http.StatusOK, "oauth_success.html", gin.H{
			"message": "BindSuccess",
		})
	case service.OauthActionTypeLogin:
		// 登录
		if userId != 0 {
			c.HTML(http.StatusOK, "oauth_fail.html", gin.H{
				"message": "OauthHasBeenSuccess",
			})
			return
		}
		user, err = service.AllService.UserService.InfoByOauthId(op, openid)
		if err != nil && !errors.Is(err, service.ErrNotFound) {
			c.HTML(http.StatusOK, "oauth_fail.html", gin.H{
				"message": response.IDErr(c, "OauthFailed", err),
			})
			return
		}
		if user == nil {
			oauthConfig, err := oauthService.InfoByOp(op)
			if err != nil {
				c.HTML(http.StatusOK, "oauth_fail.html", gin.H{
					"message": response.IDErr(c, "OauthFailed", err),
				})
				return
			}
			// Un provider salvato senza auto_register ha NULL: vale spenta,
			// come il default di FormatOauthInfo.
			if oauthConfig.AutoRegister == nil || !*oauthConfig.AutoRegister {
				// c.String(http.StatusInternalServerError, "还未绑定用户，请先绑定")
				oauthCache.UpdateFromOauthUser(oauthUser)
				c.Redirect(http.StatusFound, "/_admin/#/oauth/bind/"+cacheKey)
				return
			}

			// 自动注册
			user, err = service.AllService.UserService.RegisterByOauth(oauthUser, op)
			if err != nil {
				c.HTML(http.StatusOK, "oauth_fail.html", gin.H{
					"message": response.IDErr(c, "OauthFailed", err),
				})
				return
			}
		}
		oauthCache.UserId = user.Id
		oauthService.SetOauthCache(cacheKey, oauthCache, 0)
		// 如果是webadmin，登录成功后跳转到webadmin
		if oauthCache.DeviceType == model.LoginLogClientWebAdmin {
			/*service.AllService.UserService.Login(u, &model.LoginLog{
				UserId:   u.Id,
				Client:   "webadmin",
				Uuid:     "", //must be empty
				Ip:       c.ClientIP(),
				Type:     model.LoginLogTypeOauth,
				Platform: oauthService.DeviceOs,
			})*/
			c.Redirect(http.StatusFound, "/_admin/#/")
			return
		}
		c.HTML(http.StatusOK, "oauth_success.html", gin.H{
			"message": "OauthSuccess",
		})
	default:
		c.HTML(http.StatusOK, "oauth_fail.html", gin.H{
			"message": "ParamsError",
		})
	}
}

type MessageParams struct {
	Lang  string `json:"lang" form:"lang"`
	Title string `json:"title" form:"title"`
	Msg   string `json:"msg" form:"msg"`
}

// Message risponde alle pagine OAuth con lo script che assegna a title e msg
// le traduzioni degli ID nella query. I valori sono letterali JSON, che sono
// stringhe JavaScript valide qualunque cosa contengano: tra apici, un
// apostrofo della traduzione ("L'elemento esiste già.") chiudeva la
// stringa, lo script non partiva e la pagina mostrava l'ID.
func (o *Oauth) Message(c *gin.Context) {
	mp := &MessageParams{}
	if err := c.ShouldBindQuery(mp); err != nil {
		return
	}
	localizer := global.Localizer(mp.Lang)
	res := ""
	for _, v := range []struct{ nome, id string }{{"title", mp.Title}, {"msg", mp.Msg}} {
		if v.id == "" {
			continue
		}
		testo, err := localizer.LocalizeMessage(&i18n.Message{ID: v.id})
		if err != nil {
			continue
		}
		letterale, err := json.Marshal(testo)
		if err != nil {
			continue
		}
		res += ";" + v.nome + " = " + string(letterale) + ";"
	}

	// lo script lo carica la pagina con un tag <script>
	c.Header("Content-Type", "application/javascript")
	c.String(http.StatusOK, res)
}
