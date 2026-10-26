package http

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/global"
	applog "github.com/lejianwen/rustdesk-api/v2/lib/logger"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// tecniciDiProva prepara lasciapassareDiProva con: mario (2) anche con un
// token nato da OIDC, collaudo-contratto (6, non amministratore) col token
// della password, il PC 999000333 in nessuna rubrica, e in
// app.pwd-login-eccezioni prova (1, l'amministratore del pannello, token
// della password) e collaudo-contratto.
func tecniciDiProva(t *testing.T, accesa bool) (*gin.Engine, ed25519.PublicKey, *strings.Builder) {
	t.Helper()
	g, pub, registro := lasciapassareDiProva(t, accesa)
	if err := service.DB.AutoMigrate(&model.LoginLog{}); err != nil {
		t.Fatal(err)
	}
	ora := time.Now().Add(time.Hour).Unix()
	crea(t, &model.UserToken{UserId: 2, Token: "tok-mario-oidc", TipoLogin: model.LoginLogTypeOauth, ExpiredAt: ora})
	no := false
	collaudo := &model.User{Username: "collaudo-contratto", GroupId: 1, IsAdmin: &no, Status: model.COMMON_STATUS_ENABLE}
	crea(t, collaudo)
	crea(t, &model.UserToken{UserId: collaudo.Id, Token: "tok-collaudo", TipoLogin: model.LoginLogTypeAccount, ExpiredAt: ora})
	crea(t, &model.Peer{Id: "999000333", Hostname: "PC-BIANCHI", Uuid: "YWx0cm8"})
	prec := global.Config.App.PwdLoginEccezioni
	t.Cleanup(func() { global.Config.App.PwdLoginEccezioni = prec })
	global.Config.App.PwdLoginEccezioni = "prova, collaudo-contratto"
	return g, pub, registro
}

