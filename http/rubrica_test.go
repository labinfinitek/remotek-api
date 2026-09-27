package http

// Test delle letture della rubrica sul router vero, dalle rotte del client:
// un errore del database e' una risposta d'errore, non una rubrica vuota.

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/lib/jwt"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// rubricheDiProva prepara il router vero con due utenti del gruppo 1: quello
// di pannello, con la voce 999000111 e il tag lavoro nella rubrica personale
// e la sua rubrica ufficio, e il proprietario della rubrica condivisa, che
// gliela condivide con la regola regola e ha li' la voce 999000222 e il tag
// condiviso. Da' ai servizi un Jwt senza chiave, che per RustAuth vale il
// token del database. Restituisce il router, il registro del log, una
// funzione che manda una richiesta col token di pannello, come il client, e
// un Replacer che mette nella rotta il guid di PERSONALE e CONDIVISA.
func rubricheDiProva(t *testing.T, regola int) (*gin.Engine, *strings.Builder, func(metodo, rotta, corpo string) *httptest.ResponseRecorder, *strings.Replacer) {
	t.Helper()
	g, utente, registro := pannello(t, false)
	err := service.DB.AutoMigrate(&model.AddressBook{}, &model.AddressBookCollection{}, &model.AddressBookCollectionRule{}, &model.Peer{})
	if err == nil {
		err = service.DB.Model(utente).Update("group_id", 1).Error
	}
	if err != nil {
		t.Fatal(err)
	}
	precJwt := global.Jwt
	global.Jwt = jwt.NewJwt("", time.Hour)
	t.Cleanup(func() { global.Jwt = precJwt })

	proprietario := &model.User{Username: "proprietario", GroupId: 1, Status: model.COMMON_STATUS_ENABLE}
	crea(t, proprietario)
	crea(t, &model.AddressBook{Id: "999000111", UserId: utente.Id})
	crea(t, &model.Tag{Name: "lavoro", UserId: utente.Id})
	crea(t, &model.AddressBookCollection{UserId: utente.Id, Name: "ufficio"})
	condivisa := &model.AddressBookCollection{UserId: proprietario.Id, Name: "condivisa"}
	crea(t, condivisa)
	crea(t, &model.AddressBookCollectionRule{UserId: proprietario.Id, CollectionId: condivisa.Id, Rule: regola,
		Type: model.ShareAddressBookRuleTypePersonal, ToId: utente.Id})
	crea(t, &model.AddressBook{Id: "999000222", UserId: proprietario.Id, CollectionId: condivisa.Id})
	crea(t, &model.Tag{Name: "condiviso", UserId: proprietario.Id, CollectionId: condivisa.Id})

	invia := func(metodo, rotta, corpo string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(metodo, rotta, strings.NewReader(corpo))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenDelPannello)
		rec := httptest.NewRecorder()
		g.ServeHTTP(rec, req)
		return rec
	}
	id := func(n uint) string { return strconv.FormatUint(uint64(n), 10) }
	guid := strings.NewReplacer("PERSONALE", "1-"+id(utente.Id)+"-0", "CONDIVISA", "1-"+id(proprietario.Id)+"-"+id(condivisa.Id))
	return g, registro, invia, guid
}

