package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sicuri raccoglie le opzioni che ADR-0008 vuole sicure senza configurazione.
type sicuri struct {
	WebClient        int
	WebSso           bool
	Register         bool
	ShowSwagger      int
	CaptchaThreshold int
	BanThreshold     int
	TrustProxy       string
}

// TestDefaultSicuri verifica che, senza variabili RUSTDESK_API_*, un file
// senza le chiavi di ADR-0008 e il conf/config.yaml del repo diano gli
// stessi valori sicuri. Init usa il viper globale: niente t.Parallel, e ogni
// caso rilegge il suo file da capo.
func TestDefaultSicuri(t *testing.T) {
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if !strings.HasPrefix(name, "RUSTDESK_API_") {
			continue
		}
		t.Setenv(name, "") // a fine test torna il valore di prima
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("Unsetenv(%s): %v", name, err)
		}
	}

	want := sicuri{WebClient: 0, WebSso: false, Register: false, ShowSwagger: 0,
		CaptchaThreshold: 3, BanThreshold: 10, TrustProxy: ""}
	for _, tc := range []struct{ name, path string }{
		{"file senza le chiavi", fileSenzaChiavi(t)},
		{"conf/config.yaml", filepath.Join("..", "conf", "config.yaml")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := leggi(tc.path); got != want {
				t.Errorf("Init(%s):\ngot  %+v\nwant %+v", tc.path, got, want)
			}
		})
	}
}

// TestVariabiliBattonoDefault verifica che i default non blocchino la
// configurazione: una variabile RUSTDESK_API_* vale anche per una chiave che
// il file non nomina, per esempio gin.trust-proxy dietro un reverse proxy.
func TestVariabiliBattonoDefault(t *testing.T) {
	t.Setenv("RUSTDESK_API_APP_BAN_THRESHOLD", "0")
	t.Setenv("RUSTDESK_API_GIN_TRUST_PROXY", "192.0.2.1")
	got := leggi(fileSenzaChiavi(t))
	if got.BanThreshold != 0 || got.TrustProxy != "192.0.2.1" {
		t.Errorf("ban-threshold %d, trust-proxy %q; want 0, %q", got.BanThreshold, got.TrustProxy, "192.0.2.1")
	}
}

// fileSenzaChiavi scrive un config.yaml con le sezioni app e gin ma senza le
// chiavi di ADR-0008, come quello di chi configura solo cio' che gli serve.
func fileSenzaChiavi(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("app:\n  token-expire: 168h\ngin:\n  mode: release\n"), 0o600); err != nil {
		t.Fatalf("file di prova: %v", err)
	}
	return path
}

// leggi carica path con Init e ne estrae le opzioni di ADR-0008.
func leggi(path string) sicuri {
	var c Config
	Init(&c, path)
	return sicuri{
		WebClient: c.App.WebClient, WebSso: c.App.WebSso, Register: c.App.Register,
		ShowSwagger: c.App.ShowSwagger, CaptchaThreshold: c.App.CaptchaThreshold,
		BanThreshold: c.App.BanThreshold, TrustProxy: c.Gin.TrustProxy,
	}
}

// TestLinguaPredefinita verifica che senza RUSTDESK_API_LANG la lingua sia
// l'italiano, col conf/config.yaml del repo e con un file che non la nomina:
// e' la lingua dei messaggi al client RustDesk, che non manda
// Accept-Language.
func TestLinguaPredefinita(t *testing.T) {
	t.Setenv("RUSTDESK_API_LANG", "") // vuota, per viper non c'e'
	for _, path := range []string{fileSenzaChiavi(t), filepath.Join("..", "conf", "config.yaml")} {
		var c Config
		Init(&c, path)
		if c.Lang != "it" {
			t.Errorf("Init(%s): lang %q, atteso \"it\"", path, c.Lang)
		}
	}
}

// TestErroriDiInit verifica che, se la configurazione non si legge o non si
// decodifica, il panic di Init sia un errore che nomina il file e avvolge
// la causa con %w, senza maiuscola iniziale e senza a capo aggiunti.
func TestErroriDiInit(t *testing.T) {
	dir := t.TempDir()
	scrivi := func(nome, testo string) string {
		path := filepath.Join(dir, nome)
		if err := os.WriteFile(path, []byte(testo), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	lettura := "lettura della configurazione %s: "
	for _, tc := range []struct {
		caso, path, formato string
		causa               error
	}{
		{"file che manca", filepath.Join(dir, "manca.yaml"), lettura, fs.ErrNotExist},
		{"yaml rotto", scrivi("rotto.yaml", "app: [\n"), lettura, nil},
		{"valore del tipo sbagliato", scrivi("tipo.yaml", "app:\n  web-client: tanti\n"), "configurazione %s non valida: ", nil},
	} {
		t.Run(tc.caso, func(t *testing.T) {
			err := panicDiInit(tc.path)
			if err == nil {
				t.Fatal("Init non ha fatto panic con un errore")
			}
			causa := errors.Unwrap(err)
			if causa == nil || (tc.causa != nil && !errors.Is(err, tc.causa)) {
				t.Fatalf("l'errore %q non avvolge la causa (%v)", err, tc.causa)
			}
			if want := fmt.Sprintf(tc.formato, tc.path) + causa.Error(); err.Error() != want {
				t.Errorf("messaggio\n%q\natteso\n%q", err, want)
			}
		})
	}
}

// panicDiInit chiama Init su path e restituisce l'errore del panic, nil se
// Init non fa panic o il panic non e' un errore.
func panicDiInit(path string) (err error) {
	defer func() {
		err, _ = recover().(error)
	}()
	var c Config
	Init(&c, path)
	return nil
}
