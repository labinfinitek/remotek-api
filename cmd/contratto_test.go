package main

// Test del contratto sul router vero dell'API. Sta in package main solo
// perche' InitGlobal e' qui: e' provvisorio, finche' il bootstrap non esce
// da cmd/ (test/contratto/README.md, sezione Stato).
//
// In questo pacchetto UN SOLO test di primo livello puo' chiamare
// InitGlobal: lo stato globale non ha guardie, e una seconda chiamata
// aprirebbe un altro file di log, avvierebbe un'altra goroutine del limiter
// e sovrascriverebbe i servizi. Niente t.Parallel: t.Chdir e t.Setenv lo
// vietano.

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/global"
	apihttp "github.com/lejianwen/rustdesk-api/v2/http"
	"github.com/lejianwen/rustdesk-api/v2/test/contratto"
)

// TestContract riesegue i golden dei gruppi attivi di test/contratto sul
// router vero, servito da httptest dopo il bootstrap di produzione
// (InitGlobal) in una cartella temporanea.
func TestContract(t *testing.T) {
	// go test parte da cmd/: la radice del repo si prende prima di spostarsi.
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("radice del repo: %v", err)
	}

	// Il codice apre percorsi relativi alla cartella corrente
	// (./data/rustdeskapi.db, ./runtime/log.txt, resources/templates/*,
	// resources/version, resources/i18n, ./conf/admin/hello.html): nella
	// sandbox si risolvono tutti li' dentro e nel repo non si scrive nulla.
	// La cartella corrente resta la sandbox fino alla fine del test, perche'
	// sqlite apre le connessioni quando servono, col percorso relativo.
	sandbox := t.TempDir()
	for _, dir := range []string{"data", "runtime"} {
		if err := os.MkdirAll(filepath.Join(sandbox, dir), 0o750); err != nil {
			t.Fatalf("sandbox: %v", err)
		}
	}
	for _, dir := range []string{"resources", "conf"} {
		if err := os.Symlink(filepath.Join(root, dir), filepath.Join(sandbox, dir)); err != nil {
			t.Fatalf("sandbox: %v", err)
		}
	}
	t.Chdir(sandbox)

	// Configurazione dell'istanza di riferimento da cui vengono i golden,
	// fissata apposta: se cambiano i default nel codice, i golden non si
	// spostano. Per questo il test NON prova i default sicuri, che avranno
	// test propri. Il modo test di gin non stampa nulla e non cambia le
	// risposte. Log a warn: la password casuale di admin creata dalla
	// migrazione va nel log a livello info e non deve finire nel log
	// pubblico della CI.
	t.Setenv("RUSTDESK_API_LANG", "en")
	t.Setenv("RUSTDESK_API_APP_WEB_CLIENT", "0")
	t.Setenv("RUSTDESK_API_APP_SHOW_SWAGGER", "0")
	t.Setenv("RUSTDESK_API_APP_WEB_SSO", "true")
	t.Setenv("RUSTDESK_API_APP_REGISTER", "false")
	t.Setenv("RUSTDESK_API_APP_CAPTCHA_THRESHOLD", "3")
	t.Setenv("RUSTDESK_API_APP_BAN_THRESHOLD", "10")
	t.Setenv("RUSTDESK_API_RUSTDESK_PERSONAL", "1")
	t.Setenv("RUSTDESK_API_GIN_MODE", "test")
	t.Setenv("RUSTDESK_API_LOGGER_LEVEL", "warn")

	global.ConfigPath = filepath.Join(root, "conf", "config.yaml")
	InitGlobal()

	srv := httptest.NewServer(apihttp.NewEngine())
	t.Cleanup(srv.Close)

	contratto.Run(t, contratto.Options{
		BaseURL: srv.URL,
		Golden:  os.DirFS(filepath.Join(root, "test", "contratto", "testdata", "client-1.4.9")),
		Groups:  []string{"anonime"},
	})
}
