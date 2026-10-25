package service

import (
	"crypto/md5"
	"encoding/hex"
	"path/filepath"
	"testing"

	"gorm.io/gorm/logger"

	"github.com/lejianwen/rustdesk-api/v2/config"
	"github.com/lejianwen/rustdesk-api/v2/lib/orm"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/utils"
)

// TestEccezioniConfrontoFinto prova il percorso del login con password con
// app.disable-pwd-login acceso, come lo fanno /api/login e /api/admin/login:
// EccezionePwdLogin e poi, per un'eccezione, InfoByUsernamePassword. Un nome
// dell'elenco senza utente locale, o con un hash che non e' bcrypt, passa una
// volta da hashFinto, come un nome fuori dall'elenco; un'eccezione con un
// hash bcrypt fa il suo confronto vero e non quello finto. Cosi' ogni
// richiesta costa un bcrypt e i tempi non dicono quali nomi hanno un utente.
// Prima i primi due casi rispondevano senza nessun bcrypt.
func TestEccezioniConfrontoFinto(t *testing.T) {
	db, err := orm.ApriSqlite(filepath.Join(t.TempDir(), "api.db"), logger.Discard)
	if err == nil {
		err = db.AutoMigrate(&model.User{})
	}
	if err != nil {
		t.Fatal(err)
	}
	precDB, precConfig, precConfronto := DB, Config, confrontoFinto
	DB, Config = db, &config.Config{}
	Config.App.DisablePwdLogin = true
	Config.App.PwdLoginEccezioni = "fantasma,vecchio,nuovo"
	confronti := 0
	confrontoFinto = func(string) { confronti++ }
	t.Cleanup(func() { DB, Config, confrontoFinto = precDB, precConfig, precConfronto })

	somma := md5.Sum([]byte("vecchia-password" + "rustdesk-api"))
	nuovo, err := utils.EncryptPassword("nuova-password")
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []*model.User{{Username: "vecchio", Password: hex.EncodeToString(somma[:])}, {Username: "nuovo", Password: nuovo}} {
		if err := db.Create(u).Error; err != nil {
			t.Fatal(err)
		}
	}

	us := &UserService{}
	for _, tc := range []struct {
		utente, password string
		confronti        int
	}{
		{"fantasma", "qualunque", 1},
		{"vecchio", "vecchia-password", 1},
		{"estraneo", "qualunque", 1},
		{"nuovo", "nuova-password", 0},
		{"nuovo", "sbagliata", 0},
	} {
		confronti = 0
		motivo, err := us.EccezionePwdLogin(tc.utente, tc.password, false)
		if err != nil {
			t.Fatal(err)
		}
		if motivo != "" {
			_, _ = us.InfoByUsernamePassword(tc.utente, tc.password)
		}
		if confronti != tc.confronti {
			t.Errorf("%s con %q: %d confronti con hashFinto, attesi %d", tc.utente, tc.password, confronti, tc.confronti)
		}
	}
}
