package http

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// codiceNellaPagina trova il codice dell'associazione in oauth_codice.html.
var codiceNellaPagina = regexp.MustCompile(`var codice = '([A-HJKMNP-Z2-9]{4}-[A-HJKMNP-Z2-9]{4})'`)

// richiesteDiProva prepara il router (amministratore se admin), il provider
// finto "aziendale" senza autoregistrazione e il login con password spento
// se spento; le richieste di associazione partono vuote.
func richiesteDiProva(t *testing.T, admin, spento bool) (*gin.Engine, *model.User, *strings.Builder) {
	t.Helper()
	g, utente, registro := pannello(t, admin)
	crea(t, &model.Oauth{Op: "aziendale", OauthType: model.OauthTypeOidc, ClientId: "id", ClientSecret: "segreto", Issuer: providerOidc(t)})
	prima := global.Config.App.DisablePwdLogin
	global.Config.App.DisablePwdLogin = spento
	service.RichiesteAssociazione.Svuota()
	t.Cleanup(func() {
		global.Config.App.DisablePwdLogin = prima
		service.RichiesteAssociazione.Svuota()
	})
	return g, utente, registro
}

// accesso fa il login OIDC del client fino al callback del provider finto,
// toglie il login dalla cache e restituisce corpo e stato del callback.
func accesso(t *testing.T, g *gin.Engine) (string, int) {
	t.Helper()
	rec := richiesta(g, "POST", "/api/oidc/auth", "", `{"op":"aziendale","id":"999000111","uuid":"dXVpZA==","deviceInfo":{"os":"windows","type":"client","name":"PC"}}`)
	var r struct{ Code string }
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil || rec.Code != 200 {
		t.Fatalf("POST /api/oidc/auth: %d %s (%v)", rec.Code, rec.Body, err)
	}
	rec = richiesta(g, "GET", "/api/oidc/callback?state="+r.Code+"&code="+codiceDelProvider, "", "")
	// il provider finto vuole un solo login in corso
	service.AllService.OauthService.DeleteOauthCache(r.Code)
	return rec.Body.String(), rec.Code
}

