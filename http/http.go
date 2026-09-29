package http

import (
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/http/middleware"
	"github.com/lejianwen/rustdesk-api/v2/http/router"
)

// NewEngine costruisce il router completo dell'API senza avviare il server:
// la usano ApiInit e i test del contratto.
func NewEngine() *gin.Engine {
	gin.SetMode(global.Config.Gin.Mode)
	g := gin.New()
	if err := setTrustedProxies(g, global.Config.Gin.TrustProxy); err != nil {
		panic(err)
	}

	g.NoRoute(func(c *gin.Context) {
		c.String(http.StatusNotFound, "404 not found")
	})
	g.Use(middleware.Logger(), middleware.Limiter(), recupera())
	router.WebInit(g)
	router.Init(g)
	router.ApiInit(g)
	return g
}

// setTrustedProxies dice a g di quali proxy fidarsi. ClientIP(), l'IP con cui
// il limiter dei login conta i tentativi per captcha e ban, legge
// X-Forwarded-For e X-Real-IP solo nelle richieste che arrivano da un proxy
// fidato. trustProxy e' gin.trust-proxy, IP o CIDR separati da virgola; vuoto
// (il default) vuol dire nessun proxy, non tutti come in gin.New(), che
// lascerebbe a ogni client scegliere l'IP con cui viene contato o bannato.
func setTrustedProxies(g *gin.Engine, trustProxy string) error {
	if trustProxy == "" {
		return g.SetTrustedProxies(nil)
	}
	return g.SetTrustedProxies(strings.Split(trustProxy, ","))
}

// ApiInit costruisce il router con NewEngine e lo avvia su gin.api-addr;
// restituisce l'errore di Run, nil allo stop normale.
func ApiInit() error {
	return Run(NewEngine(), global.Config.Gin.ApiAddr)
}

// recupera e' gin.Recovery con il panic nel log a error, con metodo, rotta e
// stack, e senza la richiesta: gin ne scriverebbe gli header, cookie compresi.
// Col writer nil gin non costruisce nemmeno il dump. Al client, come
// gin.Recovery, un 500 senza corpo.
func recupera() gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, err any) {
		global.Logger.Errorf("%s %s: panic: %v\n%s", c.Request.Method, c.FullPath(), err, debug.Stack())
		c.AbortWithStatus(http.StatusInternalServerError)
	})
}
