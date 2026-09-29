package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	DebugMode     = "debug"
	ReleaseMode   = "release"
	DefaultConfig = "conf/config.yaml"
)

type App struct {
	Register         bool          `mapstructure:"register"`
	RegisterStatus   int           `mapstructure:"register-status"`
	ShowSwagger      int           `mapstructure:"show-swagger"`
	TokenExpire      time.Duration `mapstructure:"token-expire"`
	WebSso           bool          `mapstructure:"web-sso"`
	DisablePwdLogin  bool          `mapstructure:"disable-pwd-login"`
	CaptchaThreshold int           `mapstructure:"captcha-threshold"`
	BanThreshold     int           `mapstructure:"ban-threshold"`
}
type Admin struct {
	Title           string `mapstructure:"title"`
	Hello           string `mapstructure:"hello"`
	HelloFile       string `mapstructure:"hello-file"`
	IdServerPort    int    `mapstructure:"id-server-port"`
	RelayServerPort int    `mapstructure:"relay-server-port"`
}
type Config struct {
	Lang     string `mapstructure:"lang"`
	Brand    Brand
	App      App
	Admin    Admin
	Gorm     Gorm
	Gin      Gin
	Logger   Logger
	Jwt      Jwt
	Rustdesk Rustdesk
	Proxy    Proxy
	Ldap     Ldap
}

// Init completa la sezione admin: il titolo vuoto e' il nome del marchio.
func (a *Admin) Init(brand Brand) {
	if a.Title == "" {
		a.Title = brand.Name
	}
	if a.IdServerPort == 0 {
		a.IdServerPort = DefaultIdServerPort
	}
	if a.RelayServerPort == 0 {
		a.RelayServerPort = DefaultRelayServerPort
	}
}

// Init 初始化配置
func Init(rowVal *Config, path string) *viper.Viper {
	if path == "" {
		path = DefaultConfig
	}
	v := viper.GetViper()
	// Default sicuri (ADR-0008): li prende una chiave che manca sia dal file
	// sia dalle variabili RUSTDESK_API_*, al posto dello zero del tipo. Il file
	// batte il default, la variabile batte il file. Col default viper conosce
	// la chiave, quindi la variabile vale anche se il file non la nomina.
	v.SetDefault("app.web-sso", false)
	v.SetDefault("app.register", false)
	v.SetDefault("app.show-swagger", 0)
	v.SetDefault("app.captcha-threshold", 3)
	v.SetDefault("app.ban-threshold", 10)
	v.SetDefault("gin.trust-proxy", "") // nessun proxy fidato: vedi http.setTrustedProxies
	// Con ldaps:// i certificati del server si verificano; una CA interna
	// si indica con ldap.tls-ca-file, false va scelto a mano.
	v.SetDefault("ldap.tls-verify", true)
	// Lingua delle risposte quando Accept-Language non ne sceglie una: il
	// client RustDesk non la manda, quindi e' quella che vede chi lo usa.
	v.SetDefault("lang", "it")
	// Marchio (vedi Brand): col default viper conosce le chiavi, e
	// RUSTDESK_API_BRAND_NAME vale anche se il file non ha la sezione brand.
	v.SetDefault("brand.name", DefaultBrandName)
	v.SetDefault("brand.dir", DefaultBrandDir)
	v.SetDefault("admin.title", "") // vuoto: brand.name (Admin.Init)
	// Log: info, e logger.path nota a viper solo perche' l'avvio dica che
	// RUSTDESK_API_LOGGER_PATH e' ignorata.
	v.SetDefault("logger.level", "info")
	v.SetDefault("logger.path", "")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.SetEnvPrefix("RUSTDESK_API")
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	err := v.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("lettura della configurazione %s: %w", path, err))
	}
	if err := v.Unmarshal(rowVal); err != nil {
		panic(fmt.Errorf("configurazione %s non valida: %w", path, err))
	}
	rowVal.Rustdesk.LoadKeyFile()
	rowVal.Brand.Init()
	rowVal.Admin.Init(rowVal.Brand)
	return v
}
