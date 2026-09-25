package http

import (
	"crypto/md5"
	"encoding/hex"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
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

const tokenDelPannello = "token-del-pannello"

// pannello prepara il router vero per le prove del pannello: lo stato
// globale e' quello di TestMessaggi, il database e' un sqlite temporaneo con
// l'utente "prova" (amministratore se admin) e il suo api-token
// tokenDelPannello. Restituisce il router, l'utente e il registro del log.
func pannello(t *testing.T, admin bool) (*gin.Engine, *model.User, *strings.Builder) {
	t.Helper()
	t.Setenv("RUSTDESK_API_LANG", "")
	t.Setenv("RUSTDESK_API_GIN_MODE", "test")
	t.Chdir("..") // il router legge resources/ dalla cartella corrente
	global.Viper = config.Init(&global.Config, filepath.Join("conf", "config.yaml"))
	registro := &strings.Builder{}
	global.Logger = logrus.New()
	global.Logger.SetOutput(registro)
	global.InitI18n()
	global.ApiInitValidator()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "api.db")), &gorm.Config{Logger: logger.Discard})
	if err == nil {
		err = db.AutoMigrate(&model.User{}, &model.UserToken{}, &model.Tag{})
	}
	if err != nil {
		t.Fatal(err)
	}
	utente := &model.User{Username: "prova", Status: model.COMMON_STATUS_ENABLE, IsAdmin: &admin}
	if err := db.Create(utente).Error; err != nil {
		t.Fatal(err)
	}
	ut := &model.UserToken{UserId: utente.Id, Token: tokenDelPannello, ExpiredAt: time.Now().Add(24 * time.Hour).Unix()}
	if err := db.Create(ut).Error; err != nil {
		t.Fatal(err)
	}
	service.New(&global.Config, db, global.Logger, nil, lock.NewLocal())
	global.LoginLimiter = utils.NewLoginLimiter(utils.SecurityPolicy{CaptchaThreshold: global.Config.App.CaptchaThreshold})
	return NewEngine(), utente, registro
}

// alPannello manda corpo in POST a rotta con l'api-token di pannello.
func alPannello(g *gin.Engine, rotta, corpo string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", rotta, strings.NewReader(corpo))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-token", tokenDelPannello)
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	return rec
}

// TestPannelloMy prova sul router vero che il pannello, nella sezione
// dell'utente (/api/admin/my/*), riceve solo il messaggio: un corpo JSON
// rotto a /api/admin/my/tag/create risponde 200 con code 101 e
// ParamsError, e il testo dell'errore di JSON va solo nel log, con metodo e
// rotta. Le rotte vogliono l'api-token di un utente abilitato, che pannello
// crea nel database temporaneo.
func TestPannelloMy(t *testing.T) {
	g, _, registro := pannello(t, false)

	const erroreJSON = "invalid character 'x' looking for beginning of value"
	rec := alPannello(g, "/api/admin/my/tag/create", `{"name": x}`)
	if got, want := rec.Code, 200; got != want {
		t.Errorf("POST /api/admin/my/tag/create con JSON rotto: stato %d, atteso %d", got, want)
	}
	if got, want := rec.Body.String(), `{"code":101,"message":"Parametri non validi.","data":null}`; got != want {
		t.Errorf("POST /api/admin/my/tag/create con JSON rotto:\n got  %s\n want %s", got, want)
	}
	if nelLog := registro.String(); !strings.Contains(nelLog, "POST /api/admin/my/tag/create: ") || !strings.Contains(nelLog, erroreJSON) {
		t.Errorf("POST /api/admin/my/tag/create con JSON rotto, nel log mancano rotta o errore:\n%s", nelLog)
	}
}

