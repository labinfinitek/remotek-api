package http

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// TestDestinatariCondivisione prova sul router vero groupUsers, che il
// pannello chiama per scegliere il destinatario di una regola di
// condivisione: gruppi con id e nome, utenti con id, username, nickname e
// gruppo, senza disattivati e account agente, e la stessa risposta per
// amministratori e utenti. Prima dava a chiunque entrasse nel pannello tutti
// gli utenti con email, nota, is_admin, stato, agente_di, avatar e date.
func TestDestinatariCondivisione(t *testing.T) {
	type gruppo struct {
		Id   uint   `json:"id"`
		Name string `json:"name"`
	}
	type utente struct {
		Id       uint   `json:"id"`
		Username string `json:"username"`
		Nickname string `json:"nickname"`
		GroupId  uint   `json:"group_id"`
	}
	type dati struct {
		Groups []gruppo `json:"groups"`
		Users  []utente `json:"users"`
	}
	corpi := map[bool]string{}
	for _, admin := range []bool{false, true} {
		t.Run(map[bool]string{false: "utente", true: "amministratore"}[admin], func(t *testing.T) {
			g, prova, _ := pannello(t, admin)
			if err := service.DB.AutoMigrate(&model.Group{}); err != nil {
				t.Fatal(err)
			}
			ufficio := &model.Group{Name: "ufficio", Type: model.GroupTypeShare}
			crea(t, ufficio)
			attivo := &model.User{Username: "attivo", Nickname: "Mario", Email: "mario@esempio.it", Remark: "nota",
				Avatar: "a.png", GroupId: ufficio.Id, Status: model.COMMON_STATUS_ENABLE}
			crea(t, attivo)
			crea(t, &model.User{Username: "spento", GroupId: ufficio.Id, Status: model.COMMON_STATUS_DISABLED})
			crea(t, &model.User{Username: "agente", GroupId: ufficio.Id, Status: model.COMMON_STATUS_ENABLE, AgenteDi: attivo.Id})

			rec := alPannello(g, "/api/admin/user/groupUsers", "")
			corpo := rec.Body.String()
			var got struct {
				Code int  `json:"code"`
				Data dati `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &got); rec.Code != 200 || err != nil || got.Code != 0 {
				t.Fatalf("admin %v: stato %d, %v, corpo %s", admin, rec.Code, err, corpo)
			}
			want := dati{
				Groups: []gruppo{{ufficio.Id, "ufficio"}},
				Users:  []utente{{prova.Id, "prova", "", 0}, {attivo.Id, "attivo", "Mario", ufficio.Id}},
			}
			if !reflect.DeepEqual(got.Data, want) {
				t.Errorf("admin %v, dati:\n got  %+v\n want %+v", admin, got.Data, want)
			}
			for _, campo := range []string{"email", "remark", "is_admin", "agente_di", "status", "avatar", "created_at", "updated_at", "type"} {
				if strings.Contains(corpo, `"`+campo+`"`) {
					t.Errorf("admin %v, nella risposta c'e' %q: %s", admin, campo, corpo)
				}
			}
			corpi[admin] = corpo
		})
	}
	if corpi[false] != corpi[true] {
		t.Errorf("risposte diverse:\n utente %s\n admin  %s", corpi[false], corpi[true])
	}
}

// TestRegolaVersoEscluso prova sul router vero che una regola di
// condivisione non nasce ne' si sposta verso un utente disattivato o un
// account agente, dall'amministrazione e dalla sezione dell'utente, anche
// per chiamata diretta; la regola gia' salvata verso amico, poi escluso,
// resta e si cancella. Prima groupUsers li toglieva solo dalla scelta del
// pannello e la creazione riusciva.
func TestRegolaVersoEscluso(t *testing.T) {
	const rifiuto = `{"code":101,"message":"Non si condivide con un utente disattivato o con un account agente.","data":null}`
	const ok = `{"code":0,"message":"success","data":null}`
	const nuova = `{"user_id":UTENTE,"collection_id":RUBRICA,"type":1,"rule":1,"to_id":ESCLUSO}`
	const modifica = `{"id":REGOLA,"user_id":UTENTE,"collection_id":RUBRICA,"type":1,"rule":1,"to_id":ESCLUSO}`
	for _, escluso := range []struct {
		nome  string
		campi map[string]any
	}{
		{"disattivato", map[string]any{"status": model.COMMON_STATUS_DISABLED}},
		{"agente", map[string]any{"agente_di": 1}},
	} {
		for _, rotta := range []string{"/api/admin/address_book_collection_rule", "/api/admin/my/address_book_collection_rule"} {
			t.Run(escluso.nome+" "+rotta, func(t *testing.T) {
				g, _, segnaposto := pannelloDiProva(t)
				altro := &model.User{Username: "escluso", Status: model.COMMON_STATUS_ENABLE}
				crea(t, altro)
				for _, u := range []string{"escluso", "amico"} {
					if err := service.DB.Model(&model.User{}).Where("username = ?", u).Updates(escluso.campi).Error; err != nil {
						t.Fatal(err)
					}
				}
				id := strings.NewReplacer("ESCLUSO", strconv.FormatUint(uint64(altro.Id), 10))
				prima := statoDelPannello(t)
				for _, caso := range []struct{ azione, corpo string }{{"/create", nuova}, {"/update", modifica}} {
					corpo := id.Replace(segnaposto.Replace(caso.corpo))
					if rec := richiesta(g, "POST", rotta+caso.azione, "", corpo); rec.Body.String() != rifiuto {
						t.Errorf("POST %s%s: %d %s\n want %s", rotta, caso.azione, rec.Code, rec.Body, rifiuto)
					}
				}
				if dopo := statoDelPannello(t); dopo != prima {
					t.Errorf("regole cambiate dopo il rifiuto:\n prima %s\n dopo  %s", prima, dopo)
				}
				// la regola verso amico, ora escluso, si cancella
				if rec := richiesta(g, "POST", rotta+"/delete", "", segnaposto.Replace(`{"id":REGOLA}`)); rec.Body.String() != ok {
					t.Errorf("POST %s/delete: %d %s", rotta, rec.Code, rec.Body)
				}
			})
		}
	}
}
