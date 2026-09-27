package model

import (
	"errors"
	"strings"
)

const OIDC_DEFAULT_SCOPES = "openid,profile,email"

const (
	// make sure the value shouldbe lowercase
	OauthTypeOidc    string = "oidc"
	OauthTypeWebauth string = "webauth"
	PKCEMethodS256   string = "S256"
	PKCEMethodPlain  string = "plain"
)

// ErrOauthTypeRemoved e' l'errore di un provider di tipo github, google o
// linuxdo, tolti (A3): il testo e' l'ID del messaggio per il pannello.
var ErrOauthTypeRemoved = errors.New("OauthTypeRemoved")

// ValidateOauthType restituisce nil se oauthType e' oidc, l'unico tipo di
// provider che l'API usa, ErrOauthTypeRemoved per un tipo tolto e un altro
// errore per il resto. Anche webauth: e' il login confermato dal pannello,
// che BeginAuth serve prima di leggere i provider, non un provider da salvare.
func ValidateOauthType(oauthType string) error {
	switch oauthType {
	case OauthTypeOidc:
		return nil
	case "github", "google", "linuxdo":
		return ErrOauthTypeRemoved
	default:
		return errors.New("invalid Oauth type")
	}
}

type Oauth struct {
	IdModel
	Op           string `json:"op"`
	OauthType    string `json:"oauth_type"`
	ClientId     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	// RedirectUrl  string `json:"redirect_url"`
	AutoRegister *bool  `json:"auto_register"`
	Scopes       string `json:"scopes"`
	Issuer       string `json:"issuer"`
	PkceEnable   *bool  `json:"pkce_enable"`
	PkceMethod   string `json:"pkce_method"`
	TimeModel
}

// FormatOauthInfo controlla il tipo e completa un provider OAuth prima di
// crearlo o aggiornarlo: Op "oidc" se vuoto, PKCE spento e S256 se non
// indicati.
func (oa *Oauth) FormatOauthInfo() error {
	oauthType := strings.TrimSpace(oa.OauthType)
	err := ValidateOauthType(oa.OauthType)
	if err != nil {
		return err
	}
	// check if the op is empty, set the default value
	op := strings.TrimSpace(oa.Op)
	if op == "" && oauthType == OauthTypeOidc {
		oa.Op = OauthTypeOidc
	}
	if oa.PkceEnable == nil {
		oa.PkceEnable = new(bool)
		*oa.PkceEnable = false
	}
	if oa.PkceMethod == "" {
		oa.PkceMethod = PKCEMethodS256
	}
	return nil
}

type OauthUser struct {
	OpenId        string `json:"open_id" gorm:"not null;index"`
	Name          string `json:"name"`
	Username      string `json:"username"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email,omitempty"`
	Picture       string `json:"picture,omitempty"`
}

func (ou *OauthUser) ToUser(user *User, overideUsername bool) {
	if overideUsername {
		user.Username = ou.Username
	}
	user.Email = ou.Email
	user.Nickname = ou.Name
	user.Avatar = ou.Picture
}

type OauthUserBase struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type OidcUser struct {
	OauthUserBase
	Sub               string `json:"sub"`
	VerifiedEmail     bool   `json:"email_verified"`
	PreferredUsername string `json:"preferred_username"`
	Picture           string `json:"picture"`
}

func (ou *OidcUser) ToOauthUser() *OauthUser {
	var username string
	// 使用 PreferredUsername，如果不存在，降级到 Email 前缀
	if ou.PreferredUsername != "" {
		username = ou.PreferredUsername
	} else {
		username = strings.ToLower(ou.Email)
	}

	return &OauthUser{
		OpenId:        ou.Sub,
		Name:          ou.Name,
		Username:      username,
		Email:         ou.Email,
		VerifiedEmail: ou.VerifiedEmail,
		Picture:       ou.Picture,
	}
}

type OauthList struct {
	Oauths []*Oauth `json:"list"`
	Pagination
}