// TestPannelloAdmin prova sul router vero che il pannello di
// amministrazione riceve solo il messaggio, con un utente amministratore
// (/create e /changePwd vogliono AdminPrivilege). Un nome utente che esiste
// gia' riceve il messaggio di UsernameExists, l'ID che UserService.Create
// restituisce, e non "Operazione non riuscita." col nudo ID attaccato. Una
// password di 25 "€" passa il validatore (15-32 caratteri) ma e' di 75 byte,
// oltre i 72 di bcrypt: al pannello va OperationFailed, il testo di bcrypt
// solo nel log.
func TestPannelloAdmin(t *testing.T) {
	g, admin, registro := pannello(t, true)

	const erroreBcrypt = "bcrypt: password length exceeds 72 bytes"
	for _, tc := range []struct {
		rotta, corpo, risposta, nelLog string
	}{
		{"/api/admin/user/create", `{"username": "prova", "group_id": 1, "status": 1}`,
			`{"code":101,"message":"Il nome utente esiste già.","data":null}`, "UsernameExists"},
		{"/api/admin/user/changePwd", `{"id": ` + strconv.FormatUint(uint64(admin.Id), 10) + `, "password": "` + strings.Repeat("€", 25) + `"}`,
			`{"code":101,"message":"Operazione non riuscita.","data":null}`, erroreBcrypt},
	} {
		registro.Reset()
		rec := alPannello(g, tc.rotta, tc.corpo)
		if got, want := rec.Code, 200; got != want {
			t.Errorf("POST %s: stato %d, atteso %d", tc.rotta, got, want)
		}
		if got := rec.Body.String(); got != tc.risposta {
			t.Errorf("POST %s:\n got  %s\n want %s", tc.rotta, got, tc.risposta)
		}
		if nelLog := registro.String(); !strings.Contains(nelLog, "POST "+tc.rotta+": ") || !strings.Contains(nelLog, tc.nelLog) {
			t.Errorf("POST %s, nel log mancano rotta o errore:\n%s", tc.rotta, nelLog)
		}
	}
}

// TestPannelloVecchiaPasswordMD5 prova sul router vero il cambio della
// propria password quando nel database c'e' l'hash md5 delle versioni molto
// vecchie di rustdesk-api: anche con la vecchia password giusta il pannello
// riceve "Vecchia password errata.", come per una password sbagliata, senza
// il testo dell'errore di bcrypt, e l'hash resta com'era.
func TestPannelloVecchiaPasswordMD5(t *testing.T) {
	g, utente, _ := pannello(t, false)
	somma := md5.Sum([]byte("vecchia-password" + "rustdesk-api"))
	vecchio := hex.EncodeToString(somma[:])
	if err := service.DB.Model(utente).Update("password", vecchio).Error; err != nil {
		t.Fatal(err)
	}

	rec := alPannello(g, "/api/admin/user/changeCurPwd", `{"old_password": "vecchia-password", "new_password": "`+strings.Repeat("n", 15)+`"}`)
	if got, want := rec.Body.String(), `{"code":101,"message":"Vecchia password errata.","data":null}`; rec.Code != 200 || got != want {
		t.Errorf("POST /api/admin/user/changeCurPwd: stato %d\n got  %s\n want %s", rec.Code, got, want)
	}
	salvato := &model.User{}
	if err := service.DB.First(salvato, utente.Id).Error; err != nil {
		t.Fatal(err)
	}
	if salvato.Password != vecchio {
		t.Errorf("la password e' cambiata: %q", salvato.Password)
	}
}

// TestPannelloBatchCreateFromPeers prova sul router vero che
// /api/admin/my/address_book/batchCreateFromPeers non risponde successo se
// una riga della rubrica non si salva: un trigger di sqlite rifiuta il
// secondo peer, il pannello riceve OperationFailed e il testo dell'errore va
// solo nel log. La riga del primo peer, creata prima dell'errore, resta.
func TestPannelloBatchCreateFromPeers(t *testing.T) {
	g, utente, registro := pannello(t, false)
	if err := service.DB.AutoMigrate(&model.Peer{}, &model.AddressBook{}); err != nil {
		t.Fatal(err)
	}
	peers := []*model.Peer{{Id: "peer-a", UserId: utente.Id}, {Id: "peer-b", UserId: utente.Id}}
	if err := service.DB.Create(&peers).Error; err != nil {
		t.Fatal(err)
	}
	const rifiuto = "riga rifiutata dalla prova"
	if err := service.DB.Exec(`CREATE TRIGGER rifiuta_peer_b BEFORE INSERT ON address_books
		WHEN NEW.id = 'peer-b' BEGIN SELECT RAISE(ABORT, '` + rifiuto + `'); END`).Error; err != nil {
		t.Fatal(err)
	}

	rotta := "/api/admin/my/address_book/batchCreateFromPeers"
	corpo := `{"peer_ids": [` + strconv.FormatUint(uint64(peers[0].RowId), 10) + `, ` + strconv.FormatUint(uint64(peers[1].RowId), 10) + `]}`
	rec := alPannello(g, rotta, corpo)
	if got, want := rec.Body.String(), `{"code":101,"message":"Operazione non riuscita.","data":null}`; rec.Code != 200 || got != want {
		t.Errorf("POST %s: stato %d\n got  %s\n want %s", rotta, rec.Code, got, want)
	}
	if nelLog := registro.String(); !strings.Contains(nelLog, "POST "+rotta+": ") || !strings.Contains(nelLog, rifiuto) {
		t.Errorf("POST %s, nel log mancano rotta o errore:\n%s", rotta, nelLog)
	}
	var salvati []string
	if err := service.DB.Model(&model.AddressBook{}).Where("user_id = ?", utente.Id).Order("id").Pluck("id", &salvati).Error; err != nil {
		t.Fatal(err)
	}
	if want := []string{"peer-a"}; !slices.Equal(salvati, want) {
		t.Errorf("righe della rubrica dopo l'errore: %q, attese %q", salvati, want)
	}
}

