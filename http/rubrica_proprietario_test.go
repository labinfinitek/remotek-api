package http

import (
	"strconv"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// TestPannelloRubricaNonPassaAdAltri prova sul router vero che un utente del
// pannello non amministratore che modifica una sua rubrica con lo user_id di
// un altro utente nel corpo cambia il nome ma la rubrica resta sua; senza
// user_id nel corpo la modifica va come prima. Prima la rubrica, con le voci
// dentro, passava all'altro utente, che se la trovava nel client.
func TestPannelloRubricaNonPassaAdAltri(t *testing.T) {
	g, utente, _ := pannello(t, false)
	if err := service.DB.AutoMigrate(&model.AddressBook{}, &model.AddressBookCollection{}, &model.AddressBookCollectionRule{}); err != nil {
		t.Fatal(err)
	}
	tecnico := &model.User{Username: "tecnico", Status: model.COMMON_STATUS_ENABLE}
	crea(t, tecnico)
	ufficio := &model.AddressBookCollection{UserId: utente.Id, Name: "ufficio"}
	crea(t, ufficio)
	const fatto = `{"code":0,"message":"success","data":null}`
	id := strconv.FormatUint(uint64(ufficio.Id), 10)
	for _, tc := range []struct{ corpo, nome string }{
		{`{"id": ` + id + `, "name": "clienti", "user_id": ` + strconv.FormatUint(uint64(tecnico.Id), 10) + `}`, "clienti"},
		{`{"id": ` + id + `, "name": "fornitori"}`, "fornitori"},
	} {
		if got := richiesta(g, "POST", "/api/admin/my/address_book_collection/update", "", tc.corpo).Body.String(); got != fatto {
			t.Errorf("POST %s:\n got  %s\n want %s", tc.corpo, got, fatto)
		}
		ex := &model.AddressBookCollection{}
		if err := service.DB.First(ex, ufficio.Id).Error; err != nil {
			t.Fatal(err)
		}
		if ex.UserId != utente.Id || ex.Name != tc.nome {
			t.Errorf("dopo %s: rubrica %q dell'utente %d, attesa %q dell'utente %d", tc.corpo, ex.Name, ex.UserId, tc.nome, utente.Id)
		}
	}
}
