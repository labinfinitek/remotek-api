package http

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// indirizzoEsterno trova un indirizzo assoluto (http://, https:// o che
// comincia con //) in src, href, url() o @import: il browser lo chiederebbe
// a un altro sito.
var indirizzoEsterno = regexp.MustCompile(`(?i)(\b(src|href)\s*=\s*|url\(|@import\s+(url\()?)\s*["']?\s*(https?:)?//`)

// esterni restituisce i pezzi di testo che fanno caricare una risorsa esterna.
func esterni(testo string) []string {
	return indirizzoEsterno.FindAllString(testo, -1)
}

// TestEsterni prova che il controllo riconosca i casi che deve fermare, tra
// cui il Font Awesome da CDN che le pagine OAuth caricavano, e lasci passare
// gli indirizzi relativi, javascript: e data:.
func TestEsterni(t *testing.T) {
	for testo, esterno := range map[string]bool{
		`<link rel="stylesheet" href="https://lf9-cdn-tos.bytecdntp.com/cdn/all.min.css">`: true,
		`<script src='http://esempio.it/a.js'></script>`:                                   true,
		`<img src=//esempio.it/a.png>`:                                                     true,
		`s.src = "//esempio.it/a.js";`:                                                     true,
		`background: url( "https://esempio.it/a.png")`:                                     true,
		`@import url(//esempio.it/a.css);`:                                                 true,
		`@import "https://esempio.it/a.css";`:                                              true,
		`<a href="javascript:window.close()">`:                                             false,
		`s.src = '/api/oidc/msg?lang=' + encodeURIComponent(lang);`:                        false,
		`background: url(data:image/png;base64,AAAA)`:                                      false,
		`<path d="M12 3 22 20.5H2Z"/>`:                                                     false,
	} {
		if got := len(esterni(testo)) > 0; got != esterno {
			t.Errorf("esterni(%q): trovato %v, atteso %v", testo, got, esterno)
		}
	}
}

// TestPagineOAuthSenzaRisorseEsterne prova che i modelli di
// resources/templates, le pagine del login OAuth, non carichino niente da
// altri siti: il browser di chi le apre manderebbe il suo IP a terzi e la
// pagina dipenderebbe da un file che non controlliamo.
func TestPagineOAuthSenzaRisorseEsterne(t *testing.T) {
	cartella := filepath.Join("..", "resources", "templates")
	voci, err := os.ReadDir(cartella)
	if err != nil {
		t.Fatal(err)
	}
	if len(voci) == 0 {
		t.Fatalf("%s: nessun modello", cartella)
	}
	for _, voce := range voci {
		testo, err := os.ReadFile(filepath.Join(cartella, voce.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if trovati := esterni(string(testo)); len(trovati) > 0 {
			t.Errorf("%s: risorse esterne %q", voce.Name(), trovati)
		}
	}
}
