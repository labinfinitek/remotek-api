package http

// Test delle transazioni sul router vero: con una connessione sola, lo
// scrittore unico di ADR-0007, niente dentro una transazione aspetta una
// seconda connessione; su errore la transazione si annulla tutta.

import (
	"context"
	"encoding/json"
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

// unaConnessione da' 5 secondi al database dei servizi, che come in
// produzione ha una connessione sola (orm.ApriSqlite). Con una connessione
// sola una query su DB fatta dentro una transazione aspetta per sempre la
// connessione che la transazione tiene: con la scadenza l'attesa finisce in
// errore e il test fallisce invece di restare appeso. Restituisce il
// contesto della scadenza: scaduto vuol dire che una richiesta si e' fermata
// ad aspettare la connessione.
func unaConnessione(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)
	service.DB = service.DB.WithContext(ctx)
	return ctx
}

// rifiuta fa fallire nel database dei servizi, con un trigger, ogni
// operazione che quando descrive, per esempio "BEFORE INSERT ON tags".
func rifiuta(t *testing.T, quando string) {
	t.Helper()
	if err := service.DB.Exec("CREATE TRIGGER rifiuta " + quando + " BEGIN SELECT RAISE(ABORT, 'rifiutato dal test'); END").Error; err != nil {
		t.Fatal(err)
	}
}

// rubricaDelClient crea le tabelle della rubrica nel database di pannello e
// restituisce una funzione che manda a POST /api/ab, come il client e col
// token di pannello, le voci peers e i colori dei tag tagColors. RustAuth
// legge la chiave dei JWT: vuota, come senza jwt.key, vale il token del
// database.
func rubricaDelClient(t *testing.T, g *gin.Engine) func(peers []map[string]string, tagColors string) *httptest.ResponseRecorder {
	t.Helper()
	if err := service.DB.AutoMigrate(&model.AddressBook{}, &model.Peer{}); err != nil {
		t.Fatal(err)
	}
	precJwt := global.Jwt
	global.Jwt = jwt.NewJwt("", time.Hour)
	t.Cleanup(func() { global.Jwt = precJwt })
	return func(peers []map[string]string, tagColors string) *httptest.ResponseRecorder {
		t.Helper()
		dati, err := json.Marshal(map[string]any{"peers": peers, "tag_colors": tagColors})
		if err != nil {
			t.Fatal(err)
		}
		corpo, err := json.Marshal(map[string]string{"data": string(dati)})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("POST", "/api/ab", strings.NewReader(string(corpo)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenDelPannello)
		rec := httptest.NewRecorder()
		g.ServeHTTP(rec, req)
		return rec
	}
}

// TestRubricaDispositivoNuovo prova POST /api/ab del client con una voce
// nuova senza platform, username e hostname, che l'API prende dal
// dispositivo, e un tag. La lettura del dispositivo sta dentro la
// transazione della rubrica: con una connessione sola, se passasse da DB e
// non dalla transazione, la richiesta non finirebbe mai. Il golden
// ab-legacy-post manda "peers":[] e non passa di li'.
func TestRubricaDispositivoNuovo(t *testing.T) {
	g, utente, _ := pannello(t, false)
	invia := rubricaDelClient(t, g)
	dispositivo := &model.Peer{Id: "999000111", Os: "Windows 11", Username: "collaudo", Hostname: "PC-COLLAUDO"}
	if err := service.DB.Create(dispositivo).Error; err != nil {
		t.Fatal(err)
	}
	scadenza := unaConnessione(t)
	rec := invia([]map[string]string{{"id": dispositivo.Id}}, `{"lavoro":4288585374}`)
	if scadenza.Err() != nil {
		t.Fatalf("POST /api/ab: ferma per 5 secondi ad aspettare la connessione del database (stallo); risposta %d %s", rec.Code, rec.Body)
	}
	if rec.Code != 200 || rec.Body.String() != "null" {
		t.Fatalf("POST /api/ab: %d %s, attesi 200 null", rec.Code, rec.Body)
	}
	voce := &model.AddressBook{}
	if err := service.DB.Where("user_id = ? AND id = ?", utente.Id, dispositivo.Id).First(voce).Error; err != nil {
		t.Fatalf("voce della rubrica: %v", err)
	}
	if voce.Platform != "Windows" || voce.Username != dispositivo.Username || voce.Hostname != dispositivo.Hostname {
		t.Errorf("voce della rubrica: platform %q, username %q, hostname %q, attesi quelli del dispositivo", voce.Platform, voce.Username, voce.Hostname)
	}
	tag := &model.Tag{}
	if err := service.DB.Where("user_id = ? AND name = ?", utente.Id, "lavoro").First(tag).Error; err != nil || tag.Color != 4288585374 {
		t.Errorf("tag lavoro: colore %d (err %v), atteso 4288585374", tag.Color, err)
	}
}

// TestRubricaTagAnnullati prova che POST /api/ab, se un tag non si salva,
// annulla tutti i cambi ai tag e risponde col messaggio d'errore del
// contratto, 400 {"error": ...}: prima cancellava gli altri tag e
// rispondeva 200 null.
func TestRubricaTagAnnullati(t *testing.T) {
	g, utente, registro := pannello(t, false)
	invia := rubricaDelClient(t, g)
	if err := service.DB.Create(&model.Tag{Name: "lavoro", Color: 1, UserId: utente.Id}).Error; err != nil {
		t.Fatal(err)
	}
	rifiuta(t, "BEFORE INSERT ON tags")
	rec := invia([]map[string]string{}, `{"casa":2}`)
	if rec.Code != 400 || rec.Body.String() != `{"error":"Operazione non riuscita."}` {
		t.Errorf("POST /api/ab col tag nuovo rifiutato: %d %s, attesi 400 e OperationFailed", rec.Code, rec.Body)
	}
	if !strings.Contains(registro.String(), "rifiutato dal test") {
		t.Errorf("l'errore del database non e' nel log:\n%s", registro)
	}
	var tags []model.Tag
	if err := service.DB.Where("user_id = ?", utente.Id).Find(&tags).Error; err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0].Name != "lavoro" {
		t.Errorf("tag dopo l'errore: %+v, atteso solo lavoro, come prima", tags)
	}
}