// TestRubricaLetturaFallita prova sul router vero che le rotte del client che
// leggono voci, tag e rubriche, se il database non le legge, rispondono 400
// {"error": "Errore di sistema."}, la forma d'errore del contratto, e
// scrivono l'errore nel log: il client 1.4.9 mostra pull_ab_failed e tiene
// la rubrica che ha. Prima rispondevano 200 con le liste vuote, che il
// client prende per una rubrica vuota vera e salva nella cache, e che un
// client legacy svuota e rimanda vuota con POST /api/ab. Senza ostacoli le
// stesse richieste rispondono 200 coi dati.
func TestRubricaLetturaFallita(t *testing.T) {
	for _, tc := range []struct {
		nome, metodo, rotta, nelLog string
		tabella, dove               string // la lettura che rifiutaLetture fa fallire
		dato                        string // nella risposta senza ostacoli
	}{
		{"rubrica legacy, voci", "GET", "/api/ab", "GET /api/ab: ", "address_books", "", "999000111"},
		{"rubrica legacy, tag", "GET", "/api/ab", "GET /api/ab: ", "tags", "", "lavoro"},
		{"voci", "POST", "/api/ab/peers?ab=PERSONALE", "POST /api/ab/peers: ", "address_books", "", "999000111"},
		{"tag", "POST", "/api/ab/tags/PERSONALE", "POST /api/ab/tags/:guid: ", "tags", "", "lavoro"},
		{"rubriche proprie", "POST", "/api/ab/shared/profiles", "POST /api/ab/shared/profiles: ", "address_book_collections", "user_id", "ufficio"},
		{"regole di lettura", "POST", "/api/ab/shared/profiles", "POST /api/ab/shared/profiles: ", "address_book_collection_rules", "rule > 0", "condivisa"},
		{"rubriche condivise", "POST", "/api/ab/shared/profiles", "POST /api/ab/shared/profiles: ", "address_book_collections", "id in", "condivisa"},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			_, registro, invia, guid := rubricheDiProva(t, model.ShareAddressBookRuleRuleRead)
			rotta := guid.Replace(tc.rotta)
			if rec := invia(tc.metodo, rotta, ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), tc.dato) {
				t.Fatalf("%s %s senza ostacoli: %d %s, atteso 200 con %s", tc.metodo, rotta, rec.Code, rec.Body, tc.dato)
			}
			rifiutaLetture(t, tc.tabella, tc.dove)
			registro.Reset()
			rec := invia(tc.metodo, rotta, "")
			if got, want := rec.Body.String(), `{"error":"Errore di sistema."}`; rec.Code != 400 || got != want {
				t.Errorf("%s %s con %s non lette: %d %s, attesi 400 e %s", tc.metodo, rotta, tc.tabella, rec.Code, got, want)
			}
			if nelLog := registro.String(); !strings.Contains(nelLog, tc.nelLog) || !strings.Contains(nelLog, "lettura rifiutata dal test") {
				t.Errorf("%s %s, nel log mancano rotta o errore:\n%s", tc.metodo, rotta, nelLog)
			}
		})
	}
}

// TestPannelloRubricaLetturaFallita prova sul router vero che gli elenchi
// della rubrica del pannello (voci, rubriche e tag, dell'amministrazione e
// della sezione dell'utente) e il cambio dei tag di piu' voci, se il
// database non legge, rispondono code 101 "Errore di sistema." e scrivono
// l'errore nel log. Prima gli elenchi rispondevano successo con l'elenco
// vuoto, e il cambio dei tag "Elemento non trovato.".
func TestPannelloRubricaLetturaFallita(t *testing.T) {
	for _, tc := range []struct{ metodo, rotta, corpo, tabella string }{
		{"GET", "/api/admin/address_book/list?user_id=1", "", "address_books"},
		{"GET", "/api/admin/my/address_book/list", "", "address_books"},
		{"POST", "/api/admin/my/address_book/batchUpdateTags", `{"row_ids":[1],"tags":["casa"]}`, "address_books"},
		{"GET", "/api/admin/address_book_collection/list?user_id=1", "", "address_book_collections"},
		{"GET", "/api/admin/my/address_book_collection/list", "", "address_book_collections"},
		{"GET", "/api/admin/tag/list?user_id=1", "", "tags"},
		{"GET", "/api/admin/my/tag/list", "", "tags"},
	} {
		t.Run(tc.rotta, func(t *testing.T) {
			g, utente, registro := pannello(t, true)
			if err := service.DB.AutoMigrate(&model.AddressBook{}, &model.AddressBookCollection{}); err != nil {
				t.Fatal(err)
			}
			crea(t, &model.AddressBook{Id: "999000111", UserId: utente.Id})
			crea(t, &model.AddressBookCollection{UserId: utente.Id, Name: "ufficio"})
			crea(t, &model.Tag{Name: "lavoro", UserId: utente.Id})
			rifiutaLetture(t, tc.tabella, "")

			rec := richiesta(g, tc.metodo, tc.rotta, "", tc.corpo)
			if got, want := rec.Body.String(), `{"code":101,"message":"Errore di sistema.","data":null}`; rec.Code != 200 || got != want {
				t.Errorf("%s %s con %s non lette: stato %d\n got  %s\n want %s", tc.metodo, tc.rotta, tc.tabella, rec.Code, got, want)
			}
			percorso, _, _ := strings.Cut(tc.rotta, "?")
			if nelLog := registro.String(); !strings.Contains(nelLog, tc.metodo+" "+percorso+": ") || !strings.Contains(nelLog, "lettura rifiutata dal test") {
				t.Errorf("%s %s, nel log mancano rotta o errore:\n%s", tc.metodo, tc.rotta, nelLog)
			}
		})
	}
}
