package model

type UserToken struct {
	IdModel
	UserId     uint   `json:"user_id" gorm:"default:0;not null;index"`
	DeviceUuid string `json:"device_uuid" gorm:"default:'';omitempty;"`
	DeviceId   string `json:"device_id" gorm:"default:'';omitempty;"`
	Token      string `json:"token" gorm:"default:'';not null;index"`
	ExpiredAt  int64  `json:"expired_at" gorm:"default:0;not null;"`
	// TipoLogin e' il tipo del login che ha creato il token, come
	// LoginLog.Type (account, oauth o webauth; versione del database 276).
	// Vuoto nei token di prima, che valgono come login con password.
	TipoLogin string `json:"tipo_login" gorm:"default:'';not null;"`
	TimeModel
}

// DaOidc dice se il token e' nato da un login OIDC; un token senza tipo
// vale come nato dalla password, e uno webauth non e' OIDC.
func (ut *UserToken) DaOidc() bool {
	return ut.TipoLogin == LoginLogTypeOauth
}

type UserTokenList struct {
	UserTokens []UserToken `json:"list"`
	Pagination
}
