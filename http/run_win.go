//go:build windows

package http

import (
	"github.com/gin-gonic/gin"
)

// Run serve g su addr e restituisce l'errore che ferma il server. Su Windows
// non c'e' lo stop controllato di endless: ogni ritorno e' un errore.
func Run(g *gin.Engine, addr string) error {
	return g.Run(addr)
}
