//go:build !windows

package http

import (
	"errors"
	"net"

	"github.com/fvbock/endless"
	"github.com/gin-gonic/gin"
)

// Run serve g su addr finche' il server non si ferma, e restituisce l'errore
// che lo ha fermato. Allo stop normale (SIGINT, SIGTERM, o il processo nuovo
// dopo un SIGHUP) endless chiude il listener e ListenAndServe restituisce
// l'errore di Accept sul listener chiuso: quello non e' un errore.
func Run(g *gin.Engine, addr string) error {
	err := endless.ListenAndServe(addr, g)
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}
