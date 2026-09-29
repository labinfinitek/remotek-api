package http

// Test delle letture della rubrica sul router vero, dalle rotte del client:
// un errore del database e' una risposta d'errore, non una rubrica vuota.

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/lib/jwt"
	"github.com/lejianwen/rustdesk-api/v2/lib/logger"
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
// un Replacer che mette nella rotta il guid di PERSONALE, UFFICIO,
// CONDIVISA e SCONOSCIUTA, una rubrica dell'utente che non c'e'.
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
	ufficio := &model.AddressBookCollection{UserId: utente.Id, Name: "ufficio"}
	crea(t, ufficio)
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
	guid := strings.NewReplacer("PERSONALE", "1-"+id(utente.Id)+"-0", "UFFICIO", "1-"+id(utente.Id)+"-"+id(ufficio.Id),
		"CONDIVISA", "1-"+id(proprietario.Id)+"-"+id(condivisa.Id), "SCONOSCIUTA", "1-"+id(utente.Id)+"-999999")
	return g, registro, invia, guid
}

// TestRubricaLetturaFallita prova sul router vero che le rotte del client che
// leggono voci, tag e rubriche, se il database non le legge, rispondono 400
// {"error": "Errore di sistema."}, la forma d'errore del contratto, e
// scrivono l'errore nel log: il client 1.4.9 mostra pull_ab_failed e non
// salva la cache. Prima rispondevano 200 con le liste vuote, che il
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

