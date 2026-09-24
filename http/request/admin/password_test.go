package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/http/request/api"
)

var initValidator = sync.OnceFunc(global.ApiInitValidator)

// valida passa form al validatore dell'API, come fanno i controller, con i
// messaggi in inglese; restituisce il primo errore, quello che va al client,
// o "" se il form passa.
func valida(t *testing.T, form any) string {
	t.Helper()
	initValidator()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Request.Header.Set("Accept-Language", "en")
	if errs := global.Validator.ValidStruct(c, form); len(errs) > 0 {
		return errs[0]
	}
	return ""
}

// TestPasswordNuovaLunghezza verifica i limiti, 15 e 32 caratteri, dei tre
// form con cui una password si crea o si cambia. I caratteri si contano come
// caratteri, non come byte: 15 "è" sono 30 byte e passano, 14 no. Il cambio
// della propria password accetta una vecchia password di 4 caratteri.
func TestPasswordNuovaLunghezza(t *testing.T) {
	forms := []struct {
		nome, campo string
		form        func(p string) any
	}{
		{"registrazione", "Password", func(p string) any {
			return &RegisterForm{Username: "collaudo", Password: p, ConfirmPassword: p}
		}},
		{"pannello", "Password", func(p string) any { return &UserPasswordForm{Id: 2, Password: p} }},
		{"propria", "NewPassword", func(p string) any {
			return &ChangeCurPasswordForm{OldPassword: "abcd", NewPassword: p}
		}},
	}
	passwords := []struct {
		nome, password, errore string
	}{
		{"14 caratteri", strings.Repeat("a", 14), " must be at least 15 characters in length"},
		{"14 caratteri non ASCII", strings.Repeat("è", 14), " must be at least 15 characters in length"},
		{"15 caratteri", strings.Repeat("a", 15), ""},
		{"15 caratteri non ASCII", strings.Repeat("è", 15), ""},
		{"32 caratteri", strings.Repeat("a", 32), ""},
		{"33 caratteri", strings.Repeat("a", 33), " must be at maximum 32 characters in length"},
	}
	for _, f := range forms {
		for _, p := range passwords {
			t.Run(f.nome+"/"+p.nome, func(t *testing.T) {
				want := ""
				if p.errore != "" {
					want = f.campo + p.errore
				}
				if got := valida(t, f.form(p.password)); got != want {
					t.Errorf("errore %q, atteso %q", got, want)
				}
			})
		}
	}
}

// TestLoginPasswordCorte verifica che i due login, del client (/api/login) e
// del pannello (/api/admin/login), accettino ancora le password corte create
// prima del minimo di 15, e che il client rifiuti le stesse di prima con lo
// stesso messaggio. Passa anche col codice di prima: e' la prova che il login
// non cambia.
func TestLoginPasswordCorte(t *testing.T) {
	for _, n := range []int{4, 8, 14} {
		p := strings.Repeat("a", n)
		if got := valida(t, &api.LoginForm{Username: "collaudo", Password: p}); got != "" {
			t.Errorf("login del client, %d caratteri: errore %q, atteso nessuno", n, got)
		}
	}
	for _, n := range []int{1, 4, 14} {
		p := strings.Repeat("a", n)
		if got := valida(t, &Login{Username: "collaudo", Password: p}); got != "" {
			t.Errorf("login del pannello, %d caratteri: errore %q, atteso nessuno", n, got)
		}
	}
	rifiutate := map[string]string{
		strings.Repeat("a", 3):  "密码 must be at least 4 characters in length",
		strings.Repeat("a", 33): "密码 must be at maximum 32 characters in length",
	}
	for p, want := range rifiutate {
		if got := valida(t, &api.LoginForm{Username: "collaudo", Password: p}); got != want {
			t.Errorf("login del client, %d caratteri: errore %q, atteso %q", len(p), got, want)
		}
	}
}

// TestRegistrazioneConfermaDiversa verifica che la registrazione rifiuti una
// conferma diversa dalla password, con il messaggio del validatore come per
// gli altri campi del form.
func TestRegistrazioneConfermaDiversa(t *testing.T) {
	f := &RegisterForm{Username: "collaudo", Password: strings.Repeat("a", 15), ConfirmPassword: strings.Repeat("b", 15)}
	if got, want := valida(t, f), "ConfirmPassword must be equal to Password"; got != want {
		t.Errorf("errore %q, atteso %q", got, want)
	}
}
