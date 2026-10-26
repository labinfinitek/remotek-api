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

// lasciapassare277 e' model.Lasciapassare com'era alla versione del database
// 277, prima di capacita, porte e durata.
type lasciapassare277 struct {
	model.IdModel
	Nonce       string `gorm:"size:32;not null;uniqueIndex"`
	Pc          string `gorm:"size:100;not null;index"`
	AgenteId    uint   `gorm:"not null;index"`
	TecnicoId   uint   `gorm:"not null"`
	Modo        string `gorm:"size:20;not null;default:''"`
	Emesso      int64  `gorm:"not null"`
	Scade       int64  `gorm:"not null"`
	ConnId      int64  `gorm:"not null;default:0"`
	AuditConnId uint   `gorm:"not null;default:0;index"`
	Creato      int64  `gorm:"not null"`
}

func (lasciapassare277) TableName() string { return "lasciapassare" }

// TestLasciapassareDiPrima prova sul binario vero che una riga di
// lasciapassare scritta alla versione 277 del database, senza le colonne
// della 278, dopo la migrazione si legge col terminale, nessuna porta e
// durata 0, e il resto com'era.
func TestLasciapassareDiPrima(t *testing.T) {
	dir := sandbox(t)
	if codice, out := esegui(t, dir); codice != 0 {
		t.Fatalf("primo avvio: codice %d\n%s", codice, out)
	}
	db := apriDB(t, dir)
	prima := lasciapassare277{Nonce: "bm9uY2UtZGktcHJpbWE", Pc: "999000111", AgenteId: 3, TecnicoId: 2,
		Modo: "utente", Emesso: 1793000000, Scade: 1793000060, ConnId: 7, AuditConnId: 5, Creato: 1793000000}
	err := db.Migrator().DropTable("lasciapassare")
	if err == nil {
		err = db.AutoMigrate(&lasciapassare277{})
	}
	if err == nil {
		err = db.Create(&prima).Error
	}
	if err == nil {
		err = db.Exec("UPDATE versions SET version = 277").Error
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{"capacita", "porte", "durata"} {
		if db.Migrator().HasColumn("lasciapassare", c) {
			t.Fatalf("la tabella di prima ha gia' la colonna %s", c)
		}
	}

	if codice, out := esegui(t, dir); codice != 0 {
		t.Fatalf("avvio sul database alla versione 277: codice %d\n%s", codice, out)
	}
	var dopo model.Lasciapassare
	if err := db.Where("nonce = ?", prima.Nonce).First(&dopo).Error; err != nil {
		t.Fatalf("lasciapassare di prima dopo la migrazione: %v", err)
	}
	atteso := model.Lasciapassare{IdModel: prima.IdModel, Nonce: prima.Nonce, Pc: prima.Pc, AgenteId: prima.AgenteId,
		TecnicoId: prima.TecnicoId, Modo: prima.Modo, Capacita: "terminale", Porte: "", Durata: 0, Emesso: prima.Emesso,
		Scade: prima.Scade, ConnId: prima.ConnId, AuditConnId: prima.AuditConnId, Creato: prima.Creato}
	if dopo != atteso {
		t.Errorf("lasciapassare di prima dopo la migrazione:\n%+v\natteso\n%+v", dopo, atteso)
	}
}
