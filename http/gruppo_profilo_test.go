package http

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// TestGruppoNelProfilo prova sul router vero che /api/admin/user/current, la
// risposta del login del pannello, ha il nome del gruppo dell'utente per il
// Profilo: vuoto se il gruppo non c'e'. Se il gruppo non si legge il login
// vale lo stesso, col nome vuoto e un warn nel log.
func TestGruppoNelProfilo(t *testing.T) {
	for _, tc := range []struct {
		nome, gruppo string // gruppo: il nome del gruppo dell'utente, vuoto per un gruppo che non c'e'
		rifiuta      bool
		atteso       string
	}{
		{"gruppo", "Assistenza", false, "Assistenza"},
		{"gruppo assente", "", false, ""},
		{"gruppo non letto", "Assistenza", true, ""},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g, utente, registro := pannello(t, false)
			if err := service.DB.AutoMigrate(&model.Group{}); err != nil {
				t.Fatal(err)
			}
			id := uint(999)
			if tc.gruppo != "" {
				gruppo := &model.Group{Name: tc.gruppo, Type: model.GroupTypeShare}
				crea(t, gruppo)
				id = gruppo.Id
			}
			if err := service.DB.Model(utente).Update("group_id", id).Error; err != nil {
				t.Fatal(err)
			}
			if tc.rifiuta {
				rifiutaLetture(t, "groups", "id = ?")
			}

			rec := richiesta(g, "GET", "/api/admin/user/current", "", "")
			var got struct {
				Code int `json:"code"`
				Data struct {
					Token     string  `json:"token"`
					GroupName *string `json:"group_name"`
				} `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != 200 || got.Code != 0 || got.Data.Token == "" {
				t.Fatalf("GET /api/admin/user/current: %d %s (%v), atteso il login", rec.Code, rec.Body, err)
			}
			if got.Data.GroupName == nil || *got.Data.GroupName != tc.atteso {
				t.Errorf("group_name: %s, atteso %q", rec.Body, tc.atteso)
			}
			if nelLog := strings.Contains(registro.String(), "nome del gruppo dell'utente"); nelLog != tc.rifiuta {
				t.Errorf("warn nel log: %t, atteso %t\n%s", nelLog, tc.rifiuta, registro)
			}
		})
	}
}