// elenco legge le richieste in attesa dal pannello.
func elenco(t *testing.T, g *gin.Engine) []map[string]any {
	t.Helper()
	rec := richiesta(g, "GET", "/api/admin/oauth/richieste", "", "")
	var r struct {
		Code int
		Data []map[string]any
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil || r.Code != 0 {
		t.Fatalf("GET richieste: %d %s (%v)", rec.Code, rec.Body, err)
	}
	return r.Data
}

// TestAssociazioneConCodice prova sul router vero, col login con password
// spento, il tecnico nuovo: il callback mostra una pagina col codice invece
// di mandare al pannello, l'amministratore vede la richiesta senza sub e la
// associa a un utente, il log dice chi senza sub ne' email, e al login dopo
// lo stesso account entra. Un secondo accesso prima dell'associazione rinnova
// la richiesta con un codice nuovo. Prima il callback mandava alla pagina di
// associazione, che vuole il login al pannello.
func TestAssociazioneConCodice(t *testing.T) {
	g, amministratore, registro := richiesteDiProva(t, true, true)
	tecnico := &model.User{Username: "tecnico", Status: model.COMMON_STATUS_ENABLE}
	crea(t, tecnico)

	vecchio, _ := accesso(t, g)
	pagina, stato := accesso(t, g)
	trovato := codiceNellaPagina.FindStringSubmatch(pagina)
	if stato != 200 || trovato == nil || !strings.Contains(pagina, "var msg = 'OauthBindPendingCode'") {
		t.Fatalf("GET /api/oidc/callback: %d, attesa la pagina col codice\n%s", stato, pagina)
	}
	codice := trovato[1]
	if strings.Contains(vecchio, codice) {
		t.Errorf("il secondo accesso ha lo stesso codice del primo: %s", codice)
	}
	lista := elenco(t, g)
	if len(lista) != 1 || lista[0]["codice"] != codice || lista[0]["op"] != "aziendale" || lista[0]["nome"] != "Utente OIDC" ||
		lista[0]["email"] != "utente@esempio.it" || lista[0]["email_verificata"] != true || lista[0]["sub"] != nil {
		t.Fatalf("richieste: %v, attesa una sola col codice %s", lista, codice)
	}

	corpo := `{"codice":"` + strings.ToLower(strings.Replace(codice, "-", " ", 1)) + `","user_id":` + strconv.Itoa(int(tecnico.Id)) + `}`
	if rec := richiesta(g, "POST", "/api/admin/oauth/associa", "", corpo); rec.Body.String() != `{"code":0,"message":"success","data":null}` {
		t.Fatalf("POST associa: %d %s", rec.Code, rec.Body)
	}
	riga := "associazione OIDC dall'amministratore: utente " + strconv.Itoa(int(tecnico.Id)) + ", provider aziendale, amministratore " + strconv.Itoa(int(amministratore.Id))
	if !strings.Contains(registro.String(), riga) || strings.Contains(registro.String(), "sub-1") || strings.Contains(registro.String(), "utente@esempio.it") {
		t.Errorf("log: attesa la riga %q, senza sub ne' email\n%s", riga, registro)
	}
	if lista := elenco(t, g); len(lista) != 0 {
		t.Errorf("richieste dopo l'associazione: %v", lista)
	}

	if pagina, _ = accesso(t, g); !strings.Contains(pagina, "var msg = 'OauthSuccess'") {
		t.Errorf("accesso dopo l'associazione: atteso OauthSuccess\n%s", pagina)
	}
	// username come la scrive bindConfirm: il preferred_username del provider
	ut, err := service.AllService.OauthService.UserThirdInfo("aziendale", "sub-1")
	if err != nil || ut.UserId != tecnico.Id || ut.Username != "utente-oidc" {
		t.Errorf("associazione di sub-1: %+v (%v), attesa all'utente %d con username utente-oidc", ut, err, tecnico.Id)
	}
}

// TestAssociazioneConPassword prova sul router vero che col login con
// password acceso il callback manda ancora alla pagina di associazione del
// pannello, che legge il codice da /api/admin/oauth/info; l'associazione da
// quella pagina toglie la richiesta.
func TestAssociazioneConPassword(t *testing.T) {
	g, _, _ := richiesteDiProva(t, false, false)
	rec := richiesta(g, "POST", "/api/oidc/auth", "", `{"op":"aziendale","id":"999000111","uuid":"dXVpZA=="}`)
	var r struct{ Code string }
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { service.AllService.OauthService.DeleteOauthCache(r.Code) })
	rec = richiesta(g, "GET", "/api/oidc/callback?state="+r.Code+"&code="+codiceDelProvider, "", "")
	if rec.Code != 302 || rec.Header().Get("Location") != "/_admin/#/oauth/bind/"+r.Code {
		t.Fatalf("GET /api/oidc/callback: %d %q, atteso il redirect alla pagina di associazione", rec.Code, rec.Header().Get("Location"))
	}
	codice := service.RichiesteAssociazione.Elenco()[0].Codice
	rec = richiesta(g, "GET", "/api/admin/oauth/info?code="+r.Code, "", "")
	if !strings.Contains(rec.Body.String(), `"codice":"`+codice+`"`) {
		t.Errorf("GET info: %s, manca il codice %s", rec.Body, codice)
	}
	if rec := richiesta(g, "POST", "/api/admin/oauth/bindConfirm", "", `{"code":"`+r.Code+`"}`); !strings.Contains(rec.Body.String(), `"code":0`) {
		t.Fatalf("POST bindConfirm: %s", rec.Body)
	}
	if n := len(service.RichiesteAssociazione.Elenco()); n != 0 {
		t.Errorf("richieste dopo bindConfirm: %d, attese 0", n)
	}
}