// TestRubricheCondiviseProprietarioTolto prova sul router vero che
// POST /api/ab/shared/profiles, se una rubrica condivisa non ha piu' il suo
// proprietario (dati rimasti orfani, tolto con una Exec e non con
// UserService.Delete, che cancellerebbe anche la regola), salta quella
// rubrica con una riga di warn nel log e risponde 200 con le altre. Prima
// andava in panic e rispondeva 500: il client 1.4.9 toglieva dall'elenco
// tutte le rubriche condivise.
func TestRubricheCondiviseProprietarioTolto(t *testing.T) {
	_, registro, invia, _ := rubricheDiProva(t, model.ShareAddressBookRuleRuleRead)
	var utente, orfana model.AddressBookCollection
	err := service.DB.Raw("SELECT * FROM address_book_collections WHERE name = 'ufficio'").Scan(&utente).Error
	if err == nil {
		err = service.DB.Raw("SELECT * FROM address_book_collections WHERE name = 'condivisa'").Scan(&orfana).Error
	}
	if err != nil {
		t.Fatal(err)
	}
	secondo := &model.User{Username: "secondo", GroupId: 1, Status: model.COMMON_STATUS_ENABLE}
	crea(t, secondo)
	altra := &model.AddressBookCollection{UserId: secondo.Id, Name: "altra"}
	crea(t, altra)
	crea(t, &model.AddressBookCollectionRule{UserId: secondo.Id, CollectionId: altra.Id, Rule: model.ShareAddressBookRuleRuleRead,
		Type: model.ShareAddressBookRuleTypePersonal, ToId: utente.UserId})
	if err := service.DB.Exec("DELETE FROM users WHERE id = ?", orfana.UserId).Error; err != nil {
		t.Fatal(err)
	}
	registro.Reset()

	rec := invia("POST", "/api/ab/shared/profiles", "")
	var corpo struct {
		Total *int `json:"total"`
		Data  []struct {
			Guid, Name, Owner string
			Rule              int
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &corpo); rec.Code != 200 || err != nil || corpo.Total == nil {
		t.Fatalf("POST /api/ab/shared/profiles con una rubrica orfana: %d %s, atteso 200 con total e data", rec.Code, rec.Body)
	}
	var nomi []string
	for _, r := range corpo.Data {
		nomi = append(nomi, r.Name+" "+r.Owner)
	}
	if got, want := strings.Join(nomi, ", "), "ufficio prova, altra secondo"; got != want {
		t.Errorf("rubriche nella risposta: %q, attese %q", got, want)
	}
	riga := fmt.Sprintf("POST /api/ab/shared/profiles: rubrica %d saltata, il proprietario %d non c'e'", orfana.Id, orfana.UserId)
	if nelLog := registro.String(); logger.Conta(nelLog, "WARN", riga) != 1 {
		t.Errorf("nel log manca il warn %q:\n%s", riga, nelLog)
	}
}

// TestPannelloRubricaLetturaFallita prova sul router vero che gli elenchi
// della rubrica del pannello (voci, rubriche, regole di condivisione e tag,
// dell'amministrazione e della sezione dell'utente) e il cambio dei tag di
// piu' voci, se il
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
		{"GET", "/api/admin/address_book_collection_rule/list?user_id=1", "", "address_book_collection_rules"},
		{"GET", "/api/admin/my/address_book_collection_rule/list", "", "address_book_collection_rules"},
		{"GET", "/api/admin/tag/list?user_id=1", "", "tags"},
		{"GET", "/api/admin/my/tag/list", "", "tags"},
	} {
		t.Run(tc.rotta, func(t *testing.T) {
			g, utente, registro := pannello(t, true)
			if err := service.DB.AutoMigrate(&model.AddressBook{}, &model.AddressBookCollection{}, &model.AddressBookCollectionRule{}); err != nil {
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

// statoRubrica descrive con Raw, che rifiutaLetture non ferma, le voci e i
// tag di tutte le rubriche.
func statoRubrica(t *testing.T) string {
	t.Helper()
	var voci, tag []string
	err := service.DB.Raw("SELECT id || ' ' || alias || ' ' || collection_id FROM address_books ORDER BY row_id").Scan(&voci).Error
	if err == nil {
		err = service.DB.Raw("SELECT name || ' ' || color || ' ' || collection_id FROM tags ORDER BY id").Scan(&tag).Error
	}
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprint("voci ", voci, ", tag ", tag)
}

// TestRubricaPermessoNonLetto prova sul router vero le rotte del client su
// una rubrica condivisa quando la regola dei permessi non si legge:
// rispondono 400 {"error": "Errore di sistema."}, scrivono l'errore nel log
// e non cambiano niente. Prima rispondevano "Non hai i permessi per questa
// operazione.", come a chi il permesso non ce l'ha, e l'errore si perdeva;
// se non si leggeva solo la regola del gruppo valeva quella dell'utente.
// Senza ostacoli la regola di controllo completo lascia leggere, scrivere e
// cancellare.
func TestRubricaPermessoNonLetto(t *testing.T) {
	// prova prepara le rubriche con la regola regola per l'utente e, se non
	// e' il controllo completo, quella di controllo completo per il suo
	// gruppo; se dove non e' vuoto rifiuta le letture delle regole che lo
	// contengono. Senza ostacoli la richiesta deve riuscire; con, fallire
	// senza cambiare niente e con regolaNelLog nel log.
	prova := func(t *testing.T, regola int, dove, regolaNelLog, metodo, rotta, corpo, dato string) {
		t.Helper()
		_, registro, invia, guid := rubricheDiProva(t, regola)
		if regola != model.ShareAddressBookRuleRuleFullControl {
			condivisa := &model.AddressBookCollection{}
			if err := service.DB.Raw("SELECT * FROM address_book_collections WHERE name = 'condivisa'").Scan(condivisa).Error; err != nil {
				t.Fatal(err)
			}
			crea(t, &model.AddressBookCollectionRule{UserId: condivisa.UserId, CollectionId: condivisa.Id, Rule: model.ShareAddressBookRuleRuleFullControl,
				Type: model.ShareAddressBookRuleTypeGroup, ToId: 1})
		}
		prima := statoRubrica(t)
		if dove == "" {
			rec := invia(metodo, guid.Replace(rotta), corpo)
			if rec.Code != 200 || !strings.Contains(rec.Body.String(), dato) {
				t.Errorf("%s %s: %d %s, atteso 200 con %q", metodo, rotta, rec.Code, rec.Body, dato)
			}
			if dopo := statoRubrica(t); dato == "" && dopo == prima {
				t.Errorf("%s %s non ha cambiato la rubrica: %s", metodo, rotta, dopo)
			}
			return
		}
		rifiutaLetture(t, "address_book_collection_rules", dove)
		registro.Reset()
		rec := invia(metodo, guid.Replace(rotta), corpo)
		if got, want := rec.Body.String(), `{"error":"Errore di sistema."}`; rec.Code != 400 || got != want {
			t.Errorf("%s %s con la regola non letta: %d %s, attesi 400 e %s", metodo, rotta, rec.Code, got, want)
		}
		percorso, _, _ := strings.Cut(strings.Replace(rotta, "CONDIVISA", ":guid", 1), "?")
		if nelLog := registro.String(); !strings.Contains(nelLog, metodo+" "+percorso+": ") || !strings.Contains(nelLog, regolaNelLog) ||
			!strings.Contains(nelLog, "lettura rifiutata dal test") {
			t.Errorf("%s %s, nel log mancano rotta, regola o errore:\n%s", metodo, rotta, nelLog)
		}
		if dopo := statoRubrica(t); dopo != prima {
			t.Errorf("%s %s con la regola non letta ha cambiato la rubrica:\n prima %s\n dopo  %s", metodo, rotta, prima, dopo)
		}
	}
	const completo = model.ShareAddressBookRuleRuleFullControl
	for _, tc := range []struct {
		metodo, rotta, corpo string
		dato                 string // nella risposta senza ostacoli; vuoto se la rotta scrive
	}{
		{"POST", "/api/ab/peers?ab=CONDIVISA", "", "999000222"},
		{"POST", "/api/ab/tags/CONDIVISA", "", "condiviso"},
		{"POST", "/api/ab/peer/add/CONDIVISA", `{"id":"999000333"}`, ""},
		{"PUT", "/api/ab/peer/update/CONDIVISA", `{"id":"999000222","alias":"nuovo"}`, ""},
		{"DELETE", "/api/ab/peer/CONDIVISA", `["999000222"]`, ""},
		{"POST", "/api/ab/tag/add/CONDIVISA", `{"name":"nuovo","color":1}`, ""},
		{"PUT", "/api/ab/tag/rename/CONDIVISA", `{"old":"condiviso","new":"nuovo"}`, ""},
		{"PUT", "/api/ab/tag/update/CONDIVISA", `{"name":"condiviso","color":1}`, ""},
		{"DELETE", "/api/ab/tag/CONDIVISA", `["condiviso"]`, ""},
	} {
		t.Run(tc.metodo+" "+tc.rotta, func(t *testing.T) {
			t.Run("senza ostacoli", func(t *testing.T) {
				prova(t, completo, "", "", tc.metodo, tc.rotta, tc.corpo, tc.dato)
			})
			t.Run("regole non lette", func(t *testing.T) {
				prova(t, completo, "collection_id = ? and to_id", "regola dell'utente", tc.metodo, tc.rotta, tc.corpo, tc.dato)
			})
		})
	}
	// La regola dell'utente da' la lettura, quella del gruppo il controllo
	// completo: senza la seconda la cancellazione non si decide.
	const lettura, cancella, voce = model.ShareAddressBookRuleRuleRead, "/api/ab/peer/CONDIVISA", `["999000222"]`
	t.Run("regola del gruppo, senza ostacoli", func(t *testing.T) {
		prova(t, lettura, "", "", "DELETE", cancella, voce, "")
	})
	t.Run("regola del gruppo non letta", func(t *testing.T) {
		prova(t, lettura, "to_id = ? [2 ", "regola del gruppo", "DELETE", cancella, voce, "")
	})
}

// TestRubricaLegacySoloPersonale prova che POST /api/ab, la rubrica legacy,
// cambia solo la rubrica personale (collezione 0), la sola che GET /api/ab
// manda: la voce e il tag della rubrica ufficio dell'utente, che la
// richiesta non contiene, restano, e una voce che nella richiesta dice di
// stare in ufficio va nella personale. Nella personale la voce e il tag che
// la richiesta non contiene se ne vanno, come prima. Prima si cancellavano
// anche quelli di ufficio, che il client legacy non ha mai visto, e la voce
// finiva in ufficio.
func TestRubricaLegacySoloPersonale(t *testing.T) {
	_, _, invia, _ := rubricheDiProva(t, model.ShareAddressBookRuleRuleRead)
	ufficio := &model.AddressBookCollection{}
	if err := service.DB.Where("name = ?", "ufficio").First(ufficio).Error; err != nil {
		t.Fatal(err)
	}
	crea(t, &model.AddressBook{Id: "999000444", UserId: ufficio.UserId, CollectionId: ufficio.Id})
	crea(t, &model.Tag{Name: "riunioni", UserId: ufficio.UserId, CollectionId: ufficio.Id})
	dati, err := json.Marshal(map[string]any{
		"peers":      []map[string]any{{"id": "999000333", "collection_id": ufficio.Id}},
		"tag_colors": `{"nuovo":1}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	corpo, err := json.Marshal(map[string]string{"data": string(dati)})
	if err != nil {
		t.Fatal(err)
	}

	if rec := invia("POST", "/api/ab", string(corpo)); rec.Code != 200 || rec.Body.String() != "null" {
		t.Fatalf("POST /api/ab: %d %s, attesi 200 null", rec.Code, rec.Body)
	}
	for _, tc := range []struct {
		cosa, tabella, dove string
		attese              int64
	}{
		{"voce di ufficio", "address_books", "id = '999000444' AND collection_id = " + strconv.FormatUint(uint64(ufficio.Id), 10), 1},
		{"tag di ufficio", "tags", "name = 'riunioni' AND collection_id = " + strconv.FormatUint(uint64(ufficio.Id), 10), 1},
		{"voce nuova, nella personale", "address_books", "id = '999000333' AND collection_id = 0", 1},
		{"voce nuova, in tutto", "address_books", "id = '999000333'", 1},
		{"tag nuovo, nella personale", "tags", "name = 'nuovo' AND collection_id = 0", 1},
		{"voce personale non inviata", "address_books", "id = '999000111'", 0},
		{"tag personale non inviato", "tags", "name = 'lavoro'", 0},
		{"voce del proprietario della condivisa", "address_books", "id = '999000222'", 1},
	} {
		var n int64
		if err := service.DB.Raw("SELECT count(*) FROM " + tc.tabella + " WHERE " + tc.dove).Scan(&n).Error; err != nil {
			t.Fatal(err)
		}
		if n != tc.attese {
			t.Errorf("%s dopo POST /api/ab: %d righe, attese %d (%s)", tc.cosa, n, tc.attese, statoRubrica(t))
		}
	}
}

// TestRubricaTagNonLetto prova sul router vero le rotte del client che
// cercano un tag per nome: se il database non lo legge rispondono 400
// {"error": "Errore di sistema."}, scrivono l'errore nel log e non cambiano
// niente. Prima una lettura fallita valeva "il tag non c'e'": l'aggiunta
// creava un tag doppio, la rinomina rinominava su un nome gia' usato, le
// altre rispondevano "Elemento non trovato.". Un tag che c'e' o non c'e'
// davvero ha le risposte di prima.
func TestRubricaTagNonLetto(t *testing.T) {
	const erroreDiSistema = `{"error":"Errore di sistema."}`
	for _, tc := range []struct {
		nome, metodo, rotta, corpo string
		dove                       string // il nome del tag che non si legge; vuoto, nessuno
		risposta                   string
	}{
		{"aggiunta, tag non letto", "POST", "/api/ab/tag/add/PERSONALE", `{"name":"lavoro","color":1}`, "lavoro", erroreDiSistema},
		{"rinomina, tag non letto", "PUT", "/api/ab/tag/rename/PERSONALE", `{"old":"lavoro","new":"nuovo"}`, "lavoro", erroreDiSistema},
		{"rinomina, nome nuovo non letto", "PUT", "/api/ab/tag/rename/PERSONALE", `{"old":"lavoro","new":"casa"}`, "casa", erroreDiSistema},
		{"colore, tag non letto", "PUT", "/api/ab/tag/update/PERSONALE", `{"name":"lavoro","color":5}`, "lavoro", erroreDiSistema},
		{"cancellazione, tag non letto", "DELETE", "/api/ab/tag/PERSONALE", `["lavoro"]`, "lavoro", erroreDiSistema},
		{"aggiunta, tag che c'e'", "POST", "/api/ab/tag/add/PERSONALE", `{"name":"lavoro","color":1}`, "", `{"error":"L'elemento esiste già."}`},
		{"rinomina, tag che non c'e'", "PUT", "/api/ab/tag/rename/PERSONALE", `{"old":"sconosciuto","new":"nuovo"}`, "", `{"error":"Elemento non trovato."}`},
		{"rinomina, nome nuovo che c'e'", "PUT", "/api/ab/tag/rename/PERSONALE", `{"old":"lavoro","new":"casa"}`, "", `{"error":"L'elemento esiste già."}`},
		{"colore, tag che non c'e'", "PUT", "/api/ab/tag/update/PERSONALE", `{"name":"sconosciuto","color":5}`, "", `{"error":"Elemento non trovato."}`},
		{"cancellazione, tag che non c'e'", "DELETE", "/api/ab/tag/PERSONALE", `["sconosciuto"]`, "", `{"error":"Elemento non trovato."}`},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			_, registro, invia, guid := rubricheDiProva(t, model.ShareAddressBookRuleRuleRead)
			prova := &model.User{}
			if err := service.DB.Where("username = ?", "prova").First(prova).Error; err != nil {
				t.Fatal(err)
			}
			crea(t, &model.Tag{Name: "casa", UserId: prova.Id})
			prima := statoRubrica(t)
			if tc.dove != "" {
				rifiutaLetture(t, "tags", tc.dove)
			}
			registro.Reset()
			rec := invia(tc.metodo, guid.Replace(tc.rotta), tc.corpo)
			if rec.Code != 400 || rec.Body.String() != tc.risposta {
				t.Errorf("%s %s: %d %s, attesi 400 e %s", tc.metodo, tc.rotta, rec.Code, rec.Body, tc.risposta)
			}
			if dopo := statoRubrica(t); dopo != prima {
				t.Errorf("%s %s ha cambiato la rubrica:\n prima %s\n dopo  %s", tc.metodo, tc.rotta, prima, dopo)
			}
			percorso := strings.Replace(tc.rotta, "PERSONALE", ":guid", 1)
			if nelLog := registro.String(); tc.dove != "" && (!strings.Contains(nelLog, tc.metodo+" "+percorso+": ") || !strings.Contains(nelLog, "lettura rifiutata dal test")) {
				t.Errorf("%s %s, nel log mancano rotta o errore:\n%s", tc.metodo, tc.rotta, nelLog)
			}
		})
	}
}

// TestRubricaVoceNonLetta prova sul router vero la cancellazione e la
// modifica di una voce della rubrica dal client: se il database non legge
// la voce rispondono 400 {"error": "Errore di sistema."}, scrivono l'errore
// nel log e non cambiano niente; prima rispondevano "Elemento non trovato.".
// Una voce che non c'e' ha la risposta di prima.
func TestRubricaVoceNonLetta(t *testing.T) {
	for _, tc := range []struct {
		metodo, rotta, corpo string
		rifiuta              bool // le letture delle voci non riescono
		risposta             string
	}{
		{"DELETE", "/api/ab/peer/PERSONALE", `["999000111"]`, true, `{"error":"Errore di sistema."}`},
		{"PUT", "/api/ab/peer/update/PERSONALE", `{"id":"999000111","alias":"nuovo"}`, true, `{"error":"Errore di sistema."}`},
		{"DELETE", "/api/ab/peer/PERSONALE", `["999000999"]`, false, `{"error":"Elemento non trovato."}`},
		{"PUT", "/api/ab/peer/update/PERSONALE", `{"id":"999000999","alias":"nuovo"}`, false, `{"error":"Elemento non trovato."}`},
	} {
		caso := ", voce che non c'e'"
		if tc.rifiuta {
			caso = ", voce non letta"
		}
		t.Run(tc.metodo+" "+tc.rotta+caso, func(t *testing.T) {
			_, registro, invia, guid := rubricheDiProva(t, model.ShareAddressBookRuleRuleRead)
			prima := statoRubrica(t)
			if tc.rifiuta {
				rifiutaLetture(t, "address_books", "")
			}
			registro.Reset()
			rec := invia(tc.metodo, guid.Replace(tc.rotta), tc.corpo)
			if rec.Code != 400 || rec.Body.String() != tc.risposta {
				t.Errorf("%s %s: %d %s, attesi 400 e %s", tc.metodo, tc.rotta, rec.Code, rec.Body, tc.risposta)
			}
			if dopo := statoRubrica(t); dopo != prima {
				t.Errorf("%s %s ha cambiato la rubrica:\n prima %s\n dopo  %s", tc.metodo, tc.rotta, prima, dopo)
			}
			percorso := strings.Replace(tc.rotta, "PERSONALE", ":guid", 1)
			if nelLog := registro.String(); tc.rifiuta && (!strings.Contains(nelLog, tc.metodo+" "+percorso+": ") || !strings.Contains(nelLog, "lettura rifiutata dal test")) {
				t.Errorf("%s %s, nel log mancano rotta o errore:\n%s", tc.metodo, tc.rotta, nelLog)
			}
		})
	}
}

// TestPannelloVoceNonLetta prova sul router vero le rotte del pannello che,
// prima di creare una voce della rubrica, guardano se c'e' gia': se il
// database non la legge rispondono code 101 "Errore di sistema.", con
// l'errore nel log, e non creano niente. Prima la lettura fallita valeva
// "la voce non c'e'" e nasceva un doppione. Una voce che c'e' davvero ha la
// risposta di prima.
func TestPannelloVoceNonLetta(t *testing.T) {
	const erroreDiSistema = `{"code":101,"message":"Errore di sistema.","data":null}`
	for _, tc := range []struct {
		rotta, corpo string
		rifiuta      bool // le letture delle voci non riescono
		risposta     string
	}{
		{"/api/admin/address_book/create", `{"id":"999000111","user_id":UTENTE}`, true, erroreDiSistema},
		{"/api/admin/my/address_book/create", `{"id":"999000111"}`, true, erroreDiSistema},
		{"/api/admin/address_book/batchCreate", `{"id":"999000111","user_ids":[UTENTE]}`, true, erroreDiSistema},
		{"/api/admin/address_book/batchCreateFromPeers", `{"peer_ids":[DISPOSITIVO],"user_id":UTENTE}`, true, erroreDiSistema},
		{"/api/admin/my/address_book/batchCreateFromPeers", `{"peer_ids":[DISPOSITIVO]}`, true, erroreDiSistema},
		{"/api/admin/address_book/create", `{"id":"999000111","user_id":UTENTE}`, false, `{"code":101,"message":"L'elemento esiste già.","data":null}`},
		{"/api/admin/my/address_book/create", `{"id":"999000111"}`, false, `{"code":101,"message":"L'elemento esiste già.","data":null}`},
		{"/api/admin/address_book/batchCreate", `{"id":"999000111","user_ids":[UTENTE]}`, false, `{"code":0,"message":"success","data":null}`},
		{"/api/admin/my/address_book/batchCreateFromPeers", `{"peer_ids":[DISPOSITIVO]}`, false, `{"code":0,"message":"success","data":null}`},
	} {
		caso := ", voce che c'e'"
		if tc.rifiuta {
			caso = ", voce non letta"
		}
		t.Run(tc.rotta+caso, func(t *testing.T) {
			g, utente, registro := pannello(t, true)
			if err := service.DB.AutoMigrate(&model.AddressBook{}, &model.Peer{}); err != nil {
				t.Fatal(err)
			}
			dispositivo := &model.Peer{Id: "999000111", UserId: utente.Id}
			crea(t, dispositivo)
			crea(t, &model.AddressBook{Id: "999000111", UserId: utente.Id})
			if tc.rifiuta {
				rifiutaLetture(t, "address_books", "")
			}
			id := func(n uint) string { return strconv.FormatUint(uint64(n), 10) }
			rec := alPannello(g, tc.rotta, strings.NewReplacer("UTENTE", id(utente.Id), "DISPOSITIVO", id(dispositivo.RowId)).Replace(tc.corpo))
			if got := rec.Body.String(); rec.Code != 200 || got != tc.risposta {
				t.Errorf("POST %s: stato %d\n got  %s\n want %s", tc.rotta, rec.Code, got, tc.risposta)
			}
			if n := righe(t, "address_books"); n != 1 {
				t.Errorf("voci dopo POST %s: %d, attesa 1", tc.rotta, n)
			}
			if nelLog := registro.String(); tc.rifiuta && (!strings.Contains(nelLog, "POST "+tc.rotta+": ") || !strings.Contains(nelLog, "lettura rifiutata dal test")) {
				t.Errorf("POST %s, nel log mancano rotta o errore:\n%s", tc.rotta, nelLog)
			}
		})
	}
}

// TestRubricaCollezioneNonLetta prova sul router vero le rotte del client
// su una rubrica diversa dalla personale, che CheckGuid legge per vedere se
// c'e' ed e' dell'utente del guid: se il database non la legge rispondono
// 400 {"error": "Errore di sistema."}, scrivono l'errore nel log e non
// cambiano niente. Prima rispondevano "Parametri non validi.", come a un
// guid sbagliato. Una rubrica che non c'e' ha la risposta di prima.
func TestRubricaCollezioneNonLetta(t *testing.T) {
	for _, tc := range []struct {
		metodo, rotta, corpo string
		rifiuta              bool // le letture delle rubriche non riescono
		risposta             string
	}{
		{"POST", "/api/ab/peers?ab=UFFICIO", "", true, `{"error":"Errore di sistema."}`},
		{"POST", "/api/ab/peer/add/UFFICIO", `{"id":"999000333"}`, true, `{"error":"Errore di sistema."}`},
		{"POST", "/api/ab/peers?ab=SCONOSCIUTA", "", false, `{"error":"Parametri non validi."}`},
		{"POST", "/api/ab/peer/add/SCONOSCIUTA", `{"id":"999000333"}`, false, `{"error":"Parametri non validi."}`},
	} {
		caso := ", rubrica che non c'e'"
		if tc.rifiuta {
			caso = ", rubrica non letta"
		}
		t.Run(tc.metodo+" "+tc.rotta+caso, func(t *testing.T) {
			_, registro, invia, guid := rubricheDiProva(t, model.ShareAddressBookRuleRuleRead)
			prima := statoRubrica(t)
			if tc.rifiuta {
				rifiutaLetture(t, "address_book_collections", "")
			}
			registro.Reset()
			rec := invia(tc.metodo, guid.Replace(tc.rotta), tc.corpo)
			if rec.Code != 400 || rec.Body.String() != tc.risposta {
				t.Errorf("%s %s: %d %s, attesi 400 e %s", tc.metodo, tc.rotta, rec.Code, rec.Body, tc.risposta)
			}
			if dopo := statoRubrica(t); dopo != prima {
				t.Errorf("%s %s ha cambiato la rubrica:\n prima %s\n dopo  %s", tc.metodo, tc.rotta, prima, dopo)
			}
			percorso, _, _ := strings.Cut(strings.Replace(tc.rotta, "UFFICIO", ":guid", 1), "?")
			if nelLog := registro.String(); tc.rifiuta && (!strings.Contains(nelLog, tc.metodo+" "+percorso+": ") || !strings.Contains(nelLog, "lettura rifiutata dal test")) {
				t.Errorf("%s %s, nel log mancano rotta o errore:\n%s", tc.metodo, tc.rotta, nelLog)
			}
		})
	}
}

// TestPannelloCollezioneNonLetta prova sul router vero le rotte del pannello
// che leggono una rubrica per id: se il database non la legge rispondono
// code 101 "Errore di sistema.", con l'errore nel log, e non cambiano
// niente. Prima rispondevano "Elemento non trovato.". Una rubrica che non
// c'e' ha la risposta di prima.
func TestPannelloCollezioneNonLetta(t *testing.T) {
	const erroreDiSistema = `{"code":101,"message":"Errore di sistema.","data":null}`
	const nonTrovato = `{"code":101,"message":"Elemento non trovato.","data":null}`
	for _, tc := range []struct {
		metodo, rotta, corpo string
		rifiuta              bool // le letture delle rubriche non riescono
		risposta             string
	}{
		{"GET", "/api/admin/address_book_collection/detail/RUBRICA", "", true, erroreDiSistema},
		{"POST", "/api/admin/address_book_collection/delete", `{"id":RUBRICA}`, true, erroreDiSistema},
		{"POST", "/api/admin/my/address_book_collection/update", `{"id":RUBRICA,"name":"nuovo"}`, true, erroreDiSistema},
		{"POST", "/api/admin/my/address_book_collection/delete", `{"id":RUBRICA}`, true, erroreDiSistema},
		{"POST", "/api/admin/address_book/batchCreateFromPeers", `{"peer_ids":[DISPOSITIVO],"user_id":UTENTE,"collection_id":RUBRICA}`, true, erroreDiSistema},
		{"POST", "/api/admin/my/address_book/batchCreateFromPeers", `{"peer_ids":[DISPOSITIVO],"collection_id":RUBRICA}`, true, erroreDiSistema},
		{"GET", "/api/admin/address_book_collection/detail/999999", "", false, nonTrovato},
		{"POST", "/api/admin/address_book_collection/delete", `{"id":999999}`, false, nonTrovato},
		{"POST", "/api/admin/my/address_book_collection/update", `{"id":999999,"name":"nuovo"}`, false, nonTrovato},
		{"POST", "/api/admin/my/address_book/batchCreateFromPeers", `{"peer_ids":[DISPOSITIVO],"collection_id":999999}`, false, nonTrovato},
	} {
		caso := ", rubrica che non c'e'"
		if tc.rifiuta {
			caso = ", rubrica non letta"
		}
		t.Run(tc.metodo+" "+tc.rotta+caso, func(t *testing.T) {
			g, utente, registro := pannello(t, true)
			if err := service.DB.AutoMigrate(&model.AddressBook{}, &model.AddressBookCollection{}, &model.AddressBookCollectionRule{}, &model.Peer{}); err != nil {
				t.Fatal(err)
			}
			rubrica := &model.AddressBookCollection{UserId: utente.Id, Name: "ufficio"}
			crea(t, rubrica)
			dispositivo := &model.Peer{Id: "999000111", UserId: utente.Id}
			crea(t, dispositivo)
			prima := statoRubrica(t) + fmt.Sprint(" rubriche ", righe(t, "address_book_collections"))
			if tc.rifiuta {
				rifiutaLetture(t, "address_book_collections", "")
			}
			id := func(n uint) string { return strconv.FormatUint(uint64(n), 10) }
			sostituisci := strings.NewReplacer("RUBRICA", id(rubrica.Id), "UTENTE", id(utente.Id), "DISPOSITIVO", id(dispositivo.RowId))
			rec := richiesta(g, tc.metodo, sostituisci.Replace(tc.rotta), "", sostituisci.Replace(tc.corpo))
			if got := rec.Body.String(); rec.Code != 200 || got != tc.risposta {
				t.Errorf("%s %s: stato %d\n got  %s\n want %s", tc.metodo, tc.rotta, rec.Code, got, tc.risposta)
			}
			if dopo := statoRubrica(t) + fmt.Sprint(" rubriche ", righe(t, "address_book_collections")); dopo != prima {
				t.Errorf("%s %s ha cambiato la rubrica:\n prima %s\n dopo  %s", tc.metodo, tc.rotta, prima, dopo)
			}
			percorso := strings.Replace(tc.rotta, "/RUBRICA", "/:id", 1)
			if nelLog := registro.String(); tc.rifiuta && (!strings.Contains(nelLog, tc.metodo+" "+percorso+": ") || !strings.Contains(nelLog, "lettura rifiutata dal test")) {
				t.Errorf("%s %s, nel log mancano rotta o errore:\n%s", tc.metodo, tc.rotta, nelLog)
			}
		})
	}
}

// pannelloDiProva prepara il router vero del pannello con l'amministratore di
// pannello: la sua rubrica ufficio, condivisa in lettura con l'utente amico
// dalla regola REGOLA, e nella rubrica personale la voce 999000111 e il tag
// lavoro; amico ha la rubrica altrui. Restituisce il router, il registro del
// log e un Replacer che mette gli id di UTENTE, AMICO, RUBRICA (ufficio),
// ALTRUI, REGOLA, VOCE e TAG.
func pannelloDiProva(t *testing.T) (*gin.Engine, *strings.Builder, *strings.Replacer) {
	t.Helper()
	g, utente, registro := pannello(t, true)
	if err := service.DB.AutoMigrate(&model.AddressBook{}, &model.AddressBookCollection{}, &model.AddressBookCollectionRule{}); err != nil {
		t.Fatal(err)
	}
	amico := &model.User{Username: "amico", Status: model.COMMON_STATUS_ENABLE}
	crea(t, amico)
	ufficio := &model.AddressBookCollection{UserId: utente.Id, Name: "ufficio"}
	crea(t, ufficio)
	altrui := &model.AddressBookCollection{UserId: amico.Id, Name: "altrui"}
	crea(t, altrui)
	regola := &model.AddressBookCollectionRule{UserId: utente.Id, CollectionId: ufficio.Id, Rule: model.ShareAddressBookRuleRuleRead,
		Type: model.ShareAddressBookRuleTypePersonal, ToId: amico.Id}
	crea(t, regola)
	voce := &model.AddressBook{Id: "999000111", UserId: utente.Id}
	crea(t, voce)
	tag := &model.Tag{Name: "lavoro", UserId: utente.Id, Color: 1}
	crea(t, tag)
	id := func(n uint) string { return strconv.FormatUint(uint64(n), 10) }
	return g, registro, strings.NewReplacer("UTENTE", id(utente.Id), "AMICO", id(amico.Id), "RUBRICA", id(ufficio.Id),
		"ALTRUI", id(altrui.Id), "REGOLA", id(regola.Id), "VOCE", id(voce.RowId), "TAG", id(tag.Id))
}

// statoDelPannello descrive con Raw, che rifiutaLetture non ferma, voci e tag
// come statoRubrica, e rubriche e regole di condivisione.
func statoDelPannello(t *testing.T) string {
	t.Helper()
	var rubriche, regole []string
	err := service.DB.Raw("SELECT name || ' ' || user_id FROM address_book_collections ORDER BY id").Scan(&rubriche).Error
	if err == nil {
		err = service.DB.Raw("SELECT collection_id || ' ' || type || ' ' || to_id || ' ' || rule FROM address_book_collection_rules ORDER BY id").Scan(&regole).Error
	}
	if err != nil {
		t.Fatal(err)
	}
	return statoRubrica(t) + fmt.Sprint(", rubriche ", rubriche, ", regole ", regole)
}

// casoDelPannello e' una richiesta al router di pannelloDiProva, coi
// segnaposto del suo Replacer in rotta e corpo, e la risposta attesa; con
// rifiuta, le letture che provaPannello rifiuta non riescono.
type casoDelPannello struct {
	metodo, rotta, corpo string
	rifiuta              bool
	risposta             string
}

// provaPannello manda ogni caso a un router nuovo di pannelloDiProva, con le
// letture della tabella tabella la cui WHERE contiene dove rifiutate se il
// caso lo chiede, e controlla la risposta. Con le letture rifiutate
// controlla anche che il database resti com'era e che nel log ci siano la
// rotta e l'errore.
func provaPannello(t *testing.T, tabella, dove string, casi []casoDelPannello) {
	t.Helper()
	rotta := strings.NewReplacer("/REGOLA", "/:id", "/TAG", "/:id")
	for _, tc := range casi {
		caso := ", senza ostacoli"
		if tc.rifiuta {
			caso = ", " + tabella + " non lette"
		}
		t.Run(tc.metodo+" "+tc.rotta+caso, func(t *testing.T) {
			g, registro, segnaposto := pannelloDiProva(t)
			prima := statoDelPannello(t)
			if tc.rifiuta {
				rifiutaLetture(t, tabella, dove)
			}
			rec := richiesta(g, tc.metodo, segnaposto.Replace(tc.rotta), "", segnaposto.Replace(tc.corpo))
			if got := rec.Body.String(); rec.Code != 200 || got != tc.risposta {
				t.Errorf("%s %s: stato %d\n got  %s\n want %s", tc.metodo, tc.rotta, rec.Code, got, tc.risposta)
			}
			if !tc.rifiuta {
				return
			}
			if dopo := statoDelPannello(t); dopo != prima {
				t.Errorf("%s %s ha cambiato il database:\n prima %s\n dopo  %s", tc.metodo, tc.rotta, prima, dopo)
			}
			if nelLog := registro.String(); !strings.Contains(nelLog, tc.metodo+" "+rotta.Replace(tc.rotta)+": ") || !strings.Contains(nelLog, "lettura rifiutata dal test") {
				t.Errorf("%s %s, nel log mancano rotta o errore:\n%s", tc.metodo, tc.rotta, nelLog)
			}
		})
	}
}

// TestPannelloRegolaNonLetta prova sul router vero le rotte del pannello che
// leggono una regola di condivisione per id: se il database non la legge
// rispondono code 101 "Errore di sistema.", con l'errore nel log, e non
// cambiano niente. Prima rispondevano "Elemento non trovato.". Una regola che
// non c'e' ha la risposta di prima.
func TestPannelloRegolaNonLetta(t *testing.T) {
	const erroreDiSistema = `{"code":101,"message":"Errore di sistema.","data":null}`
	const nonTrovato = `{"code":101,"message":"Elemento non trovato.","data":null}`
	const modifica = `{"id":REGOLA,"user_id":UTENTE,"collection_id":RUBRICA,"type":1,"to_id":AMICO,"rule":2}`
	provaPannello(t, "address_book_collection_rules", "", []casoDelPannello{
		{"GET", "/api/admin/address_book_collection_rule/detail/REGOLA", "", true, erroreDiSistema},
		{"POST", "/api/admin/address_book_collection_rule/delete", `{"id":REGOLA}`, true, erroreDiSistema},
		{"POST", "/api/admin/my/address_book_collection_rule/update", modifica, true, erroreDiSistema},
		{"POST", "/api/admin/my/address_book_collection_rule/delete", `{"id":REGOLA}`, true, erroreDiSistema},
		{"GET", "/api/admin/address_book_collection_rule/detail/999999", "", false, nonTrovato},
		{"POST", "/api/admin/address_book_collection_rule/delete", `{"id":999999}`, false, nonTrovato},
		{"POST", "/api/admin/my/address_book_collection_rule/update", strings.Replace(modifica, "REGOLA", "999999", 1), false, nonTrovato},
		{"POST", "/api/admin/my/address_book_collection_rule/delete", `{"id":999999}`, false, nonTrovato},
	})
}

// TestPannelloRegolaDoppia prova sul router vero le rotte del pannello che
// creano o cambiano una regola di condivisione, e prima guardano se ce n'e'
// gia' una con lo stesso tipo, destinatario e rubrica: se il database non la
// legge rispondono code 101 "Errore di sistema.", con l'errore nel log, e non
// salvano niente. Prima la lettura fallita valeva "non c'e'" e la regola
// doppia si salvava. Una regola che c'e' davvero ha la risposta di prima.
func TestPannelloRegolaDoppia(t *testing.T) {
	const erroreDiSistema = `{"code":101,"message":"Errore di sistema.","data":null}`
	const regola = `"user_id":UTENTE,"collection_id":RUBRICA,"type":1,"to_id":AMICO`
	provaPannello(t, "address_book_collection_rules", "to_id", []casoDelPannello{
		{"POST", "/api/admin/address_book_collection_rule/create", `{` + regola + `,"rule":2}`, true, erroreDiSistema},
		{"POST", "/api/admin/my/address_book_collection_rule/create", `{` + regola + `,"rule":2}`, true, erroreDiSistema},
		{"POST", "/api/admin/address_book_collection_rule/update", `{"id":REGOLA,` + regola + `,"rule":2}`, true, erroreDiSistema},
		{"POST", "/api/admin/my/address_book_collection_rule/update", `{"id":REGOLA,` + regola + `,"rule":2}`, true, erroreDiSistema},
		{"POST", "/api/admin/address_book_collection_rule/create", `{` + regola + `,"rule":2}`, false, `{"code":101,"message":"L'elemento esiste già.","data":null}`},
		{"POST", "/api/admin/my/address_book_collection_rule/create", `{` + regola + `,"rule":2}`, false, `{"code":101,"message":"L'elemento esiste già.","data":null}`},
		{"POST", "/api/admin/my/address_book_collection_rule/update", `{"id":REGOLA,` + regola + `,"rule":2}`, false, `{"code":0,"message":"success","data":null}`},
	})
}

// TestPannelloProprietarioNonLetto prova sul router vero le rotte del
// pannello che, prima di mettere una voce, un tag o una regola in una
// rubrica, guardano se la rubrica e' dell'utente: se il database non la
// legge rispondono code 101 "Errore di sistema.", con l'errore nel log, e non
// salvano niente. Prima rispondevano "Parametri non validi.", come a chi
// sceglie la rubrica di un altro, e l'errore si perdeva. Una rubrica di un
// altro o che non c'e' ha la risposta di prima.
func TestPannelloProprietarioNonLetto(t *testing.T) {
	const erroreDiSistema = `{"code":101,"message":"Errore di sistema.","data":null}`
	const parametri = `{"code":101,"message":"Parametri non validi.","data":null}`
	const voce = `"id":"999000111","user_id":UTENTE,"alias":"nuovo","collection_id":`
	const tag = `{"id":TAG,"name":"lavoro","color":2,"user_id":UTENTE,"collection_id":`
	const regola = `{"user_id":UTENTE,"type":1,"to_id":AMICO,"rule":2,"collection_id":`
	provaPannello(t, "address_book_collections", "", []casoDelPannello{
		{"POST", "/api/admin/address_book/create", `{` + voce + `RUBRICA}`, true, erroreDiSistema},
		{"POST", "/api/admin/address_book/update", `{"row_id":VOCE,` + voce + `RUBRICA}`, true, erroreDiSistema},
		{"POST", "/api/admin/my/address_book/create", `{` + voce + `RUBRICA}`, true, erroreDiSistema},
		{"POST", "/api/admin/my/address_book/update", `{"row_id":VOCE,` + voce + `RUBRICA}`, true, erroreDiSistema},
		{"POST", "/api/admin/my/tag/update", tag + `RUBRICA}`, true, erroreDiSistema},
		{"POST", "/api/admin/address_book_collection_rule/create", regola + `RUBRICA}`, true, erroreDiSistema},
		{"POST", "/api/admin/my/address_book_collection_rule/create", regola + `RUBRICA}`, true, erroreDiSistema},
		{"POST", "/api/admin/address_book/create", `{` + voce + `ALTRUI}`, false, parametri},
		{"POST", "/api/admin/my/address_book/update", `{"row_id":VOCE,` + voce + `999999}`, false, parametri},
		{"POST", "/api/admin/my/tag/update", tag + `ALTRUI}`, false, parametri},
		{"POST", "/api/admin/address_book_collection_rule/create", regola + `999999}`, false, parametri},
		{"POST", "/api/admin/my/address_book_collection_rule/create", regola + `ALTRUI}`, false, parametri},
	})
}

// TestPannelloVocePerRigaNonLetta prova sul router vero le rotte del pannello
// che leggono una voce per row_id: se il database non la legge rispondono
// code 101 "Errore di sistema.", con l'errore nel log, e non cambiano niente.
// Prima rispondevano "Elemento non trovato.". Una voce che non c'e' ha la
// risposta di prima.
func TestPannelloVocePerRigaNonLetta(t *testing.T) {
	const erroreDiSistema = `{"code":101,"message":"Errore di sistema.","data":null}`
	const nonTrovato = `{"code":101,"message":"Elemento non trovato.","data":null}`
	const modifica = `{"row_id":VOCE,"id":"999000111","user_id":UTENTE,"alias":"nuovo"}`
	provaPannello(t, "address_books", "row_id", []casoDelPannello{
		{"POST", "/api/admin/address_book/update", modifica, true, erroreDiSistema},
		{"POST", "/api/admin/address_book/delete", `{"row_id":VOCE}`, true, erroreDiSistema},
		{"POST", "/api/admin/my/address_book/update", modifica, true, erroreDiSistema},
		{"POST", "/api/admin/my/address_book/delete", `{"row_id":VOCE}`, true, erroreDiSistema},
		{"POST", "/api/admin/address_book/update", strings.Replace(modifica, "VOCE", "999999", 1), false, nonTrovato},
		{"POST", "/api/admin/address_book/delete", `{"row_id":999999}`, false, nonTrovato},
		{"POST", "/api/admin/my/address_book/update", strings.Replace(modifica, "VOCE", "999999", 1), false, nonTrovato},
		{"POST", "/api/admin/my/address_book/delete", `{"row_id":999999}`, false, nonTrovato},
	})
}

// TestPannelloTagPerIdNonLetto prova sul router vero le rotte del pannello che
// leggono un tag per id: se il database non lo legge rispondono code 101
// "Errore di sistema.", con l'errore nel log, e non cambiano niente. Prima
// rispondevano "Elemento non trovato.". Un tag che non c'e' ha la risposta di
// prima.
func TestPannelloTagPerIdNonLetto(t *testing.T) {
	const erroreDiSistema = `{"code":101,"message":"Errore di sistema.","data":null}`
	const nonTrovato = `{"code":101,"message":"Elemento non trovato.","data":null}`
	const modifica = `{"id":TAG,"name":"lavoro","color":2,"user_id":UTENTE}`
	provaPannello(t, "tags", "", []casoDelPannello{
		{"GET", "/api/admin/tag/detail/TAG", "", true, erroreDiSistema},
		{"POST", "/api/admin/tag/update", modifica, true, erroreDiSistema},
		{"POST", "/api/admin/tag/delete", `{"id":TAG}`, true, erroreDiSistema},
		{"POST", "/api/admin/my/tag/update", modifica, true, erroreDiSistema},
		{"POST", "/api/admin/my/tag/delete", `{"id":TAG}`, true, erroreDiSistema},
		{"GET", "/api/admin/tag/detail/999999", "", false, nonTrovato},
		{"POST", "/api/admin/tag/update", strings.Replace(modifica, "TAG", "999999", 1), false, nonTrovato},
		{"POST", "/api/admin/tag/delete", `{"id":999999}`, false, nonTrovato},
		{"POST", "/api/admin/my/tag/update", strings.Replace(modifica, "TAG", "999999", 1), false, nonTrovato},
		{"POST", "/api/admin/my/tag/delete", `{"id":999999}`, false, nonTrovato},
	})
}