// TestCollezioneCancellazioneAnnullata prova che la cancellazione di una
// collezione dal pannello, se la collezione non si cancella, lascia anche
// regole e voci: prima le cancellava e rispondeva successo.
func TestCollezioneCancellazioneAnnullata(t *testing.T) {
	g, utente, _ := pannello(t, false)
	err := service.DB.AutoMigrate(&model.AddressBookCollection{}, &model.AddressBookCollectionRule{}, &model.AddressBook{})
	if err != nil {
		t.Fatal(err)
	}
	collezione := &model.AddressBookCollection{UserId: utente.Id, Name: "ufficio"}
	if err := service.DB.Create(collezione).Error; err != nil {
		t.Fatal(err)
	}
	err = service.DB.Create(&model.AddressBookCollectionRule{UserId: utente.Id, CollectionId: collezione.Id, Rule: 1, Type: 1, ToId: 2}).Error
	if err == nil {
		err = service.DB.Create(&model.AddressBook{Id: "999000111", UserId: utente.Id, CollectionId: collezione.Id}).Error
	}
	if err != nil {
		t.Fatal(err)
	}
	rifiuta(t, "BEFORE DELETE ON address_book_collections")
	rec := alPannello(g, "/api/admin/my/address_book_collection/delete", `{"id":`+strconv.FormatUint(uint64(collezione.Id), 10)+`}`)
	if got, want := rec.Body.String(), `{"code":101,"message":"Operazione non riuscita.","data":null}`; got != want {
		t.Errorf("cancellazione della collezione rifiutata:\n got  %s\n want %s", got, want)
	}
	for _, m := range []any{&model.AddressBookCollectionRule{}, &model.AddressBook{}} {
		var n int64
		if err := service.DB.Model(m).Where("collection_id = ?", collezione.Id).Count(&n).Error; err != nil || n != 1 {
			t.Errorf("%T della collezione dopo l'errore: %d (err %v), attesa 1", m, n, err)
		}
	}
}

