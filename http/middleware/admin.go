package middleware

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// BackendUserAuth 后台权限验证中间件
func BackendUserAuth() gin.HandlerFunc {
	return func(c *gin.Context) {

		// 测试先关闭
		token := c.GetHeader("api-token")
		if token == "" {
			response.Fail(c, 403, response.TranslateMsg(c, "NeedLogin"))
			c.Abort()
			return
		}
		user, ut, err := service.AllService.UserService.InfoByAccessToken(token)
		if errors.Is(err, service.ErrNotFound) {
			response.Fail(c, 403, response.TranslateMsg(c, "NeedLogin"))
			c.Abort()
			return
		}
		if err != nil {
			// Non 403, che per il pannello e' un logout (request.js).
			response.FailErr(c, 101, "SystemError", err)
			c.Abort()
			return
		}

		if !service.AllService.UserService.CheckUserEnable(user) {
			c.JSON(401, gin.H{
				"error": "Unauthorized",
			})
			c.Abort()
			return
		}

		c.Set("curUser", user)
		c.Set("token", token)
		// 如果时间小于1天,token自动续期
		if err := service.AllService.UserService.AutoRefreshAccessToken(ut); err != nil {
			// Il token vale fino alla scadenza di prima: la richiesta va avanti.
			global.Logger.Per(c.Request.Context()).Warnf("%s %s: rinnovo del token non riuscito: %v", c.Request.Method, c.FullPath(), err)
		}

		c.Next()
	}
}
