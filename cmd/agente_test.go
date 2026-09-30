package main

import (
	"regexp"
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/utils"
)

// TestComandoAgenteAi prova agente-ai sul binario vero: crea l'agente nel
// gruppo del tecnico, legato a lui e non amministratore, e stampa la sua
// password di 24 caratteri una volta, fuori dalle righe di log; rifiuta con
// codice 1, senza creare utenti, uno username che c'e' gia', un tecnico
// che non c'e', un tecnico che e' a sua volta un agente e un tecnico
// disattivato.
func TestComandoAgenteAi(t *testing.T) {
	dir := sandbox(t)
	db := apriDB(t, dir)
	if codice, out := esegui(t, dir); codice != 0 {
		t.Fatalf("primo avvio: codice %d\n%s", codice, out)
	}
	no := false
	mario := &model.User{Username: "mario", GroupId: 7, IsAdmin: &no, Status: model.COMMON_STATUS_ENABLE}
	if err := db.Create(mario).Error; err != nil {
		t.Fatal(err)
	}
	spento := &model.User{Username: "spento", GroupId: 7, IsAdmin: &no, Status: model.COMMON_STATUS_DISABLED}
	if err := db.Create(spento).Error; err != nil {
		t.Fatal(err)
	}

	codice, out := esegui(t, dir, "agente-ai", "agente-mario", "mario")
	if codice != 0 {
		t.Fatalf("agente-ai: codice %d\n%s", codice, out)
	}
	senzaTraccia(t, dir)
	pwd := regexp.MustCompile(`(?m)^[A-Za-z0-9]{24}$`).FindAllString(out, -1)
	if len(pwd) != 1 || strings.Count(out, pwd[0]) != 1 {
		t.Fatalf("attesa una riga con la password di 24 caratteri, una volta sola:\n%s", out)
	}
	a := &model.User{}
	if err := db.Where("username = ?", "agente-mario").First(a).Error; err != nil {
		t.Fatal(err)
	}
	if a.AgenteDi != mario.Id || a.GroupId != 7 || a.IsAdmin == nil || *a.IsAdmin || a.Status != model.COMMON_STATUS_ENABLE {
		t.Errorf("agente salvato: agente_di %d, gruppo %d, admin %v, stato %d; attesi %d, 7, false, 1", a.AgenteDi, a.GroupId, a.IsAdmin, a.Status, mario.Id)
	}
	if ok, err := utils.VerifyPassword(a.Password, pwd[0]); !ok || err != nil {
		t.Errorf("l'agente non ha la password stampata (%v)", err)
	}

	for _, p := range []struct {
		caso   string
		args   []string
		uscita string
	}{
		{"username che c'e' gia'", []string{"agente-mario", "mario"}, "non creato"},
		{"tecnico che non c'e'", []string{"agente-nuovo", "nessuno"}, "tecnico \\\"nessuno\\\" non trovato"},
		{"tecnico agente", []string{"agente-nuovo", "agente-mario"}, "e' un agente AI, non una persona"},
		{"username di un carattere", []string{"a", "mario"}, "da 2 a 32 caratteri"},
		{"tecnico disattivato", []string{"agente-nuovo", "spento"}, "tecnico \\\"spento\\\" e' disattivato"},
	} {
		prima := utenti(t, db)
		codice, out := esegui(t, dir, append([]string{"agente-ai"}, p.args...)...)
		dopo := utenti(t, db)
		if codice != 1 || !strings.Contains(out, p.uscita) || dopo != prima {
			t.Errorf("%s: codice %d, utenti da %d a %d; attesi 1, nessun utente nuovo e %q\n%s", p.caso, codice, prima, dopo, p.uscita, out)
		}
	}
}

// utenti conta gli utenti nel database del figlio.
func utenti(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&model.User{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}
