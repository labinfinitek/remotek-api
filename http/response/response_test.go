package response

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/lejianwen/rustdesk-api/v2/global"
)

// TestErrorErr prova ErrorErr e FailErr su un router di gin, con i file di
// lingua del repo e lang en. Nella risposta va il messaggio dell'ID che sta
// nella catena dell'errore, altrimenti quello del punto (qui
// OperationFailed); nel log una riga sola, con metodo, rotta del router e
// testo dell'errore, che nella risposta non c'e'.
func TestErrorErr(t *testing.T) {
	gin.SetMode(gin.TestMode)
	global.Config.Gin.ResourcesPath = filepath.Join("..", "..", "resources")
	global.Config.Lang = "en"
	var registro strings.Builder
	global.Logger = logrus.New()
	global.Logger.SetOutput(&registro)
	global.Logger.SetFormatter(&logrus.TextFormatter{DisableQuote: true, DisableTimestamp: true})
	global.InitI18n()

	interno := errors.New("dial tcp 192.0.2.1:5432: connect: connection refused")
	for _, tc := range []struct {
		nome              string
		err               error
		messaggio, nelLog string
	}{
		{"ID avvolto da %w", fmt.Errorf("registrazione: %w", errors.New("UsernameExists")),
			"Username already exists.", `UsernameExists, errore "registrazione: UsernameExists"`},
		{"errors.Join di un ID e un dettaglio", errors.Join(errors.New("LdapConnectFailed"), interno),
			"Cannot connect to the LDAP server.", `LdapConnectFailed, errore "LdapConnectFailed\ndial tcp 192.0.2.1:5432: connect: connection refused"`},
		{"errore interno", interno,
			"the operation failed.", `OperationFailed, errore "dial tcp 192.0.2.1:5432: connect: connection refused"`},
	} {
		for _, f := range []struct {
			nome     string
			risponde gin.HandlerFunc
			stato    int
			corpo    string
		}{
			{"ErrorErr", func(c *gin.Context) { ErrorErr(c, "OperationFailed", tc.err) },
				http.StatusBadRequest, `{"error":"` + tc.messaggio + `"}`},
			{"FailErr", func(c *gin.Context) { FailErr(c, 101, "OperationFailed", tc.err) },
				http.StatusOK, `{"code":101,"message":"` + tc.messaggio + `","data":null}`},
		} {
			registro.Reset()
			g := gin.New()
			g.POST("/prova/:guid", f.risponde)
			rec := httptest.NewRecorder()
			g.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/prova/1-2-3", nil))
			if rec.Code != f.stato || rec.Body.String() != f.corpo {
				t.Errorf("%s, %s:\n got  %d %s\n want %d %s", tc.nome, f.nome, rec.Code, rec.Body, f.stato, f.corpo)
			}
			if riga := "level=warning msg=POST /prova/:guid: al client va " + tc.nelLog + "\n"; registro.String() != riga {
				t.Errorf("%s, %s, log:\n got  %s want %s", tc.nome, f.nome, registro.String(), riga)
			}
		}
	}
}

// TestRipiegoInglese prova con un bundle di prova, lang it, che un ID che
// manca in it.toml esca in inglese e senza riga nel log, sia con
// Accept-Language italiano sia senza, come dal client RustDesk; un ID che
// c'e' esce in italiano. Coi file del repo non si prova: it.toml ha tutte le
// chiavi di en.toml (TestChiaviDeiFileDiLingua).
func TestRipiegoInglese(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := filepath.Join(t.TempDir(), "i18n")
	err := errors.Join(os.Mkdir(dir, 0o750),
		os.WriteFile(filepath.Join(dir, "en.toml"), []byte("[SystemError]\nother = \"System error.\"\n[SoloInglese]\nother = \"Only in English.\"\n"), 0o600),
		os.WriteFile(filepath.Join(dir, "it.toml"), []byte("[SystemError]\nother = \"Errore di sistema.\"\n"), 0o600))
	if err != nil {
		t.Fatal(err)
	}
	global.Config.Gin.ResourcesPath = filepath.Dir(dir)
	global.Config.Lang = "it"
	var registro strings.Builder
	global.Logger = logrus.New()
	global.Logger.SetOutput(&registro)
	global.InitI18n()

	for _, lingua := range []string{"it-IT", ""} {
		for id, atteso := range map[string]string{"SoloInglese": "Only in English.", "SystemError": "Errore di sistema."} {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			if lingua != "" {
				c.Request.Header.Set("Accept-Language", lingua)
			}
			if got := TranslateMsg(c, id); got != atteso {
				t.Errorf("%s, Accept-Language %q: %q, atteso %q", id, lingua, got, atteso)
			}
		}
	}
	if registro.Len() != 0 {
		t.Errorf("il log doveva essere vuoto:\n%s", registro.String())
	}
}
