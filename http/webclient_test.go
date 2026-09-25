package http

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestSenzaWebClient prova sul router vero che il web client non c'e' piu',
// anche con RUSTDESK_API_APP_WEB_CLIENT=1: le sue pagine, le rotte che lo
// servivano e le condivisioni del pannello rispondono come una rotta che non
// esiste, cioe' col NoRoute che il client conosce dal golden
// devices-cli-404. /api/admin/config/app manda ancora web_client, fisso a 0,
// perche' il pannello non mostri le voci del web client.
func TestSenzaWebClient(t *testing.T) {
	t.Setenv("RUSTDESK_API_APP_WEB_CLIENT", "1")
	g, _, _ := pannello(t, true) // pannello si sposta nella radice del repo

	var noRoute struct {
		Stato       int    `json:"stato"`
		ContentType string `json:"content_type"`
		Corpo       string `json:"corpo"`
	}
	golden, err := os.ReadFile(filepath.Join("test", "contratto", "testdata", "client-1.4.9", "devices-cli-404", "risposta.golden.json"))
	if err == nil {
		err = json.Unmarshal(golden, &noRoute)
	}
	if err != nil {
		t.Fatalf("golden del NoRoute: %v", err)
	}

	for _, r := range []struct{ metodo, rotta string }{
		{"GET", "/webclient/"},
		{"GET", "/webclient2/"},
		{"GET", "/webclient-config/index.js"},
		{"POST", "/api/shared-peer"},
		{"POST", "/api/server-config"},
		{"POST", "/api/server-config-v2"},
		{"POST", "/api/admin/address_book/shareByWebClient"},
		{"GET", "/api/admin/share_record/list"},
		{"GET", "/api/admin/my/share_record/list"},
	} {
		rec := conToken(g, r.metodo, r.rotta, tokenDelPannello)
		if rec.Code != noRoute.Stato || rec.Header().Get("Content-Type") != noRoute.ContentType || rec.Body.String() != noRoute.Corpo {
			t.Errorf("%s %s: stato %d, Content-Type %q, corpo %q; atteso il NoRoute: %d, %q, %q", r.metodo, r.rotta,
				rec.Code, rec.Header().Get("Content-Type"), rec.Body.String(), noRoute.Stato, noRoute.ContentType, noRoute.Corpo)
		}
	}

	rec := conToken(g, "GET", "/api/admin/config/app", tokenDelPannello)
	if got, want := rec.Body.String(), `{"code":0,"message":"success","data":{"web_client":0}}`; rec.Code != 200 || got != want {
		t.Errorf("GET /api/admin/config/app: stato %d\n got  %s\n want %s", rec.Code, got, want)
	}
}
