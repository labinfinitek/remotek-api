package service

import (
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/logger"

	"github.com/lejianwen/rustdesk-api/v2/config"
	"github.com/lejianwen/rustdesk-api/v2/lib/orm"
	"github.com/lejianwen/rustdesk-api/v2/model"
)

// TestLoginDaOidc prova che il token ricorda il tipo del login che l'ha
// creato: da OIDC si', dalla password no, e no anche per un token di prima
// della versione 276, che nella tabella vecchia non aveva la colonna e dopo
// la migrazione ce l'ha vuota. LoginDaOidc legge il token che RustAuth ha
// messo nella richiesta; senza, e' false.
func TestLoginDaOidc(t *testing.T) {
	senzaChiaveJwt(t)
	db, err := orm.ApriSqlite(filepath.Join(t.TempDir(), "api.db"), logger.Discard)
	if err != nil {
		t.Fatal(err)
	}
	// la tabella com'era fino alla versione 275, con un token dentro
	err = db.Exec("CREATE TABLE user_tokens (id integer PRIMARY KEY AUTOINCREMENT, user_id integer NOT NULL DEFAULT 0, " +
		"device_uuid text DEFAULT '', device_id text DEFAULT '', token text NOT NULL DEFAULT '', " +
		"expired_at integer NOT NULL DEFAULT 0, created_at datetime, updated_at datetime)").Error
	if err == nil {
		err = db.Exec("INSERT INTO user_tokens (user_id, token, expired_at) VALUES (1, 'di-prima', ?)", time.Now().Add(time.Hour).Unix()).Error
	}
	if err == nil {
		err = db.AutoMigrate(&model.User{}, &model.UserToken{}, &model.LoginLog{})
	}
	if err != nil {
		t.Fatal(err)
	}
	precDB, precConfig := DB, Config
	DB, Config = db, &config.Config{}
	t.Cleanup(func() { DB, Config = precDB, precConfig })

	us := &UserService{}
	u := &model.User{Username: "tecnico"}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	token := map[string]string{"di-prima": "di-prima"}
	for _, tipo := range []string{model.LoginLogTypeAccount, model.LoginLogTypeOauth} {
		ut, err := us.Login(u, &model.LoginLog{UserId: u.Id, Type: tipo})
		if err != nil {
			t.Fatal(err)
		}
		token[tipo] = ut.Token
	}
	for _, tc := range []struct {
		nome, token string
		oidc        bool
	}{
		{"password", token[model.LoginLogTypeAccount], false},
		{"OIDC", token[model.LoginLogTypeOauth], true},
		{"di prima della 276", token["di-prima"], false},
	} {
		_, ut, err := us.InfoByAccessToken(tc.token)
		if err != nil {
			t.Fatalf("token %s: %v", tc.nome, err)
		}
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("curUserToken", ut)
		if oidc := us.LoginDaOidc(c); oidc != tc.oidc {
			t.Errorf("token %s: da OIDC %t, atteso %t", tc.nome, oidc, tc.oidc)
		}
	}
	if c, _ := gin.CreateTestContext(httptest.NewRecorder()); us.LoginDaOidc(c) {
		t.Error("richiesta senza token: da OIDC true, atteso false")
	}
}
