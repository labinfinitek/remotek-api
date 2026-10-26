package model

type LoginLog struct {
	IdModel
	UserId      uint   `json:"user_id" gorm:"default:0;not null;"`
	Client      string `json:"client"` // webadmin,webclient,app,
	DeviceId    string `json:"device_id"`
	Uuid        string `json:"uuid"`
	Ip          string `json:"ip"`
	Type        string `json:"type"`     // account,oauth
	Platform    string `json:"platform"` // windows,linux,mac,android,ios
	UserTokenId uint   `json:"user_token_id" gorm:"default:0;not null;"`
	IsDeleted   uint   `json:"is_deleted" gorm:"default:0;not null;"`
	TimeModel
}

const (
	LoginLogClientWebAdmin = "webadmin"
	LoginLogClientApp      = "app"
)

const (
	LoginLogTypeAccount = "account"
	LoginLogTypeOauth   = "oauth"
	// LoginLogTypeWebauth e' il login del client confermato dal pannello
	// (app.web-sso): chi lo conferma puo' essere entrato con la password,
	// quindi il token non vale come OIDC.
	LoginLogTypeWebauth = "webauth"
)

const (
	IsDeletedNo  = 0
	IsDeletedYes = 1
)

type LoginLogList struct {
	LoginLogs []*LoginLog `json:"list"`
	Pagination
}