// payload verifica la firma del lasciapassare lp e ne restituisce il
// payload.
func payload(t *testing.T, pub ed25519.PublicKey, lp string) service.PayloadLasciapassare {
	t.Helper()
	pl, firma, _ := strings.Cut(lp, ".")
	pb, err1 := base64.RawURLEncoding.DecodeString(pl)
	fb, err2 := base64.RawURLEncoding.DecodeString(firma)
	if err1 != nil || err2 != nil || !ed25519.Verify(pub, append([]byte(service.PrefissoLasciapassare), pb...), fb) {
		t.Fatalf("firma non verificata (%v, %v): %s", err1, err2, lp)
	}
	var p service.PayloadLasciapassare
	if err := json.Unmarshal(pb, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestLasciapassareTecnico prova sul router vero POST /api/lasciapassare
// (ADR-0032): il tecnico entrato con OIDC lo riceve per qualunque PC in
// peers, anche fuori dalla sua rubrica, modo tecnico e agente vuoto, con una
// riga INFO; l'admin di emergenza (amministratore in
// app.pwd-login-eccezioni) col token della password lo riceve con un WARN.
// Rifiutati, con un messaggio tradotto e senza registro: il tecnico entrato
// con la password, collaudo-contratto (nell'elenco ma non amministratore),
// un amministratore fuori dall'elenco, un agente AI (che ha la sua rotta),
// la chiave spenta, un PC che non e' in peers. Nessun evento verso
// l'integrazione.
func TestLasciapassareTecnico(t *testing.T) {
	for _, tc := range []struct {
		caso, token, pc string
		accesa          bool
		prima           func()
		tecnico, errore string
		warn            bool
	}{
		{"tecnico OIDC, PC nella rubrica", "tok-mario-oidc", "999000111", true, nil, "mario", "", false},
		{"tecnico OIDC, PC fuori rubrica", "tok-mario-oidc", "999000333", true, nil, "mario", "", false},
		{"admin di emergenza", tokenDelPannello, "999000333", true, nil, "prova", "", true},
		{"tecnico con la password", "tok-mario", "999000111", true, nil, "", "Il lasciapassare per i PC va solo a chi è entrato con OIDC: esci e rientra con l'accesso aziendale.", false},
		{"collaudo-contratto", "tok-collaudo", "999000111", true, nil, "", "Il lasciapassare per i PC va solo a chi è entrato con OIDC: esci e rientra con l'accesso aziendale.", false},
		{"amministratore fuori dall'elenco", tokenDelPannello, "999000111", true, func() { global.Config.App.PwdLoginEccezioni = "collaudo-contratto" }, "", "Il lasciapassare per i PC va solo a chi è entrato con OIDC: esci e rientra con l'accesso aziendale.", false},
		{"agente AI", "tok-agente-mario", "999000111", true, nil, "", "Un agente AI chiede il lasciapassare con /api/agente/lasciapassare.", false},
		{"chiave spenta", "tok-mario-oidc", "999000111", false, nil, "", "Il lasciapassare dell'agente non è configurato su questo server.", false}, //nolint:misspell // italiano
		{"PC che non e' in peers", "tok-mario-oidc", "999000222", true, nil, "", "Nessun PC con questo ID è noto al server.", false},
	} {
		t.Run(tc.caso, func(t *testing.T) {
			g, pub, registro := tecniciDiProva(t, tc.accesa)
			if tc.prima != nil {
				tc.prima()
			}
			prima := time.Now().Unix()
			rec := daTecnico(g, "POST", "/api/lasciapassare", tc.token, `{"id":"`+tc.pc+`"}`)
			var righe []model.Lasciapassare
			if err := service.DB.Find(&righe).Error; err != nil {
				t.Fatal(err)
			}
			var eventi int64
			if err := service.DB.Model(&model.Evento{}).Count(&eventi).Error; err != nil || eventi != 0 {
				t.Errorf("%d eventi in coda (%v), attesi 0", eventi, err)
			}
			log := registro.String()
			if tc.errore != "" {
				want, _ := json.Marshal(map[string]string{"error": tc.errore})
				if rec.Code != 400 || rec.Body.String() != string(want) || len(righe) != 0 {
					t.Errorf("%d %s, %d lasciapassare; attesi 400 %s e nessuno", rec.Code, rec.Body, len(righe), want)
				}
				return
			}
			var res struct{ Lasciapassare string }
			if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &res) != nil {
				t.Fatalf("%d %s", rec.Code, rec.Body)
			}
			p := payload(t, pub, res.Lasciapassare)
			if p.V != 1 || p.Pc != tc.pc || p.Agente != "" || p.Tecnico != tc.tecnico || p.Modo != "tecnico" ||
				p.Emesso < prima || p.Scade != p.Emesso+60 {
				t.Errorf("payload %+v", p)
			}
			var tecnico model.User
			if err := service.DB.Where("username = ?", tc.tecnico).First(&tecnico).Error; err != nil {
				t.Fatal(err)
			}
			want := model.Lasciapassare{IdModel: righe[0].IdModel, Nonce: p.Nonce, Pc: tc.pc, AgenteId: 0, TecnicoId: tecnico.Id,
				Modo: "tecnico", Capacita: "terminale", Emesso: p.Emesso, Scade: p.Scade, Creato: p.Emesso}
			if len(righe) != 1 || righe[0] != want {
				t.Errorf("registro %+v, atteso %+v", righe, want)
			}
			emergenza := applog.Conta(log, "WARN", "lasciapassare all'admin di emergenza: utente "+tc.tecnico+", PC "+tc.pc)
			info := applog.Conta(log, "INFO", "lasciapassare rilasciato al tecnico "+tc.tecnico+", PC "+tc.pc)
			if tc.warn && (emergenza != 1 || info != 0) || !tc.warn && (emergenza != 0 || info != 1) || strings.Contains(log, p.Nonce) {
				t.Errorf("warn %d, info %d (warn atteso %t):\n%s", emergenza, info, tc.warn, log)
			}
		})
	}
}

// TestLasciapassareAgenteModo prova sul router vero il campo modo di POST
// /api/agente/lasciapassare: assente o amministratore danno il modo di
// sempre, utente il modo utente, anche nel registro e nell'evento
// agente.lasciapassare (amministratore falso); un altro valore e' rifiutato
// senza registro.
func TestLasciapassareAgenteModo(t *testing.T) {
	for _, tc := range []struct{ corpo, modo string }{
		{`{"id":"999000111"}`, "amministratore"},
		{`{"id":"999000111","modo":"amministratore"}`, "amministratore"},
		{`{"id":"999000111","modo":"utente"}`, "utente"},
		{`{"id":"999000111","modo":"tecnico"}`, ""},
	} {
		t.Run(tc.corpo, func(t *testing.T) {
			g, pub, _ := lasciapassareDiProva(t, true)
			rec := daTecnico(g, "POST", "/api/agente/lasciapassare", "tok-agente-mario", tc.corpo)
			var righe []model.Lasciapassare
			if err := service.DB.Find(&righe).Error; err != nil {
				t.Fatal(err)
			}
			if tc.modo == "" {
				if rec.Code != 400 || rec.Body.String() != `{"error":"Parametri non validi."}` || len(righe) != 0 {
					t.Errorf("%d %s, %d lasciapassare", rec.Code, rec.Body, len(righe))
				}
				return
			}
			var res struct{ Lasciapassare string }
			if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &res) != nil {
				t.Fatalf("%d %s", rec.Code, rec.Body)
			}
			if p := payload(t, pub, res.Lasciapassare); p.Modo != tc.modo || p.Agente != "agente-mario" || p.Tecnico != "mario" {
				t.Errorf("payload %+v, modo atteso %s", p, tc.modo)
			}
			if len(righe) != 1 || righe[0].Modo != tc.modo || righe[0].AgenteId != 4 {
				t.Errorf("registro %+v, modo atteso %s", righe, tc.modo)
			}
			// L'evento dice i permessi: falsi col modo utente.
			var e model.Evento
			ammin := `"amministratore":` + strconv.FormatBool(tc.modo == "amministratore")
			if err := service.DB.First(&e).Error; err != nil || e.Tipo != service.EventoAgenteLasciapassare || !strings.Contains(e.Corpo, ammin) {
				t.Errorf("evento %+v (%v), atteso %s", e, err, ammin)
			}
		})
	}
}