// TestRegistrazioneOidcAnnullata prova che il login OIDC con
// autoregistrazione, se l'associazione al provider non si salva, non lascia
// l'utente creato e risponde OauthRegisterFailed: prima confermava l'utente
// senza associazione e rispondeva successo, e al login dopo ne nasceva un
// altro.
func TestRegistrazioneOidcAnnullata(t *testing.T) {
	g, _, _ := pannello(t, false)
	registra := true
	if err := service.DB.Create(&model.Oauth{Op: "aziendale", OauthType: model.OauthTypeOidc, ClientId: "id", ClientSecret: "segreto",
		Issuer: providerOidc(t), AutoRegister: &registra}).Error; err != nil {
		t.Fatal(err)
	}
	rifiuta(t, "BEFORE INSERT ON user_thirds")
	rec := richiesta(g, "POST", "/api/oidc/auth", "", `{"op":"aziendale","id":"999000111","uuid":"dXVpZA==","deviceInfo":{"os":"windows","type":"client","name":"PC-COLLAUDO"}}`)
	var risposta struct{ Code string }
	if err := json.Unmarshal(rec.Body.Bytes(), &risposta); err != nil || rec.Code != 200 {
		t.Fatalf("POST /api/oidc/auth: %d %s (%v)", rec.Code, rec.Body, err)
	}
	t.Cleanup(func() { service.AllService.OauthService.DeleteOauthCache(risposta.Code) })
	rec = richiesta(g, "GET", "/api/oidc/callback?state="+risposta.Code+"&code="+codiceDelProvider, "", "")
	if !strings.Contains(rec.Body.String(), "var msg = 'OauthRegisterFailed'") {
		t.Errorf("GET /api/oidc/callback con l'associazione rifiutata: %d, atteso il messaggio OauthRegisterFailed", rec.Code)
	}
	var n int64
	if err := service.DB.Model(&model.User{}).Where("username = ?", "utente-oidc").Count(&n).Error; err != nil || n != 0 {
		t.Errorf("utenti utente-oidc dopo l'errore: %d (err %v), attesi 0", n, err)
	}
}

// TestCancellazioneUtente prova la cancellazione di un utente dal pannello:
// se una delle cancellazioni collegate non riesce, resta tutto e la
// risposta e' OperationFailed; senza ostacoli se ne vanno l'utente, le sue
// voci della rubrica e le sue regole, con una connessione sola.
func TestCancellazioneUtente(t *testing.T) {
	g, _, _ := pannello(t, true)
	err := service.DB.AutoMigrate(&model.AddressBook{}, &model.AddressBookCollection{}, &model.AddressBookCollectionRule{}, &model.Peer{})
	if err != nil {
		t.Fatal(err)
	}
	utente := &model.User{Username: "da-cancellare", Status: model.COMMON_STATUS_ENABLE}
	if err := service.DB.Create(utente).Error; err != nil {
		t.Fatal(err)
	}
	err = service.DB.Create(&model.AddressBook{Id: "999000111", UserId: utente.Id}).Error
	if err == nil {
		err = service.DB.Create(&model.AddressBookCollectionRule{UserId: utente.Id, CollectionId: 1, Rule: 1, Type: 1, ToId: 2}).Error
	}
	if err != nil {
		t.Fatal(err)
	}
	righe := func() (n [3]int64) {
		t.Helper()
		for i, m := range []any{&model.User{}, &model.AddressBook{}, &model.AddressBookCollectionRule{}} {
			campo := "user_id"
			if i == 0 {
				campo = "id"
			}
			if err := service.DB.Model(m).Where(campo+" = ?", utente.Id).Count(&n[i]).Error; err != nil {
				t.Fatal(err)
			}
		}
		return n
	}
	corpo := `{"id":` + strconv.FormatUint(uint64(utente.Id), 10) + `}`

	rifiuta(t, "BEFORE DELETE ON address_book_collection_rules")
	scadenza := unaConnessione(t)
	rec := alPannello(g, "/api/admin/user/delete", corpo)
	if got, want := rec.Body.String(), `{"code":101,"message":"Operazione non riuscita.","data":null}`; got != want {
		t.Errorf("cancellazione con le regole rifiutate:\n got  %s\n want %s", got, want)
	}
	if n := righe(); n != [3]int64{1, 1, 1} {
		t.Errorf("utente, voci e regole dopo l'errore: %v, attesi [1 1 1]", n)
	}

	if err := service.DB.Exec("DROP TRIGGER rifiuta").Error; err != nil {
		t.Fatal(err)
	}
	rec = alPannello(g, "/api/admin/user/delete", corpo)
	if scadenza.Err() != nil {
		t.Fatal("POST /api/admin/user/delete: ferma per 5 secondi ad aspettare la connessione del database (stallo)")
	}
	if got, want := rec.Body.String(), `{"code":0,"message":"success","data":null}`; got != want {
		t.Errorf("cancellazione senza ostacoli:\n got  %s\n want %s", got, want)
	}
	if n := righe(); n != [3]int64{} {
		t.Errorf("utente, voci e regole dopo la cancellazione: %v, attesi [0 0 0]", n)
	}
}
