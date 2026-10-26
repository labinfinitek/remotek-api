package http

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/config"
	"github.com/lejianwen/rustdesk-api/v2/global"
	applog "github.com/lejianwen/rustdesk-api/v2/lib/logger"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// lasciapassareDiProva prepara gli agenti di agentiDiProva, il PC
// 999000111 nella rubrica personale di mario, l'integrazione accesa (senza
// consegna) e, se accesa, la chiave dei lasciapassare in un file 600.
// Restituisce il router, la chiave pubblica (nil se spenta) e il registro
// del log.
func lasciapassareDiProva(t *testing.T, accesa bool) (*gin.Engine, ed25519.PublicKey, *strings.Builder) {
	t.Helper()
	g := agentiDiProva(t)
	registro := &strings.Builder{}
	global.Logger = applog.Su(registro)
	service.Logger = global.Logger
	if err := service.DB.AutoMigrate(&model.Lasciapassare{}, &model.Evento{}); err != nil {
		t.Fatal(err)
	}
	crea(t, &model.Peer{Id: "999000111", Hostname: "PC-ROSSI", Uuid: "dXVpZA"})
	crea(t, &model.AddressBook{Id: "999000111", UserId: 2})
	prec := global.Config.Integrazione
	t.Cleanup(func() { global.Config.Integrazione, global.Config.Agente = prec, config.Agente{} })
	global.Config.Integrazione = config.Integrazione{Url: "https://n8n.example/webhook", Segreto: strings.Repeat("s", 32)}
	if !accesa {
		return g, nil, registro
	}
	p := filepath.Join(t.TempDir(), "lasciapassare.key")
	if _, err := config.ChiaveNuova(p); err != nil {
		t.Fatal(err)
	}
	global.Config.Agente = config.Agente{LasciapassareChiave: p}
	if err := global.Config.Agente.Carica(); err != nil {
		t.Fatal(err)
	}
	pub, _ := base64.StdEncoding.DecodeString(global.Config.Agente.Pubblica())
	return g, pub, registro
}

// TestLasciapassareRifiutato prova sul router vero i rifiuti di POST
// /api/agente/lasciapassare: senza token, agente disattivato (401 di
// RustAuth), funzione spenta, utente che non e' un agente, tecnico
// disattivato, PC sconosciuto, corpo senza id. Nessun rifiuto registra un
// lasciapassare.
func TestLasciapassareRifiutato(t *testing.T) {
	spento := model.COMMON_STATUS_DISABLED
	for _, tc := range []struct {
		caso, token, corpo string
		accesa             bool
		prima              func()
		codice             int
		errore             string
	}{
		{"senza token", "", `{"id":"999000111"}`, true, nil, 401, "Unauthorized"},
		{"agente disattivato", "tok-agente-mario", `{"id":"999000111"}`, true, func() { stato(t, 4, spento) }, 401, "Unauthorized"},
		{"funzione spenta", "tok-agente-mario", `{"id":"999000111"}`, false, nil, 400, "Il lasciapassare dell'agente non è configurato su questo server."}, //nolint:misspell // italiano
		{"non agente", "tok-mario", `{"id":"999000111"}`, true, nil, 400, "Solo un agente AI attivo può chiedere il lasciapassare."},
		{"tecnico disattivato", "tok-agente-mario", `{"id":"999000111"}`, true, func() { stato(t, 2, spento) }, 400, "Il tecnico di questo agente AI non esiste o è disattivato."},
		{"PC sconosciuto", "tok-agente-mario", `{"id":"999000222"}`, true, nil, 400, "Nessun PC con questo ID è noto al server."},
		{"senza id", "tok-agente-mario", `{}`, true, nil, 400, "Parametri non validi."},
	} {
		t.Run(tc.caso, func(t *testing.T) {
			g, _, _ := lasciapassareDiProva(t, tc.accesa)
			if tc.prima != nil {
				tc.prima()
			}
			rec := daTecnico(g, "POST", "/api/agente/lasciapassare", tc.token, tc.corpo)
			want, _ := json.Marshal(map[string]string{"error": tc.errore})
			if rec.Code != tc.codice || rec.Body.String() != string(want) {
				t.Errorf("%d %s, attesi %d %s", rec.Code, rec.Body, tc.codice, want)
			}
			var n int64
			if err := service.DB.Model(&model.Lasciapassare{}).Count(&n).Error; err != nil || n != 0 {
				t.Errorf("%d lasciapassare registrati (%v), attesi 0", n, err)
			}
		})
	}
}

// stato cambia nel database lo stato dell'utente id, senza le regole del
// pannello (che col tecnico disattiverebbero anche i suoi agenti).
func stato(t *testing.T, id uint, s model.StatusCode) {
	t.Helper()
	if err := service.DB.Model(&model.User{}).Where("id = ?", id).Update("status", s).Error; err != nil {
		t.Fatal(err)
	}
}

