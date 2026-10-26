package http

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// TestLasciapassareCapacita prova sul router vero le capacita' di POST
// /api/agente/lasciapassare (ADR-0033): senza campi, o col terminale, il
// payload e' il v 1 di prima, byte per byte; con le altre e' il v 2, con
// capacita, porte e durata dopo modo (col tunnel 3600 se manca). Il
// registro ha capacita', porte e durata, l'evento agente.lasciapassare
// agente.capacita e, per il tunnel, agente.porte e agente.durata.
func TestLasciapassareCapacita(t *testing.T) {
	for _, tc := range []struct {
		caso, campi, capacita, porte, evento string
		durata                               int
	}{
		{"senza campi", ``, "terminale", "", `,"capacita":"terminale"}`, 0},
		{"terminale", `,"capacita":"terminale"`, "terminale", "", `,"capacita":"terminale"}`, 0},
		{"exec", `,"capacita":"exec"`, "exec", "", `,"capacita":"exec"}`, 0},
		{"file", `,"capacita":"file"`, "file", "", `,"capacita":"file"}`, 0},
		{"schermo", `,"capacita":"schermo"`, "schermo", "", `,"capacita":"schermo"}`, 0},
		{"tunnel", `,"capacita":"tunnel","porte":[3389,22]`, "tunnel", "3389,22", `,"capacita":"tunnel","porte":[3389,22],"durata":3600}`, 3600},
		{"tunnel con durata", `,"capacita":"tunnel","porte":[22],"durata":28800`, "tunnel", "22", `,"capacita":"tunnel","porte":[22],"durata":28800}`, 28800},
	} {
		t.Run(tc.caso, func(t *testing.T) {
			g, pub, _ := lasciapassareDiProva(t, true)
			rec := daTecnico(g, "POST", "/api/agente/lasciapassare", "tok-agente-mario", `{"id":"999000111"`+tc.campi+`}`)
			var res struct{ Lasciapassare string }
			if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &res) != nil {
				t.Fatalf("%d %s", rec.Code, rec.Body)
			}
			p := payload(t, pub, res.Lasciapassare)
			pb, _ := base64.RawURLEncoding.DecodeString(strings.Split(res.Lasciapassare, ".")[0])
			want := fmt.Sprintf(`{"v":1,"pc":"999000111","agente":"agente-mario","tecnico":"mario","modo":"amministratore",`+
				`"emesso":%d,"scade":%d,"nonce":"%s"}`, p.Emesso, p.Scade, p.Nonce)
			if tc.capacita != "terminale" {
				porte := "[" + tc.porte + "]"
				want = fmt.Sprintf(`{"v":2,"pc":"999000111","agente":"agente-mario","tecnico":"mario","modo":"amministratore",`+
					`"capacita":"%s","porte":%s,"durata":%d,"emesso":%d,"scade":%d,"nonce":"%s"}`, tc.capacita, porte, tc.durata, p.Emesso, p.Scade, p.Nonce)
			}
			if string(pb) != want {
				t.Errorf("payload\n got  %s\n want %s", pb, want)
			}
			var r model.Lasciapassare
			if err := service.DB.First(&r).Error; err != nil || r.Capacita != tc.capacita || r.Porte != tc.porte || r.Durata != tc.durata {
				t.Errorf("registro %+v (%v)", r, err)
			}
			var e model.Evento
			if err := service.DB.First(&e).Error; err != nil ||
				!strings.Contains(e.Corpo, `"agente":{"agente":"agente-mario","tecnico":"mario","amministratore":true`+tc.evento) {
				t.Errorf("evento %s (%v)", e.Corpo, err)
			}
		})
	}
}

// TestLasciapassarePorteRifiutate prova sul router vero che porte e
// capacita' fuori regola danno 400 col messaggio tradotto, senza registro
// ne' evento: tunnel senza porte o con [], porte fuori da 1-65535, doppie
// o piu' di 8, durata fuori da 60-28800, porte o durata con un'altra
// capacita', capacita' fuori elenco.
func TestLasciapassarePorteRifiutate(t *testing.T) {
	for _, tc := range []struct{ caso, campi string }{
		{"tunnel senza porte", `"capacita":"tunnel"`},
		{"tunnel con porte vuote", `"capacita":"tunnel","porte":[]`},
		{"porta 0", `"capacita":"tunnel","porte":[0]`},
		{"porta 65536", `"capacita":"tunnel","porte":[22,65536]`},
		{"porta doppia", `"capacita":"tunnel","porte":[22,3389,22]`},
		{"nove porte", `"capacita":"tunnel","porte":[1,2,3,4,5,6,7,8,9]`},
		{"durata 59", `"capacita":"tunnel","porte":[22],"durata":59`},
		{"durata 28801", `"capacita":"tunnel","porte":[22],"durata":28801`},
		{"porte con exec", `"capacita":"exec","porte":[22]`},
		{"durata con file", `"capacita":"file","durata":3600}`},
		{"porte senza capacita", `"porte":[22]`},
		{"capacita fuori elenco", `"capacita":"shell"`},
	} {
		t.Run(tc.caso, func(t *testing.T) {
			g, _, _ := lasciapassareDiProva(t, true)
			rec := daTecnico(g, "POST", "/api/agente/lasciapassare", "tok-agente-mario", `{"id":"999000111",`+tc.campi+`}`)
			if rec.Code != 400 || rec.Body.String() != `{"error":"Parametri non validi."}` {
				t.Errorf("%d %s", rec.Code, rec.Body)
			}
			var n, m int64
			if err := service.DB.Model(&model.Lasciapassare{}).Count(&n).Error; err != nil || n != 0 {
				t.Errorf("%d lasciapassare registrati (%v)", n, err)
			}
			if err := service.DB.Model(&model.Evento{}).Count(&m).Error; err != nil || m != 0 {
				t.Errorf("%d eventi (%v)", m, err)
			}
		})
	}
}
