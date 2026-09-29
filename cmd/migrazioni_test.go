package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	applog "github.com/lejianwen/rustdesk-api/v2/lib/logger"
	"github.com/lejianwen/rustdesk-api/v2/model"
)

// TestVincoloDispositivi prova sul binario vero la migrazione di un database
// nato dall'upstream fino al 2024-10-14, con la chiave esterna fk_peers_user
// in peers, che il test crea con gorm come allora (da Peer.User): prima,
// con foreign_keys acceso, un dispositivo senza utente non si salva; dopo
// l'avvio la chiave non c'e' piu', dispositivi e indici restano, e lo
// stesso dispositivo si salva.
func TestVincoloDispositivi(t *testing.T) {
	dir := sandbox(t)
	if codice, out := esegui(t, dir); codice != 0 {
		t.Fatalf("primo avvio: codice %d\n%s", codice, out)
	}
	db := apriDB(t, dir)
	indici := indiciDeiDispositivi(t, db)
	// CreateConstraint ricrea la tabella senza indici: AutoMigrate li rimette,
	// come li aveva il database di allora.
	err := db.Migrator().CreateConstraint(&model.Peer{}, "fk_peers_user")
	if err == nil {
		err = db.AutoMigrate(&model.Peer{})
	}
	if err == nil {
		err = db.Create(&model.Peer{Id: "999000111", Uuid: "dXVpZA=="}).Error
	}
	if err != nil {
		t.Fatal(err)
	}
	salva := func() error {
		t.Helper()
		conChiavi, err := gorm.Open(sqlite.Open(filepath.Join(dir, "data", "rustdeskapi.db")+"?_foreign_keys=on"), &gorm.Config{Logger: logger.Discard})
		if err != nil {
			t.Fatal(err)
		}
		sqlDB, err := conChiavi.DB()
		if err != nil {
			t.Fatal(err)
		}
		defer sqlDB.Close()
		return conChiavi.Create(&model.Peer{Id: "999000222", Uuid: "YWx0cm8="}).Error
	}
	if err := salva(); err == nil || !strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
		t.Fatalf("dispositivo senza utente col vincolo e foreign_keys acceso: %v, atteso FOREIGN KEY constraint failed", err)
	}

	codice, out := esegui(t, dir)
	if codice != 0 {
		t.Fatalf("avvio sul database col vincolo: codice %d\n%s", codice, out)
	}
	var chiavi int64
	if err := db.Raw("SELECT count(*) FROM pragma_foreign_key_list('peers')").Scan(&chiavi).Error; err != nil || chiavi != 0 {
		t.Errorf("chiavi esterne di peers dopo l'avvio: %d (%v), attese nessuna", chiavi, err)
	}
	if dopo := indiciDeiDispositivi(t, db); !slices.Equal(dopo, indici) {
		t.Errorf("indici di peers dopo l'avvio %v, attesi %v", dopo, indici)
	}
	if err := db.Where("id = ?", "999000111").First(&model.Peer{}).Error; err != nil {
		t.Errorf("dispositivo salvato prima dell'avvio: %v", err)
	}
	if err := salva(); err != nil {
		t.Errorf("dispositivo senza utente dopo l'avvio, con foreign_keys acceso: %v", err)
	}
	if applog.Conta(out, "INFO", "tolta da peers la chiave esterna fk_peers_user") != 1 {
		t.Errorf("il log non dice della chiave tolta:\n%s", out)
	}
}

// indiciDeiDispositivi restituisce, in ordine, i nomi degli indici di peers.
func indiciDeiDispositivi(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var indici []string
	if err := db.Raw("SELECT name FROM sqlite_master WHERE type = 'index' AND tbl_name = 'peers' ORDER BY name").Scan(&indici).Error; err != nil || len(indici) == 0 {
		t.Fatalf("indici di peers: %v (%v)", indici, err)
	}
	return indici
}
