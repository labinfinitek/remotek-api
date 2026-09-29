package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/lib/logger"
	"github.com/lejianwen/rustdesk-api/v2/model"
)

// TestAvvioProviderNonSupportati prova sul binario vero che l'avvio con
// provider OAuth di tipo github, google e linuxdo nel database (A3) riesca e
// scriva un warn per ciascuno, col suo op; nessun warn per il provider oidc
// e nessuna riga persa.
func TestAvvioProviderNonSupportati(t *testing.T) {
	dir := sandbox(t)
	codice, out := esegui(t, dir)
	if codice != 0 {
		t.Fatalf("primo avvio: codice %d\n%s", codice, out)
	}
	if strings.Contains(out, "provider OAuth") {
		t.Errorf("primo avvio, database senza provider: warn inatteso\n%s", out)
	}
	provider := []*model.Oauth{
		{Op: "github", OauthType: "github"},
		{Op: "google-aziendale", OauthType: "google"},
		{Op: "linuxdo", OauthType: "linuxdo"},
		{Op: "aziendale", OauthType: model.OauthTypeOidc},
	}
	db := apriDB(t, dir)
	if err := db.Create(&provider).Error; err != nil {
		t.Fatal(err)
	}

	codice, out = esegui(t, dir)
	if codice != 0 {
		t.Fatalf("avvio coi provider dei tipi tolti: codice %d, atteso 0\n%s", codice, out)
	}
	for _, p := range provider[:3] {
		riga := `provider OAuth "` + p.Op + `" di tipo "` + p.OauthType + `" ignorato: il login usa solo il tipo oidc`
		if n := logger.Conta(out, "WARN", riga); n != 1 {
			t.Errorf("%d righe WARN %q, attesa 1\n%s", n, riga, out)
		}
	}
	for _, r := range logger.Righe(out) {
		if strings.Contains(r.Msg, `"aziendale"`) {
			t.Errorf("riga per il provider oidc\n%s", out)
		}
	}
	var ops []string
	if err := db.Model(&model.Oauth{}).Order("id").Pluck("op", &ops).Error; err != nil {
		t.Fatal(err)
	}
	if want := []string{"github", "google-aziendale", "linuxdo", "aziendale"}; !slices.Equal(ops, want) {
		t.Errorf("provider nel database dopo l'avvio: %q, attesi %q", ops, want)
	}
}
