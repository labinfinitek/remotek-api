package service

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"gorm.io/gorm/logger"

	"github.com/lejianwen/rustdesk-api/v2/config"
	"github.com/lejianwen/rustdesk-api/v2/lib/orm"
	"github.com/lejianwen/rustdesk-api/v2/model"
)

// TestFirmaUuidDiverso: con la firma non obbligatoria, un sysinfo firmato
// con pk e un uuid che non e' quello salvato non registra niente, quindi la
// firma non si verifica e non entra nella memoria delle firme viste; il
// motivo dice se il PC ha un altro uuid o nessuno (creato dal pannello). Un
// sysinfo firmato dello stesso PC invece si verifica e si ricorda.
func TestFirmaUuidDiverso(t *testing.T) {
	db, err := orm.ApriSqlite(filepath.Join(t.TempDir(), "api.db"), logger.Discard)
	if err == nil {
		err = db.AutoMigrate(&model.Peer{})
	}
	if err == nil {
		err = db.Create([]*model.Peer{{Id: "999000111", Uuid: "dXVpZA=="}, {Id: "999000222"}}).Error
	}
	if err != nil {
		t.Fatal(err)
	}
	precDB, precConfig := DB, Config
	DB, Config = db, &config.Config{}
	t.Cleanup(func() { DB, Config = precDB, precConfig })

	pub, k, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pk := base64.StdEncoding.EncodeToString(pub)
	firmata := func(i int) RichiestaFirmata {
		corpo := []byte(`{"n":` + strconv.Itoa(i) + `}`)
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		firma := base64.StdEncoding.EncodeToString(ed25519.Sign(k, MessaggioFirma("POST", "/api/sysinfo", ts, corpo)))
		return RichiestaFirmata{Metodo: "POST", Percorso: "/api/sysinfo", Intestazione: ts + "." + firma, Corpo: corpo, Pk: pk}
	}
	viste := func() int {
		firmeViste.mu.Lock()
		defer firmeViste.mu.Unlock()
		return len(firmeViste.scadenza)
	}

	prima := viste()
	for i, tc := range []struct{ id, motivo string }{
		{"999000111", "uuid diverso da quello salvato"},
		{"999000222", "PC senza uuid, in attesa del primo sysinfo"},
	} {
		for j := range 20 {
			f, err := (&PeerService{}).RiconosciFirmato(tc.id, "YWx0cm8=", firmata(100*i+j))
			if err != nil || f.Esito != UuidDiverso || f.NonRegistrata != tc.motivo || f.Rifiuto != "" || f.Chiave != "" {
				t.Fatalf("PC %s, uuid diverso: %+v %v; atteso UuidDiverso con motivo %q", tc.id, f, err, tc.motivo)
			}
		}
	}
	if got := viste(); got != prima {
		t.Errorf("firme viste da %d a %d con 40 sysinfo di un uuid diverso", prima, got)
	}
	f, err := (&PeerService{}).RiconosciFirmato("999000111", "dXVpZA==", firmata(1000))
	if err != nil || f.Chiave != pk || viste() != prima+1 {
		t.Errorf("sysinfo firmato dello stesso PC: %+v %v, firme viste %d, attese %d", f, err, viste(), prima+1)
	}
}
