package admin

import "github.com/lejianwen/rustdesk-api/v2/model"

// OauthPannello e' un provider come lo vede il pannello: senza il segreto
// del client, che non si rilegge; HasClientSecret dice se c'e'.
type OauthPannello struct {
	*model.Oauth
	// ClientSecret copre il campo di model.Oauth: sempre nil, non esce.
	ClientSecret    *struct{} `json:"client_secret,omitempty"`
	HasClientSecret bool      `json:"has_client_secret"`
}

// NuovoOauthPannello restituisce o per il pannello.
func NuovoOauthPannello(o *model.Oauth) OauthPannello {
	return OauthPannello{Oauth: o, HasClientSecret: o.ClientSecret != ""}
}

// OauthListPannello e' la lista dei provider del pannello, con le pagine di
// model.OauthList.
type OauthListPannello struct {
	Oauths []OauthPannello `json:"list"`
	model.Pagination
}
