package model

import (
	"strconv"
	"testing"
)

// TestFormatOauthInfoDefault prova i default sicuri di un provider OIDC
// salvato senza i campi facoltativi: op "oidc", autoregistrazione e PKCE
// spenti, metodo S256. Un valore scelto resta com'e'.
func TestFormatOauthInfoDefault(t *testing.T) {
	acceso := true
	for _, tc := range []struct {
		nome, op, metodo string
		oa               Oauth
		registra, pkce   string
	}{
		{"senza campi facoltativi", OauthTypeOidc, PKCEMethodS256, Oauth{OauthType: OauthTypeOidc}, "false", "false"},
		{"con i campi indicati", "aziendale", PKCEMethodPlain, Oauth{Op: "aziendale", OauthType: OauthTypeOidc,
			AutoRegister: &acceso, PkceEnable: &acceso, PkceMethod: PKCEMethodPlain}, "true", "true"},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			oa := tc.oa
			if err := oa.FormatOauthInfo(); err != nil {
				t.Fatal(err)
			}
			if got := comeTesto(oa.AutoRegister); got != tc.registra {
				t.Errorf("AutoRegister: %s, atteso %s", got, tc.registra)
			}
			if got := comeTesto(oa.PkceEnable); got != tc.pkce {
				t.Errorf("PkceEnable: %s, atteso %s", got, tc.pkce)
			}
			if oa.Op != tc.op || oa.PkceMethod != tc.metodo {
				t.Errorf("Op %q e PkceMethod %q, attesi %q e %q", oa.Op, oa.PkceMethod, tc.op, tc.metodo)
			}
		})
	}
}

// comeTesto scrive un *bool come "nil", "true" o "false".
func comeTesto(b *bool) string {
	if b == nil {
		return "nil"
	}
	return strconv.FormatBool(*b)
}