// conToken manda una richiesta a rotta con l'api-token token.
func conToken(g *gin.Engine, metodo, rotta, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(metodo, rotta, nil)
	req.Header.Set("api-token", token)
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	return rec
}

// TestPannelloLogout prova sul router vero che il logout del pannello
// invalida il token: dopo POST /api/admin/logout con l'api-token,
// GET /api/admin/user/current con lo stesso token risponde come a un token
// che non esiste. Prima la rotta di logout
// stava fuori da BackendUserAuth: senza utente ne' token nel contesto
// rispondeva successo e il token restava valido fino alla scadenza.
func TestPannelloLogout(t *testing.T) {
	g, _, _ := pannello(t, false)
	const protetta = "/api/admin/user/current"
	nonValido := conToken(g, "GET", protetta, "token-che-non-esiste")

	if rec := conToken(g, "GET", protetta, tokenDelPannello); rec.Code != 200 || !strings.HasPrefix(rec.Body.String(), `{"code":0,`) {
		t.Fatalf("GET %s prima del logout: stato %d, corpo %s", protetta, rec.Code, rec.Body.String())
	}
	rec := alPannello(g, "/api/admin/logout", "")
	if got, want := rec.Body.String(), `{"code":0,"message":"success","data":null}`; rec.Code != 200 || got != want {
		t.Errorf("POST /api/admin/logout: stato %d\n got  %s\n want %s", rec.Code, got, want)
	}
	dopo := conToken(g, "GET", protetta, tokenDelPannello)
	if dopo.Code != nonValido.Code || dopo.Body.String() != nonValido.Body.String() {
		t.Errorf("GET %s dopo il logout: stato %d, corpo %s; a un token non valido: stato %d, corpo %s",
			protetta, dopo.Code, dopo.Body.String(), nonValido.Code, nonValido.Body.String())
	}
}

// TestPannelloLogoutFallito prova sul router vero che il logout del pannello
// non risponde successo se il token non si cancella: un trigger di sqlite
// rifiuta la cancellazione, il pannello riceve OperationFailed, il testo
// dell'errore va solo nel log e il token resta valido.
func TestPannelloLogoutFallito(t *testing.T) {
	g, _, registro := pannello(t, false)
	const rifiuto = "cancellazione rifiutata dalla prova"
	if err := service.DB.Exec(`CREATE TRIGGER rifiuta_logout BEFORE DELETE ON user_tokens
		BEGIN SELECT RAISE(ABORT, '` + rifiuto + `'); END`).Error; err != nil {
		t.Fatal(err)
	}

	rec := alPannello(g, "/api/admin/logout", "")
	if got, want := rec.Body.String(), `{"code":101,"message":"Operazione non riuscita.","data":null}`; rec.Code != 200 || got != want {
		t.Errorf("POST /api/admin/logout: stato %d\n got  %s\n want %s", rec.Code, got, want)
	}
	if nelLog := registro.String(); !strings.Contains(nelLog, "POST /api/admin/logout: ") || !strings.Contains(nelLog, rifiuto) {
		t.Errorf("POST /api/admin/logout, nel log mancano rotta o errore:\n%s", nelLog)
	}
	if rec := conToken(g, "GET", "/api/admin/user/current", tokenDelPannello); rec.Code != 200 || !strings.HasPrefix(rec.Body.String(), `{"code":0,`) {
		t.Errorf("GET /api/admin/user/current dopo il logout fallito: stato %d, corpo %s", rec.Code, rec.Body.String())
	}
}