// TestLasciapassareRilasciato prova sul router vero il rilascio: la firma
// si verifica con la chiave pubblica sul prefisso piu' il payload, che
// dice PC, agente, tecnico e modo, con scade = emesso + 60. Nel database
// c'e' il nonce, non il lasciapassare ne' la firma; nel log una riga INFO
// con agente, tecnico e PC, senza lasciapassare ne' nonce; in coda
// agente.lasciapassare.
func TestLasciapassareRilasciato(t *testing.T) {
	g, pub, registro := lasciapassareDiProva(t, true)
	prima := time.Now().Unix()
	rec := daTecnico(g, "POST", "/api/agente/lasciapassare", "tok-agente-mario", `{"id":"999000111"}`)
	var res struct{ Lasciapassare string }
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &res) != nil {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	payload, firma, _ := strings.Cut(res.Lasciapassare, ".")
	pb, err1 := base64.RawURLEncoding.DecodeString(payload)
	fb, err2 := base64.RawURLEncoding.DecodeString(firma)
	if err1 != nil || err2 != nil || !ed25519.Verify(pub, append([]byte(service.PrefissoLasciapassare), pb...), fb) {
		t.Fatalf("firma non verificata (%v, %v): %s", err1, err2, res.Lasciapassare)
	}
	var p service.PayloadLasciapassare
	if err := json.Unmarshal(pb, &p); err != nil {
		t.Fatal(err)
	}
	nb, err := base64.RawURLEncoding.DecodeString(p.Nonce)
	if p.V != 1 || p.Pc != "999000111" || p.Agente != "agente-mario" || p.Tecnico != "mario" || p.Modo != "amministratore" ||
		p.Emesso < prima || p.Emesso > time.Now().Unix() || p.Scade != p.Emesso+60 || err != nil || len(nb) != 16 {
		t.Errorf("payload %s", pb)
	}
	var righe []model.Lasciapassare
	if err := service.DB.Find(&righe).Error; err != nil {
		t.Fatal(err)
	}
	want := model.Lasciapassare{IdModel: righe[0].IdModel, Nonce: p.Nonce, Pc: "999000111", AgenteId: 4, TecnicoId: 2,
		Modo: "amministratore", Capacita: "terminale", Emesso: p.Emesso, Scade: p.Scade, Creato: p.Emesso}
	if len(righe) != 1 || righe[0] != want {
		t.Errorf("registro %+v, atteso %+v", righe, want)
	}
	var dump string
	if err := service.DB.Raw("SELECT group_concat(nonce || pc || agente_id || tecnico_id || emesso || scade || conn_id || creato) FROM lasciapassare").Scan(&dump).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(dump, payload) || strings.Contains(dump, firma) {
		t.Errorf("nel database c'e' il lasciapassare: %s", dump)
	}
	log := registro.String()
	if applog.Conta(log, "INFO", "lasciapassare rilasciato", "agente-mario", "mario", "999000111") != 1 ||
		strings.Contains(log, p.Nonce) || strings.Contains(log, firma) || strings.Contains(log, payload) {
		t.Errorf("log:\n%s", log)
	}
	var e model.Evento
	if err := service.DB.First(&e).Error; err != nil || e.Tipo != service.EventoAgenteLasciapassare ||
		!strings.Contains(e.Corpo, `"agente":{"agente":"agente-mario","tecnico":"mario","amministratore":true,"capacita":"terminale"}`) || strings.Contains(e.Corpo, p.Nonce) {
		t.Errorf("evento %+v (%v)", e, err)
	}
	// Due richieste danno due nonce diversi.
	rec = daTecnico(g, "POST", "/api/agente/lasciapassare", "tok-agente-mario", `{"id":"999000111"}`)
	var n int64
	if err := service.DB.Model(&model.Lasciapassare{}).Distinct("nonce").Count(&n).Error; rec.Code != 200 || err != nil || n != 2 {
		t.Errorf("secondo lasciapassare: %d, %d nonce distinti (%v)", rec.Code, n, err)
	}
}

