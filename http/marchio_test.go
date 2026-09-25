package http

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// get manda una GET a rotta, con l'api-token del pannello se token.
func get(g *gin.Engine, rotta string, token bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", rotta, nil)
	if token {
		req.Header.Set("api-token", tokenDelPannello)
	}
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	return rec
}

// TestMarchio prova sul router vero che il nome del marchio, "Remotek" senza
// configurazione o RUSTDESK_API_BRAND_NAME, sia il titolo che il pannello
// legge da /api/admin/config/admin, entri al posto di {{brand}} nel
// benvenuto di conf/admin/hello.html e nel titolo delle due pagine del login
// OAuth: quella di errore da /api/oidc/callback senza state, quella di
// successo dai modelli del router, perche' arrivarci vuole un provider.
func TestMarchio(t *testing.T) {
	for variabile, nome := range map[string]string{"": "Remotek", "Acme Assist": "Acme Assist"} {
		t.Run(nome, func(t *testing.T) {
			t.Setenv("RUSTDESK_API_BRAND_NAME", variabile)
			t.Setenv("RUSTDESK_API_ADMIN_TITLE", "")
			t.Setenv("RUSTDESK_API_ADMIN_HELLO", "")
			g, _, _ := pannello(t, false)
			for token, want := range map[bool]string{
				false: `{"code":0,"message":"success","data":{"title":"` + nome + `"}}`,
				true:  `{"code":0,"message":"success","data":{"hello":"### 👏👏👏 Ciao ***prova***, benvenuto in ` + nome + `","title":"` + nome + `"}}`,
			} {
				if rec := get(g, "/api/admin/config/admin", token); rec.Code != 200 || rec.Body.String() != want {
					t.Errorf("GET /api/admin/config/admin (token %v): %d %s\nwant 200 %s", token, rec.Code, rec.Body, want)
				}
			}
			succ := httptest.NewRecorder()
			if err := g.HTMLRender.Instance("oauth_success.html", gin.H{"message": "OauthSuccess"}).Render(succ); err != nil {
				t.Fatalf("oauth_success.html: %v", err)
			}
			for pagina, corpo := range map[string]string{"oauth_fail.html": get(g, "/api/oidc/callback", false).Body.String(), "oauth_success.html": succ.Body.String()} {
				for _, atteso := range []string{" - " + nome + "</title>", "document.title = title + ' - " + nome + "';"} {
					if !strings.Contains(corpo, atteso) || strings.Contains(corpo, "RustDesk API") {
						t.Errorf("%s: manca %q o c'e' ancora \"RustDesk API\"", pagina, atteso)
					}
				}
			}
		})
	}
}

// TestMarchioFile prova che logo e favicon di brand.dir si servano su
// /brand/ cosi' come sono, come SVG, e che la cartella non si elenchi.
func TestMarchioFile(t *testing.T) {
	t.Setenv("RUSTDESK_API_BRAND_DIR", "")
	g, _, _ := pannello(t, false)
	for _, nome := range []string{"logo.svg", "favicon.svg"} {
		file, err := os.ReadFile("resources/brand/" + nome)
		if err != nil {
			t.Fatal(err)
		}
		rec := get(g, "/brand/"+nome, false)
		if rec.Code != 200 || rec.Body.String() != string(file) || rec.Header().Get("Content-Type") != "image/svg+xml" {
			t.Errorf("GET /brand/%s: stato %d, Content-Type %q, corpo uguale al file %v", nome, rec.Code, rec.Header().Get("Content-Type"), rec.Body.String() == string(file))
		}
	}
	if rec := get(g, "/brand/", false); rec.Code != 404 || strings.Contains(rec.Body.String(), "logo.svg") {
		t.Errorf("GET /brand/: stato %d, atteso 404 senza elenco:\n%s", rec.Code, rec.Body)
	}
}
