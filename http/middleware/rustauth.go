package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

func RustAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取HTTP_AUTHORIZATION
		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(401, gin.H{
				"error": "Unauthorized",
			})
			c.Abort()
			return
		}
		if len(token) <= 7 {
			c.JSON(401, gin.H{
				"error": "Unauthorized",
			})
			c.Abort()
			return
		}
		// 提取token，格式是Bearer {token}
		// 这里只是简单的提取
		token = token[7:]

		// 验证token

		// 检查是否设置了jwt key
		if len(global.Jwt.Key) > 0 {
			uid, _ := service.AllService.UserService.VerifyJWT(token)
			if uid == 0 {
				c.JSON(401, gin.H{
					"error": "Unauthorized",
				})
				c.Abort()
				return
			}
		}

		user, ut, err := service.AllService.UserService.InfoByAccessToken(token)
		if errors.Is(err, service.ErrNotFound) {
			c.JSON(401, gin.H{
				"error": "Unauthorized",
			})
			c.Abort()
			return
		}
		if err != nil {
			// Ne' 401 ne' 400, che per il client 1.4.9 sono un logout (su
			// /api/currentUser anche il 400, user_model.dart:81-82): un
			// errore del database non fa uscire il tecnico.
			response.ErrorStatusErr(c, http.StatusInternalServerError, "SystemError", err)
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

		if err := service.AllService.UserService.AutoRefreshAccessToken(ut); err != nil {
			// Il token vale fino alla scadenza di prima: la richiesta va avanti.
			global.Logger.Warnf("%s %s: rinnovo del token non riuscito: %v", c.Request.Method, c.FullPath(), err)
		}

		c.Next()
	}
}
