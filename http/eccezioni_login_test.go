package http

import (
	"strings"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/global"
	applog "github.com/lejianwen/rustdesk-api/v2/lib/logger"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/utils"
)

// eccezioniDiProva prepara il login con password con l'interruttore acceso
// se spento e' falso: oltre a prova (che non e' un'eccezione) un agente di
// prova, collaudo (persona) ed emergenza (amministratore), questi due
// nell'elenco, tutti con passwordDiProva.
func eccezioniDiProva(t *testing.T, spento bool) (*strings.Builder, func(rotta, utente, password string) (int, string)) {
	t.Helper()
	g, utente, registro := pannello(t, false)
	preparaLogin(t, utente)
	hash, err := utils.EncryptPassword(passwordDiProva)
	if err != nil {
		t.Fatal(err)
	}
	si := true
	for _, u := range []*model.User{
		{Username: "agente-prova", AgenteDi: utente.Id},
		{Username: "collaudo"},
		{Username: "emergenza", IsAdmin: &si},
	} {
		u.Status, u.Password = model.COMMON_STATUS_ENABLE, hash
		crea(t, u)
	}
	prec := global.Config.App
	t.Cleanup(func() { global.Config.App = prec })
	global.Config.App.DisablePwdLogin = spento
	global.Config.App.PwdLoginEccezioni = " collaudo,, emergenza "
	login := func(rotta, nome, password string) (int, string) {
		rec := richiesta(g, "POST", rotta, "", `{"username":"`+nome+`","password":"`+password+`","id":"999000111","uuid":"dXVpZA=="}`)
		return rec.Code, rec.Body.String()
	}
	return registro, login
}

// TestEccezioniLoginPassword prova sul router vero app.pwd-login-eccezioni
// con app.disable-pwd-login acceso: un utente qualunque, uno che non esiste e
// un'eccezione con la password sbagliata hanno la stessa risposta di prima;
// l'agente entra solo dal client, i nomi dell'elenco da tutte e due le rotte,
// e ogni login per eccezione lascia una riga INFO, warn per l'amministratore.
func TestEccezioniLoginPassword(t *testing.T) {
	registro, login := eccezioniDiProva(t, true)
	const (
		client  = `{"error":"L'accesso con password è disattivato."}`
		pannell = `{"code":101,"message":"L'accesso con password è disattivato.","data":null}`
	)
	global.LoginLimiter = utils.NewLoginLimiter(utils.SecurityPolicy{CaptchaThreshold: -1})
	for _, tc := range []struct {
		rotta, utente, password string
		entra                   bool
	}{
		{"/api/login", "prova", passwordDiProva, false},
		{"/api/login", "nessuno", passwordDiProva, false},
		{"/api/login", "collaudo", "sbagliata", false},
		{"/api/login", "agente-prova", passwordDiProva, true},
		{"/api/login", "collaudo", passwordDiProva, true},
		{"/api/login", "emergenza", passwordDiProva, true},
		{"/api/admin/login", "prova", passwordDiProva, false},
		{"/api/admin/login", "nessuno", passwordDiProva, false},
		{"/api/admin/login", "emergenza", "sbagliata", false},
		{"/api/admin/login", "agente-prova", passwordDiProva, false},
		{"/api/admin/login", "collaudo", passwordDiProva, true},
		{"/api/admin/login", "emergenza", passwordDiProva, true},
	} {
		codice, corpo := login(tc.rotta, tc.utente, tc.password)
		rifiuto, codiceRifiuto := client, 400
		if tc.rotta == "/api/admin/login" {
			rifiuto, codiceRifiuto = pannell, 200
		}
		if tc.entra && (codice != 200 || !strings.Contains(corpo, "token")) {
			t.Errorf("POST %s di %s: %d %s, atteso il token", tc.rotta, tc.utente, codice, corpo)
		}
		if !tc.entra && (codice != codiceRifiuto || corpo != rifiuto) {
			t.Errorf("POST %s di %s: %d %s, atteso %d %s", tc.rotta, tc.utente, codice, corpo, codiceRifiuto, rifiuto)
		}
	}
	for _, tc := range []struct {
		livello string
		parti   []string
		n       int
	}{
		{"INFO", []string{"login con password per eccezione: utente agente-prova, motivo agente"}, 1},
		{"INFO", []string{"login con password per eccezione: utente collaudo, motivo elenco"}, 2},
		{"WARN", []string{"login con password per eccezione: utente emergenza (amministratore), motivo elenco"}, 2},
		{"INFO", []string{"per eccezione"}, 3},
	} {
		if n := applog.Conta(registro.String(), tc.livello, tc.parti...); n != tc.n {
			t.Errorf("%d righe %s %q, attese %d:\n%s", n, tc.livello, tc.parti, tc.n, registro)
		}
	}
}

// TestEccezioniInterruttoreSpento prova che con app.disable-pwd-login spento
// l'elenco non conta: entra anche chi non ci sta, la password sbagliata ha la
// risposta di sempre e nessun login scrive la riga dell'eccezione.
func TestEccezioniInterruttoreSpento(t *testing.T) {
	registro, login := eccezioniDiProva(t, false)
	for _, rotta := range []string{"/api/login", "/api/admin/login"} {
		for _, nome := range []string{"prova", "collaudo", "emergenza"} {
			if codice, corpo := login(rotta, nome, passwordDiProva); codice != 200 || !strings.Contains(corpo, "token") {
				t.Errorf("POST %s di %s: %d %s, atteso il token", rotta, nome, codice, corpo)
			}
		}
	}
	if codice, corpo := login("/api/login", "collaudo", "sbagliata"); codice != 400 || corpo != `{"error":"Nome utente o password errati."}` {
		t.Errorf("POST /api/login con la password sbagliata: %d %s", codice, corpo)
	}
	if strings.Contains(registro.String(), "per eccezione") {
		t.Errorf("riga dell'eccezione con l'interruttore spento:\n%s", registro)
	}
}

// TestEccezioniBan prova che con l'interruttore acceso ogni rifiuto conta
// come tentativo fallito, anche quello di chi non e' un'eccezione, e che il
// ban ferma anche un'eccezione con la password giusta.
func TestEccezioniBan(t *testing.T) {
	_, login := eccezioniDiProva(t, true)
	global.LoginLimiter = utils.NewLoginLimiter(utils.SecurityPolicy{CaptchaThreshold: 100, BanThreshold: 3})
	login("/api/login", "prova", passwordDiProva)
	login("/api/login", "nessuno", passwordDiProva)
	if codice, corpo := login("/api/login", "collaudo", passwordDiProva); codice != 200 {
		t.Fatalf("POST /api/login di collaudo prima del ban: %d %s", codice, corpo)
	}
	login("/api/admin/login", "emergenza", "sbagliata")
	if codice, corpo := login("/api/login", "collaudo", passwordDiProva); strings.Contains(corpo, "token") || !strings.Contains(corpo, `"code":423`) {
		t.Errorf("POST /api/login di collaudo dopo tre rifiuti: %d %s, atteso il ban", codice, corpo)
	}
}
