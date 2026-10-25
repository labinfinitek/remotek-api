package admin

import "github.com/lejianwen/rustdesk-api/v2/model"

// OauthPannello e' un provider come lo vede il pannello: i campi di
// model.Oauth senza il segreto del client, che non si rilegge e che cosi' lo
// swagger non descrive; HasClientSecret dice se c'e'.
type OauthPannello struct {
	model.IdModel
	Op           string `json:"op"`
	OauthType    string `json:"oauth_type"`
	ClientId     string `json:"client_id"`
	AutoRegister *bool  `json:"auto_register"`
	Scopes       string `json:"scopes"`
	Issuer       string `json:"issuer"`
	PkceEnable   *bool  `json:"pkce_enable"`
	PkceMethod   string `json:"pkce_method"`
	model.TimeModel
	HasClientSecret bool `json:"has_client_secret"`
}

// NuovoOauthPannello restituisce o per il pannello.
func NuovoOauthPannello(o *model.Oauth) OauthPannello {
	return OauthPannello{
		IdModel: o.IdModel, Op: o.Op, OauthType: o.OauthType, ClientId: o.ClientId,
		AutoRegister: o.AutoRegister, Scopes: o.Scopes, Issuer: o.Issuer,
		PkceEnable: o.PkceEnable, PkceMethod: o.PkceMethod, TimeModel: o.TimeModel,
		HasClientSecret: o.ClientSecret != "",
	}
}

// OauthListPannello e' la lista dei provider del pannello, con le pagine di
// model.OauthList.
type OauthListPannello struct {
	Oauths []OauthPannello `json:"list"`
	model.Pagination
}

// RichiestaPannello e' una richiesta di associazione OIDC come la vede
// l'amministratore: senza il sub, con l'eta' in secondi.
type RichiestaPannello struct {
	Codice          string `json:"codice"`
	Op              string `json:"op"`
	Nome            string `json:"nome"`
	Email           string `json:"email"`
	EmailVerificata bool   `json:"email_verificata"`
	Secondi         int64  `json:"secondi"`
}
