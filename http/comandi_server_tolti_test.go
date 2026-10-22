package http

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestComandiServerTolti prova sul router vero che le rotte dei comandi al
// server non ci sono piu': per l'amministratore e per un utente qualunque
// rispondono come una rotta che non esiste, cioe' col NoRoute che il client
// conosce dal golden devices-cli-404.
func TestComandiServerTolti(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("..", "test", "contratto", "testdata", "client-1.4.9", "devices-cli-404", "risposta.golden.json"))
	var noRoute struct {
		Stato       int    `json:"stato"`
		ContentType string `json:"content_type"`
		Corpo       string `json:"corpo"`
	}
	if err == nil {
		err = json.Unmarshal(golden, &noRoute)
	}
	if err != nil {
		t.Fatalf("golden del NoRoute: %v", err)
	}
	for _, admin := range []bool{true, false} {
		t.Run(map[bool]string{true: "amministratore", false: "utente"}[admin], func(t *testing.T) {
			comandiTolti(t, admin, noRoute.Stato, noRoute.ContentType, noRoute.Corpo)
		})
	}
}

// comandiTolti prova le rotte dei comandi con il token di un utente,
// amministratore se admin, contro la risposta del NoRoute.
func comandiTolti(t *testing.T, admin bool, stato int, contentType, corpo string) {
	g, _, _ := pannello(t, admin) // pannello si sposta nella radice del repo
	for _, r := range []struct{ metodo, rotta string }{
		{"POST", "/api/admin/rustdesk/sendCmd"},
		{"GET", "/api/admin/rustdesk/cmdList"},
		{"POST", "/api/admin/rustdesk/cmdDelete"},
		{"POST", "/api/admin/rustdesk/cmdCreate"},
		{"POST", "/api/admin/rustdesk/cmdUpdate"},
	} {
		rec := conToken(g, r.metodo, r.rotta, tokenDelPannello)
		if rec.Code != stato || rec.Header().Get("Content-Type") != contentType || rec.Body.String() != corpo {
			t.Errorf("%s %s: stato %d, Content-Type %q, corpo %q; atteso il NoRoute: %d, %q, %q", r.metodo, r.rotta,
				rec.Code, rec.Header().Get("Content-Type"), rec.Body.String(), stato, contentType, corpo)
		}
	}
}
