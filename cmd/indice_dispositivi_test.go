package main

import (
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/lejianwen/rustdesk-api/v2/lib/logger"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// indiceUnico dice se peers ha l'indice service.IndiceIdUnico, unico.
func indiceUnico(t *testing.T, db *gorm.DB) bool {
	t.Helper()
	var n int64
	err := db.Raw(`SELECT count(*) FROM pragma_index_list('peers') WHERE name = ? AND "unique" = 1`, service.IndiceIdUnico).Scan(&n).Error
	if err != nil {
		t.Fatal(err)
	}
	return n == 1
}

// TestIndiceUnicoDispositivi prova sul binario vero che dopo l'avvio peers
// ha l'indice unico su id se non ci sono ID doppi. Con un ID su due righe
// l'avvio riesce, l'indice non c'e', nessuna riga si cancella, e il log ha
// una riga error col numero degli ID doppi, senza ID, uuid o nomi; tolta la
// riga in piu', l'avvio dopo crea l'indice. Prima l'indice non c'era mai.
func TestIndiceUnicoDispositivi(t *testing.T) {
	dir := sandbox(t)
	if codice, out := esegui(t, dir); codice != 0 {
		t.Fatalf("primo avvio: codice %d\n%s", codice, out)
	}
	db := apriDB(t, dir)
	if !indiceUnico(t, db) {
		t.Fatalf("dopo il primo avvio, database senza doppioni: manca l'indice unico %s", service.IndiceIdUnico)
	}

	// Un database di prima, senza l'indice e con un ID su due righe.
	if err := db.Exec("DROP INDEX " + service.IndiceIdUnico).Error; err != nil {
		t.Fatal(err)
	}
	pcs := []*model.Peer{
		{Id: "999000111", Uuid: "dXVpZA==", Hostname: "PC-VERO", Username: "mario.rossi"},
		{Id: "999000111", Uuid: "YWx0cm8tdXVpZA==", Hostname: "PC-DOPPIO", Username: "anna.bianchi"},
		{Id: "999000222", Uuid: "dGVyem8=", Hostname: "PC-TERZO", Username: "luca.verdi"},
	}
	if err := db.Create(&pcs).Error; err != nil {
		t.Fatal(err)
	}

	codice, out := esegui(t, dir)
	if codice != 0 {
		t.Fatalf("avvio con un ID doppio: codice %d, atteso 0\n%s", codice, out)
	}
	if indiceUnico(t, db) {
		t.Errorf("avvio con un ID doppio: l'indice %s c'e'", service.IndiceIdUnico)
	}
	riga := "peers ha 1 ID di PC su piu' righe: l'indice unico " + service.IndiceIdUnico + " non si crea"
	if n := logger.Conta(out, "ERROR", riga); n != 1 {
		t.Errorf("%d righe ERROR %q, attesa 1\n%s", n, riga, out)
	}
	for _, pc := range pcs {
		for _, s := range []string{pc.Id, pc.Uuid, pc.Hostname, pc.Username} {
			if strings.Contains(out, s) {
				t.Errorf("nel log c'e' %q\n%s", s, out)
			}
		}
	}
	var righe int64
	if err := db.Model(&model.Peer{}).Count(&righe).Error; err != nil || righe != 3 {
		t.Errorf("righe di peers dopo l'avvio: %d (%v), attese 3", righe, err)
	}

	// Cancellata dal pannello la riga in piu', l'avvio dopo crea l'indice.
	if err := db.Delete(pcs[1]).Error; err != nil {
		t.Fatal(err)
	}
	if codice, out := esegui(t, dir); codice != 0 {
		t.Fatalf("avvio senza piu' doppioni: codice %d\n%s", codice, out)
	} else if logger.Conta(out, "ERROR") != 0 {
		t.Errorf("avvio senza piu' doppioni: riga error\n%s", out)
	}
	if !indiceUnico(t, db) {
		t.Errorf("avvio senza piu' doppioni: manca l'indice unico %s", service.IndiceIdUnico)
	}
}
