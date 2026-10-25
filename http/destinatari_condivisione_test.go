package http

import (
	"encoding/json"
	"reflect"
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
