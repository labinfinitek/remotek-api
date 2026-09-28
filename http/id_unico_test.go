package http

import (
	"strconv"
	"strings"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// TestPannelloIdDispositivo prova sul router vero che il pannello non crea
// un PC senza ID o con l'ID di un altro PC, e non da' a un PC l'ID vuoto o
// quello di un altro: risponde code 101 col messaggio del validatore o con
// PeerIdExists, nella lingua di Accept-Language, e peers resta com'era. Un
// PC che tiene il suo ID si modifica, e un ID libero si crea. Prima il
// pannello salvava tutto, e un ID su due righe rendeva ambiguo il legame
// ID-uuid.
func TestPannelloIdDispositivo(t *testing.T) {
	g, _, _ := pannello(t, true)
	if err := service.DB.AutoMigrate(&model.Peer{}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.AllService.PeerService.CreaIndiceIdUnico(); err != nil {
		t.Fatal(err)
	}
	primo := &model.Peer{Id: "999000111", Uuid: uuidSalvato, Hostname: "PC-UNO"}
	crea(t, primo)
	secondo := &model.Peer{Id: "999000222", Uuid: uuidAltro, Hostname: "PC-DUE"}
	crea(t, secondo)
	modulo := func(rowId uint, id string) string {
		return `{"row_id": ` + strconv.FormatUint(uint64(rowId), 10) + `, "id": "` + id + `", "hostname": "PC", "os": "windows"}`
	}
	const (
		usatoIt = `{"code":101,"message":"Un altro PC ha già questo ID.","data":null}`
		usatoEn = `{"code":101,"message":"Another PC already has this ID.","data":null}`
		vuotoIt = `{"code":101,"message":"ID del PC è un campo obbligatorio","data":null}`
		vuotoEn = `{"code":101,"message":"PC ID is a required field","data":null}`
		fatto   = `{"code":0,"message":"success","data":null}`
	)
	for _, tc := range []struct{ rotta, lingua, corpo, risposta string }{
		{"/api/admin/peer/create", "", modulo(0, ""), vuotoIt},
		{"/api/admin/peer/create", "en", modulo(0, ""), vuotoEn},
		{"/api/admin/peer/create", "", modulo(0, "999000111"), usatoIt},
		{"/api/admin/peer/create", "en", modulo(0, "999000111"), usatoEn},
		{"/api/admin/peer/update", "", modulo(secondo.RowId, ""), vuotoIt},
		{"/api/admin/peer/update", "", modulo(secondo.RowId, "999000111"), usatoIt},
		{"/api/admin/peer/update", "", modulo(primo.RowId, "999000111"), fatto},
		{"/api/admin/peer/create", "", modulo(0, "999000333"), fatto},
	} {
		if got := richiesta(g, "POST", tc.rotta, tc.lingua, tc.corpo).Body.String(); got != tc.risposta {
			t.Errorf("POST %s %s:\n got  %s\n want %s", tc.rotta, tc.corpo, got, tc.risposta)
		}
	}
	var ids []string
	if err := service.DB.Model(&model.Peer{}).Order("row_id").Pluck("id", &ids).Error; err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(ids, " "), "999000111 999000222 999000333"; got != want {
		t.Errorf("ID in peers: %s, attesi %s", got, want)
	}
}
