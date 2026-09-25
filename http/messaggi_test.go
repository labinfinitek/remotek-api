package http

import (
	"cmp"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/lejianwen/rustdesk-api/v2/config"
	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/lib/lock"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
	"github.com/lejianwen/rustdesk-api/v2/utils"
)

// TestMessaggi prova i messaggi delle risposte sul router vero, con il
// conf/config.yaml del repo senza RUSTDESK_API_LANG, un database sqlite
// temporaneo e un provider OIDC che risponde 500: l'errore di rete che ne
// esce non e' un ID, e al client deve arrivare SystemError, non il suo testo.
// Un ID che manca in una lingua (NoCaptchaRequired in es.toml) ripiega
// sull'inglese. Messaggi e validatore seguono la stessa regola: la lingua di
// Accept-Language se l'API la ha, altrimenti quella configurata (conf, se la
// riga la fissa; se no l'italiano del file), col nome del campo in quella
// lingua. Senza Accept-Language, come dal client RustDesk, i due testi che il
// client confronta alla lettera restano quelli di prima. Un corpo JSON rotto
// alle rotte del client senza autenticazione riceve ParamsError, e il testo
// dell'errore di JSON va solo nel log. Lo stato globale e' quello di
// InitGlobal, ridotto a cio' che serve a queste richieste; il test non
// scrive nel repo.
func TestMessaggi(t *testing.T) {
	t.Setenv("RUSTDESK_API_LANG", "")
	t.Setenv("RUSTDESK_API_GIN_MODE", "test")
	t.Chdir("..") // il router legge resources/ dalla cartella corrente
	global.Viper = config.Init(&global.Config, filepath.Join("conf", "config.yaml"))
	var registro strings.Builder
	global.Logger = logrus.New()
	global.Logger.SetOutput(&registro)
	global.InitI18n()
	global.ApiInitValidator()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "api.db")), &gorm.Config{Logger: logger.Discard})
	if err == nil {
		err = db.AutoMigrate(&model.Oauth{}, &model.Peer{}, &model.LoginLog{})
	}
	if err != nil {
		t.Fatal(err)
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "dettaglio interno del provider", http.StatusInternalServerError)
	}))
	t.Cleanup(provider.Close)
	oidc := &model.Oauth{Op: "prova", OauthType: model.OauthTypeOidc, ClientId: "id", ClientSecret: "segreto", Issuer: provider.URL}
	if err := db.Create(oidc).Error; err != nil {
		t.Fatal(err)
	}
	service.New(&global.Config, db, global.Logger, nil, lock.NewLocal())
	global.LoginLimiter = utils.NewLoginLimiter(utils.SecurityPolicy{CaptchaThreshold: global.Config.App.CaptchaThreshold})
	service.AllService.SetOauthCache("in-corso", &service.OauthCacheItem{Action: service.OauthActionTypeLogin}, 0)
	g := NewEngine()

	configurata := global.Config.Lang
	for _, tc := range []struct{ metodo, percorso, lingua, conf, corpo, atteso string }{
		{"GET", "/api/admin/captcha", "es", "", "", `{"code":101,"message":"No verification code is required.","data":null}`},
		{"POST", "/api/oidc/auth", "en", "", `{"op":"inesistente"}`, `{"error":"Config not found."}`},
		{"POST", "/api/oidc/auth", "en", "", `{"op":"prova"}`, `{"error":"System error."}`},
		{"POST", "/api/login", "it-IT", "en", `{}`, `{"error":"Nome utente è un campo obbligatorio"}`},
		{"POST", "/api/login", "it-IT,it;q=0.9,en;q=0.8", "en", `{}`, `{"error":"Nome utente è un campo obbligatorio"}`},
		{"POST", "/api/login", "en", "it", `{}`, `{"error":"Username is a required field"}`},
		{"POST", "/api/login", "de-DE", "it", `{}`, `{"error":"Nome utente è un campo obbligatorio"}`},
		{"POST", "/api/oidc/auth", "de-DE", "it", `{"op":"inesistente"}`, `{"error":"Configurazione non trovata."}`},
		{"POST", "/api/login", "de-DE", "en", `{}`, `{"error":"Username is a required field"}`},
		{"POST", "/api/login", "", "", `{}`, `{"error":"Nome utente è un campo obbligatorio"}`},
		{"POST", "/api/oidc/auth", "", "", `{"op":"inesistente"}`, `{"error":"Configurazione non trovata."}`},
		{"POST", "/api/sysinfo", "", "", `{"id":"999000111","uuid":"dXVpZA=="}`, "SYSINFO_UPDATED"},
		{"GET", "/api/oidc/auth-query?code=in-corso", "", "", "", `{"error":"No authed oidc is found","message":"Authorization in progress, please login and bind"}`},
	} {
		global.Config.Lang = cmp.Or(tc.conf, configurata)
		req := httptest.NewRequest(tc.metodo, tc.percorso, strings.NewReader(tc.corpo))
		req.Header.Set("Content-Type", "application/json")
		if tc.lingua != "" {
			req.Header.Set("Accept-Language", tc.lingua)
		}
		rec := httptest.NewRecorder()
		g.ServeHTTP(rec, req)
		if got := rec.Body.String(); got != tc.atteso {
			t.Errorf("%s %s %s, Accept-Language %q, lang %q:\n got  %s\n want %s", tc.metodo, tc.percorso, tc.corpo, tc.lingua, global.Config.Lang, got, tc.atteso)
		}
	}
	if !strings.Contains(registro.String(), "dettaglio interno del provider") {
		t.Errorf("l'errore del provider non e' nel log:\n%s", registro.String())
	}

	// Ogni /api/login rifiutato conta per il limiter, che di default chiede il
	// captcha dopo 3 tentativi e banna dopo 10: questo e' il settimo del test,
	// e il limiter del test non banna.
	global.Config.Lang = configurata
	const erroreJSON = "invalid character 'x' looking for beginning of value"
	for _, percorso := range []string{"/api/sysinfo", "/api/audit/conn", "/api/audit/file", "/api/login"} {
		registro.Reset()
		req := httptest.NewRequest("POST", percorso, strings.NewReader(`{"id": x}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		g.ServeHTTP(rec, req)
		if got, want := rec.Body.String(), `{"error":"Parametri non validi."}`; got != want {
			t.Errorf("POST %s con JSON rotto:\n got  %s\n want %s", percorso, got, want)
		}
		if nelLog := registro.String(); !strings.Contains(nelLog, "POST "+percorso+": ") || !strings.Contains(nelLog, erroreJSON) {
			t.Errorf("POST %s con JSON rotto, nel log mancano rotta o errore:\n%s", percorso, nelLog)
		}
	}
}