// TestLasciapassareRubrica prova sul router vero che il lasciapassare si
// rilascia solo per un PC nella rubrica del tecnico dell'agente (mario, id
// 2): quella personale, una sua rubrica, una rubrica di luigi condivisa con
// lui o con il suo gruppo. Un PC solo nella rubrica di luigi, in una
// rubrica di un agente AI o di chi non c'e' piu', o in nessuna, e'
// rifiutato con un warn su agente, tecnico e PC, e senza registro.
func TestLasciapassareRubrica(t *testing.T) {
	for _, tc := range []struct {
		caso  string
		prima func()
		ok    bool
	}{
		{"rubrica personale", func() { crea(t, &model.AddressBook{Id: "999000111", UserId: 2}) }, true},
		{"sua rubrica", func() {
			crea(t, &model.AddressBookCollection{UserId: 2, Name: "clienti"})
			crea(t, &model.AddressBook{Id: "999000111", UserId: 2, CollectionId: 1})
		}, true},
		{"condivisa con lui", func() {
			crea(t, &model.AddressBookCollection{UserId: 3, Name: "di luigi"})
			crea(t, &model.AddressBook{Id: "999000111", UserId: 3, CollectionId: 1})
			crea(t, &model.AddressBookCollectionRule{UserId: 3, CollectionId: 1, Rule: model.ShareAddressBookRuleRuleRead, Type: model.ShareAddressBookRuleTypePersonal, ToId: 2})
		}, true},
		{"condivisa col suo gruppo", func() {
			crea(t, &model.AddressBookCollection{UserId: 3, Name: "di luigi"})
			crea(t, &model.AddressBook{Id: "999000111", UserId: 3, CollectionId: 1})
			crea(t, &model.AddressBookCollectionRule{UserId: 3, CollectionId: 1, Rule: model.ShareAddressBookRuleRuleRead, Type: model.ShareAddressBookRuleTypeGroup, ToId: 1})
		}, true},
		{"rubrica di un altro tecnico", func() { crea(t, &model.AddressBook{Id: "999000111", UserId: 3}) }, false},
		{"rubrica non condivisa", func() {
			crea(t, &model.AddressBookCollection{UserId: 3, Name: "di luigi"})
			crea(t, &model.AddressBook{Id: "999000111", UserId: 3, CollectionId: 1})
		}, false},
		// Luigi condivide con mario la sua rubrica 1, ma il PC sta altrove:
		// la condivisione vale solo per le voci di quella rubrica.
		{"altra rubrica di chi condivide", func() {
			crea(t, &model.AddressBookCollection{UserId: 3, Name: "condivisa"})
			crea(t, &model.AddressBookCollection{UserId: 3, Name: "privata"})
			crea(t, &model.AddressBook{Id: "999000111", UserId: 3, CollectionId: 2})
			crea(t, &model.AddressBookCollectionRule{UserId: 3, CollectionId: 1, Rule: model.ShareAddressBookRuleRuleRead, Type: model.ShareAddressBookRuleTypePersonal, ToId: 2})
		}, false},
		{"rubrica personale di chi condivide", func() {
			crea(t, &model.AddressBookCollection{UserId: 3, Name: "condivisa"})
			crea(t, &model.AddressBook{Id: "999000111", UserId: 3})
			crea(t, &model.AddressBookCollectionRule{UserId: 3, CollectionId: 1, Rule: model.ShareAddressBookRuleRuleRead, Type: model.ShareAddressBookRuleTypePersonal, ToId: 2})
		}, false},
		// Una rubrica di un agente AI, anche condivisa col tecnico, non conta.
		{"condivisa da un agente", func() {
			crea(t, &model.AddressBookCollection{UserId: 4, Name: "dell'agente"})
			crea(t, &model.AddressBook{Id: "999000111", UserId: 4, CollectionId: 1})
			crea(t, &model.AddressBookCollectionRule{UserId: 4, CollectionId: 1, Rule: model.ShareAddressBookRuleRuleRead, Type: model.ShareAddressBookRuleTypePersonal, ToId: 2})
		}, false},
		{"condivisa da chi non c'e' piu'", func() {
			crea(t, &model.AddressBookCollection{UserId: 99, Name: "orfana"})
			crea(t, &model.AddressBook{Id: "999000111", UserId: 99, CollectionId: 1})
			crea(t, &model.AddressBookCollectionRule{UserId: 99, CollectionId: 1, Rule: model.ShareAddressBookRuleRuleRead, Type: model.ShareAddressBookRuleTypeGroup, ToId: 1})
		}, false},
		{"nessuna rubrica", func() {}, false},
	} {
		t.Run(tc.caso, func(t *testing.T) {
			g, _, registro := lasciapassareDiProva(t, true)
			if err := service.DB.Where("1 = 1").Delete(&model.AddressBook{}).Error; err != nil {
				t.Fatal(err)
			}
			tc.prima()
			rec := daTecnico(g, "POST", "/api/agente/lasciapassare", "tok-agente-mario", `{"id":"999000111"}`)
			var n int64
			if err := service.DB.Model(&model.Lasciapassare{}).Count(&n).Error; err != nil {
				t.Fatal(err)
			}
			// Un rifiuto e' un solo warn, quello della risposta, col contesto e
			// l'ID del messaggio.
			log := registro.String()
			avvisi := applog.Conta(log, "WARN", "PassPcNotInAddressBook", "lasciapassare rifiutato", "999000111", "agente-mario")
			id, motivo := applog.Conta(log, "WARN", "PassPcNotInAddressBook"), applog.Conta(log, "WARN", "lasciapassare rifiutato")
			if tc.ok {
				if rec.Code != 200 || n != 1 || id+motivo != 0 {
					t.Errorf("%d %s, %d registrati, %d+%d warn; attesi 200, 1, 0", rec.Code, rec.Body, n, id, motivo)
				}
				return
			}
			want := `{"error":"Il PC non è nella rubrica del tecnico di questo agente AI."}`
			if rec.Code != 400 || rec.Body.String() != want || n != 0 || avvisi != 1 || id != 1 || motivo != 1 {
				t.Errorf("%d %s, %d registrati, warn %d (%d con l'ID, %d col motivo); attesi 400 %s, 0, 1 (1, 1)\n%s", rec.Code, rec.Body, n, avvisi, id, motivo, want, log)
			}
		})
	}
}
