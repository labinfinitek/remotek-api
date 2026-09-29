package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/lejianwen/rustdesk-api/v2/config"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/test/oidcfinto"
)

// providerMuto avvia un provider OIDC finto che accetta qualsiasi codice, e
// risponde con un id_token firmato senza nonce, ma dalla userinfo non risponde, finche' il test non finisce o chi chiama non
// rinuncia. Restituisce il provider e la configurazione OAuth2 del client.
func providerMuto(t *testing.T) (*oidc.Provider, *oauth2.Config) {
	t.Helper()
	sblocca := make(chan struct{})
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(sblocca) }) // prima di srv.Close, che aspetta le richieste aperte
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"issuer":%q,"authorization_endpoint":%q,"token_endpoint":%q,"userinfo_endpoint":%q,"jwks_uri":%q}`,
			srv.URL, srv.URL+"/auth", srv.URL+"/token", srv.URL+"/userinfo", srv.URL+"/jwks")
	})
	chiave := oidcfinto.Nuova(t)
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		jwks, err := chiave.JWKS()
		if err != nil {
			t.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, jwks)
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		idToken, err := chiave.IDToken(map[string]any{"iss": srv.URL, "aud": "id", "sub": "sub-1", "exp": time.Now().Add(time.Hour).Unix()})
		if err != nil {
			t.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"access_token":"at-1","token_type":"Bearer","expires_in":3600,"id_token":%q}`, idToken)
	})
	mux.HandleFunc("/userinfo", func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-sblocca:
		case <-r.Context().Done():
		}
	})
	provider, err := oidc.NewProvider(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return provider, &oauth2.Config{ClientID: "id", ClientSecret: "segreto", Endpoint: provider.Endpoint()}
}

// TestUserinfoSenzaRisposta prova che il callback OIDC rinuncia a un
// provider che dalla userinfo non risponde dopo tempoProviderOidc, con
// GetOauthUserInfoError e l'errore nel log: prima aspettava quanto il
// provider, senza limite, e con lui la pagina del login.
func TestUserinfoSenzaRisposta(t *testing.T) {
	registro := registroDiProva(t)
	precTempo, precConfig := tempoProviderOidc, Config
	tempoProviderOidc, Config = 200*time.Millisecond, &config.Config{}
	t.Cleanup(func() { tempoProviderOidc, Config = precTempo, precConfig })
	provider, oauthConfig := providerMuto(t)

	fatto := make(chan error, 1)
	go func() {
		fatto <- (&OauthService{}).callbackBase(oauthConfig, provider, "codice", "", "", &model.OidcUser{})
	}()
	select {
	case err := <-fatto:
		if err == nil || err.Error() != "GetOauthUserInfoError" {
			t.Errorf("callbackBase: errore %v, atteso GetOauthUserInfoError", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("callbackBase fermo da 5 secondi sulla userinfo di un provider che non risponde")
	}
	if !strings.Contains(registro.String(), "context deadline exceeded") {
		t.Errorf("nel log manca la scadenza della userinfo:\n%s", registro)
	}
}