// TestLegaLasciapassareTecnico prova sul router vero che il lasciapassare
// di un tecnico (AgenteId 0) si lega alla connessione come quello di un
// agente: sulla riga di audit_conns il tecnico del lasciapassare, senza
// agente; sul lasciapassare la riga e il conn_id; una riga INFO col tecnico. Un
// secondo nonce valido all'autorizzazione non la rilega. L'apertura verso
// l'integrazione non porta agente.
func TestLegaLasciapassareTecnico(t *testing.T) {
	g, registro := integrazioneDiProva(t, "https://n8n.example/webhook")
	if err := service.DB.AutoMigrate(&model.Lasciapassare{}); err != nil {
		t.Fatal(err)
	}
	no := false
	crea(t, &model.User{Username: "mario", GroupId: 1, IsAdmin: &no, Status: model.COMMON_STATUS_ENABLE})
	for _, nonce := range []string{"nonce-tecnico", "nonce-secondo"} {
		crea(t, &model.Lasciapassare{Nonce: nonce, Pc: "999000111", TecnicoId: 2, Modo: "tecnico", Emesso: 1, Scade: 61, Creato: 1})
	}
	for _, corpo := range []string{
		`{"action":"new","conn_id":7,"id":"999000111","session_id":1,"uuid":"` + uuidSalvato + `","lasciapassare":"nonce-tecnico"}`,
		`{"conn_id":7,"id":"999000111","peer":["111222333","mario"],"session_id":1,"type":0,"uuid":"` + uuidSalvato + `","lasciapassare":"nonce-secondo"}`,
	} {
		if rec := daTecnico(g, "POST", "/api/audit/conn", "", corpo); rec.Code != 200 {
			t.Fatalf("%s: %d %s", corpo, rec.Code, rec.Body)
		}
	}
	var conn model.AuditConn
	if err := service.DB.First(&conn).Error; err != nil {
		t.Fatal(err)
	}
	if conn.TecnicoLasciapassare != "mario" || conn.AgenteLasciapassare != "" {
		t.Errorf("connessione: tecnico %q, agente %q; attesi mario e vuoto", conn.TecnicoLasciapassare, conn.AgenteLasciapassare)
	}
	var lps []model.Lasciapassare
	if err := service.DB.Order("id").Find(&lps).Error; err != nil || len(lps) != 2 {
		t.Fatalf("%d lasciapassare (%v)", len(lps), err)
	}
	if lps[0].AuditConnId != conn.Id || lps[0].ConnId != 7 || lps[1].AuditConnId != 0 {
		t.Errorf("legami: %+v", lps)
	}
	log := registro.String()
	if applog.Conta(log, "INFO", "connessione 7 del PC 999000111 aperta col lasciapassare del tecnico mario") != 1 ||
		applog.Conta(log, "WARN", "lasciapassare") != 0 || applog.Conta(log, "ERROR", "lasciapassare") != 0 {
		t.Errorf("log:\n%s", log)
	}
	var aperta model.Evento
	if err := service.DB.Where("tipo = ?", service.EventoConnessioneAperta).First(&aperta).Error; err != nil || strings.Contains(aperta.Corpo, `"agente"`) {
		t.Errorf("apertura %+v (%v): non deve portare agente", aperta, err)
	}
}
