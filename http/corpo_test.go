package http

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/http/middleware"
	"github.com/lejianwen/rustdesk-api/v2/lib/logger"
	"github.com/lejianwen/rustdesk-api/v2/model"
)

// contatore conta i byte letti dal corpo di una richiesta.
type contatore struct {
	r io.Reader
	n int64
}

func (c *contatore) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// grosso manda a rotta un corpo JSON di piu' di middleware.CorpoMax byte,
// con o senza Content-Length, e restituisce la risposta e i byte letti.
func grosso(g *gin.Engine, rotta string, conLunghezza bool) (*httptest.ResponseRecorder, int64) {
	b := catena(strings.Repeat("x", middleware.CorpoMax))[0]
	corpo, _ := json.Marshal(b)
	letti := &contatore{r: strings.NewReader(string(corpo))}
	req := httptest.NewRequest("POST", rotta, letti)
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = -1
	if conLunghezza {
		req.ContentLength = int64(len(corpo))
	}
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	return rec, letti.n
}

// TestCorpoOltreIlLimite prova sul router vero che un corpo oltre 1 MiB,
// su /api/audit/terminal (senza login) e su /api/login, non si legge oltre
// il limite e non salva niente: con Content-Length risponde 413 senza
// leggerlo, senza Content-Length il bind fallisce come per un corpo
// sbagliato. Nel log un warn con metodo e rotta.
func TestCorpoOltreIlLimite(t *testing.T) {
	for _, rotta := range []string{"/api/audit/terminal", "/api/login"} {
		for _, conLunghezza := range []bool{true, false} {
			t.Run(rotta+map[bool]string{true: " con Content-Length", false: " senza Content-Length"}[conLunghezza], func(t *testing.T) {
				g, registro := terminaleDiProva(t, false, model.AuditConnTerminale)
				rec, letti := grosso(g, rotta, conLunghezza)
				stato, massimo := 400, int64(middleware.CorpoMax+1)
				if conLunghezza {
					stato, massimo = 413, 0
				}
				if rec.Code != stato || !strings.HasPrefix(rec.Body.String(), `{"error":`) {
					t.Errorf("risposta %d %s, attesi %d e {\"error\": ...}", rec.Code, rec.Body, stato)
				}
				if letti > massimo {
					t.Errorf("letti %d byte del corpo, al massimo %d", letti, massimo)
				}
				if n := righe(t, "audit_terminals"); n != 0 {
					t.Errorf("audit_terminals ha %d righe, attese 0", n)
				}
				if n := righe(t, "user_tokens"); n != 1 {
					t.Errorf("user_tokens ha %d righe, attesa 1 (quella del pannello)", n)
				}
				nelLog := registro.String()
				if logger.Conta(nelLog, "WARN", "POST "+rotta+": corpo della richiesta oltre 1048576 byte") != 1 {
					t.Errorf("atteso un warn sul corpo:\n%s", nelLog)
				}
				if strings.Contains(nelLog, "xxxx") || strings.Contains(nelLog, base64.StdEncoding.EncodeToString([]byte("xxx"))) {
					t.Errorf("nel log c'e' il corpo:\n%s", nelLog)
				}
			})
		}
	}
}

// TestCorpoBloccoPieno prova sul router vero che un blocco della
// trascrizione da 64 KiB, il corpo vero piu' grande, passa il limite.
func TestCorpoBloccoPieno(t *testing.T) {
	g, registro := terminaleDiProva(t, false, model.AuditConnTerminale)
	manda(t, g, catena(strings.Repeat("x", model.BloccoTerminaleMax))...)
	if want, got := []int64{1}, salvati(t); !reflect.DeepEqual(want, got) {
		t.Errorf("seq salvati: %+v, attesi %+v", got, want)
	}
	if nelLog := registro.String(); nelLog != "" {
		t.Errorf("blocco valido, nel log:\n%s", nelLog)
	}
}
