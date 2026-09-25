package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/http/controller/web"
	"github.com/lejianwen/rustdesk-api/v2/http/middleware"
)

func WebInit(g *gin.Engine) {
	i := &web.Index{}
	g.GET("/", i.Index)
	g.StaticFS("/_admin", http.Dir(global.Config.Gin.ResourcesPath+"/admin"))
	// Logo e favicon del marchio: brand.dir, senza elenco della cartella, con
	// le intestazioni che impediscono a un SVG aperto di eseguire script.
	g.Group("/brand", middleware.Marchio()).Static("", global.Config.Brand.Dir)
}