// TestAssociaRifiuti prova sul router vero i rifiuti dell'associazione
// dall'amministratore: nessuna associazione nasce e la richiesta resta.
func TestAssociaRifiuti(t *testing.T) {
	for _, tc := range []struct {
		nome, atteso string
		admin        bool
		prepara      func(t *testing.T, destinatario *model.User) string // restituisce il codice
	}{
		{"non amministratore", `{"code":403,"message":"Non hai i permessi per questa operazione.","data":null}`, false, nil},
		{"agente", `{"code":101,"message":"Un account OIDC non si associa a un account agente.","data":null}`, true, func(_ *testing.T, d *model.User) string {
			service.DB.Model(d).Update("agente_di", 1)
			return ""
		}},
		{"utente disattivato", `{"code":101,"message":"Utente disattivato.","data":null}`, true, func(_ *testing.T, d *model.User) string {
			service.DB.Model(d).Update("status", model.COMMON_STATUS_DISABLED)
			return ""
		}},
		{"sub di un altro", `{"code":101,"message":"` + testiDelLegame["OauthHasBindOtherUser"][""] + `","data":null}`, true, func(t *testing.T, _ *model.User) string {
			altro := &model.User{Username: "altro"}
			crea(t, altro)
			associazione(t, altro.Id, "sub-1")
			return ""
		}},
		{"utente col provider", `{"code":101,"message":"` + testiDelLegame["OauthUserHasBind"][""] + `","data":null}`, true, func(t *testing.T, d *model.User) string {
			associazione(t, d.Id, "sub-9")
			return ""
		}},
		{"codice sconosciuto", `{"code":101,"message":"Codice sconosciuto o scaduto: il tecnico rifaccia l'accesso per averne uno nuovo.","data":null}`, true, func(_ *testing.T, _ *model.User) string {
			return "AAAA-AAAA"
		}},
		{"codice scaduto", `{"code":101,"message":"Codice sconosciuto o scaduto: il tecnico rifaccia l'accesso per averne uno nuovo.","data":null}`, true, func(_ *testing.T, _ *model.User) string {
			dopo := time.Now().Add(service.DurataRichiesta)
			service.RichiesteAssociazione.Ora = func() time.Time { return dopo }
			return ""
		}},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g, _, _ := richiesteDiProva(t, tc.admin, true)
			destinatario := &model.User{Username: "tecnico", Status: model.COMMON_STATUS_ENABLE}
			crea(t, destinatario)
			codice, err := service.RichiesteAssociazione.Nuova("aziendale", &model.OauthUser{OpenId: "sub-1", Name: "Utente OIDC"})
			if err != nil {
				t.Fatal(err)
			}
			if tc.prepara != nil {
				if c := tc.prepara(t, destinatario); c != "" {
					codice = c
				}
			}
			prima := righe(t, "user_thirds")
			corpo := `{"codice":"` + codice + `","user_id":` + strconv.Itoa(int(destinatario.Id)) + `}`
			if rec := richiesta(g, "POST", "/api/admin/oauth/associa", "", corpo); rec.Body.String() != tc.atteso {
				t.Errorf("POST associa: %d %s\n want %s", rec.Code, rec.Body, tc.atteso)
			}
			if !tc.admin {
				if rec := richiesta(g, "GET", "/api/admin/oauth/richieste", "", ""); rec.Body.String() != tc.atteso {
					t.Errorf("GET richieste: %d %s\n want %s", rec.Code, rec.Body, tc.atteso)
				}
			}
			if n := righe(t, "user_thirds"); n != prima {
				t.Errorf("associazioni dopo il rifiuto: %d, erano %d", n, prima)
			}
			if tc.nome != "codice scaduto" && len(service.RichiesteAssociazione.Elenco()) != 1 {
				t.Errorf("la richiesta e' uscita dopo il rifiuto")
			}
		})
	}
}

// TestRichiesteTetto prova che oltre 100 richieste esce la piu' vecchia, e
// che il pannello le elenca dalla piu' recente.
func TestRichiesteTetto(t *testing.T) {
	g, _, _ := richiesteDiProva(t, true, true)
	var primo string
	for i := range service.TettoRichieste + 1 {
		codice, err := service.RichiesteAssociazione.Nuova("aziendale", &model.OauthUser{OpenId: "sub-" + strconv.Itoa(i)})
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			primo = codice
		}
	}
	lista := elenco(t, g)
	if len(lista) != service.TettoRichieste {
		t.Fatalf("richieste: %d, attese %d", len(lista), service.TettoRichieste)
	}
	for _, r := range lista {
		if r["codice"] == primo {
			t.Errorf("la richiesta piu' vecchia (%s) e' ancora nell'elenco", primo)
		}
	}
	rec := richiesta(g, "POST", "/api/admin/oauth/associa", "", `{"codice":"`+primo+`","user_id":1}`)
	if !strings.Contains(rec.Body.String(), "Codice sconosciuto") {
		t.Errorf("POST associa col codice uscito: %s", rec.Body)
	}
}
