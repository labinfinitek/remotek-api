package http

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// TestPannelloMy prova sul router vero che il pannello, nella sezione
// dell'utente (/api/admin/my/*), riceve solo il messaggio: un corpo JSON
// rotto a /api/admin/my/tag/create risponde 200 con code 101 e
// ParamsError, e il testo dell'errore di JSON va solo nel log, con metodo e
// rotta. Le rotte vogliono l'api-token di un utente abilitato, che il test
// crea nel database sqlite temporaneo; il resto dello stato globale e'
// quello di TestMessaggi.
func TestPannelloMy(t *testing.T) {
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
		err = db.AutoMigrate(&model.User{}, &model.UserToken{}, &model.Tag{})
	}
	if err != nil {
		t.Fatal(err)
	}
	utente := &model.User{Username: "prova", Status: model.COMMON_STATUS_ENABLE}
	if err := db.Create(utente).Error; err != nil {
		t.Fatal(err)
	}
	const token = "token-del-pannello"
	ut := &model.UserToken{UserId: utente.Id, Token: token, ExpiredAt: time.Now().Add(24 * time.Hour).Unix()}
	if err := db.Create(ut).Error; err != nil {
		t.Fatal(err)
	}
	service.New(&global.Config, db, global.Logger, nil, lock.NewLocal())
	global.LoginLimiter = utils.NewLoginLimiter(utils.SecurityPolicy{CaptchaThreshold: global.Config.App.CaptchaThreshold})
	g := NewEngine()

	const erroreJSON = "invalid character 'x' looking for beginning of value"
	req := httptest.NewRequest("POST", "/api/admin/my/tag/create", strings.NewReader(`{"name": x}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-token", token)
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
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
