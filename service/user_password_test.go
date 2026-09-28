package service

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"path/filepath"
	"testing"

	"gorm.io/gorm/logger"

	"github.com/lejianwen/rustdesk-api/v2/config"
	"github.com/lejianwen/rustdesk-api/v2/lib/orm"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/utils"
)

// TestInfoByUsernamePasswordMD5 prova il controllo della password del login
// del client e del pannello: un utente con l'hash md5 delle versioni molto
// vecchie di rustdesk-api non entra neanche con la password giusta, riceve
// ErrNotFound come per una password sbagliata, e l'hash nel database resta
// com'era. Un utente con l'hash bcrypt entra come prima.
func TestInfoByUsernamePasswordMD5(t *testing.T) {
	db, err := orm.ApriSqlite(filepath.Join(t.TempDir(), "api.db"), logger.Discard)
	if err == nil {
		err = db.AutoMigrate(&model.User{})
	}
	if err != nil {
		t.Fatal(err)
	}
	precDB, precConfig := DB, Config
	DB, Config = db, &config.Config{}
	t.Cleanup(func() { DB, Config = precDB, precConfig })

	somma := md5.Sum([]byte("vecchia-password" + "rustdesk-api"))
	vecchio := hex.EncodeToString(somma[:])
	nuovo, err := utils.EncryptPassword("nuova-password")
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []*model.User{{Username: "vecchio", Password: vecchio}, {Username: "nuovo", Password: nuovo}} {
		if err := db.Create(u).Error; err != nil {
			t.Fatal(err)
		}
	}

	us := &UserService{}
	for _, tc := range []struct {
		utente, password string
		entra            bool
	}{
		{"vecchio", "vecchia-password", false},
		{"vecchio", "sbagliata", false},
		{"nuovo", "nuova-password", true},
		{"nuovo", "sbagliata", false},
	} {
		u, err := us.InfoByUsernamePassword(tc.utente, tc.password)
		if entra := err == nil && u.Id != 0; entra != tc.entra {
			t.Errorf("%s con %q: entra %t (errore %v), atteso %t", tc.utente, tc.password, entra, err, tc.entra)
		}
		if !tc.entra && !errors.Is(err, ErrNotFound) {
			t.Errorf("%s con %q: errore %v, atteso ErrNotFound come per una password sbagliata", tc.utente, tc.password, err)
		}
	}
	salvato := &model.User{}
	if err := db.Where("username = ?", "vecchio").First(salvato).Error; err != nil {
		t.Fatal(err)
	}
	if salvato.Password != vecchio {
		t.Errorf("l'hash md5 e' stato riscritto: %q", salvato.Password)
	}
}
