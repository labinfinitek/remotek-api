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
	WebClient        int           `mapstructure:"web-client"`
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
	Lang       string `mapstructure:"lang"`
	Brand      Brand
	App        App
	Admin      Admin
	Gorm       Gorm
	Mysql      Mysql
	Postgresql Postgresql
	Gin        Gin
	Logger     Logger
	Jwt        Jwt
	Rustdesk   Rustdesk
	Proxy      Proxy
	Ldap       Ldap
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
	v.SetDefault("app.web-client", 0)
	v.SetDefault("app.web-sso", false)
	v.SetDefault("app.register", false)
	v.SetDefault("app.show-swagger", 0)
	v.SetDefault("app.captcha-threshold", 3)
	v.SetDefault("app.ban-threshold", 10)
	v.SetDefault("gin.trust-proxy", "") // nessun proxy fidato: vedi http.setTrustedProxies
	// Lingua delle risposte quando Accept-Language non ne sceglie una: il
	// client RustDesk non la manda, quindi e' quella che vede chi lo usa.
	v.SetDefault("lang", "it")
	// Marchio (vedi Brand): col default viper conosce le chiavi, e
	// RUSTDESK_API_BRAND_NAME vale anche se il file non ha la sezione brand.
	v.SetDefault("brand.name", DefaultBrandName)
	v.SetDefault("brand.dir", DefaultBrandDir)
	v.SetDefault("admin.title", "") // vuoto: brand.name (Admin.Init)
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.SetEnvPrefix("RUSTDESK_API")
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	err := v.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("lettura della configurazione %s: %w", path, err))
	}
	/*
		v.WatchConfig()


			//监听配置修改没什么必要
			v.OnConfigChange(func(e fsnotify.Event) {
				//配置文件修改监听
				fmt.Println("config file changed:", e.Name)
				if err2 := v.Unmarshal(rowVal); err2 != nil {
					fmt.Println(err2)
				}
				rowVal.Rustdesk.LoadKeyFile()
				rowVal.Rustdesk.ParsePort()
			})
	*/
	if err := v.Unmarshal(rowVal); err != nil {
		panic(fmt.Errorf("configurazione %s non valida: %w", path, err))
	}
	rowVal.Rustdesk.LoadKeyFile()
	rowVal.Brand.Init()
	rowVal.Admin.Init(rowVal.Brand)
	return v
}

// ReadEnv 读取环境变量
func ReadEnv(rowVal interface{}) *viper.Viper {
	v := viper.New()
	v.AutomaticEnv()
	if err := v.Unmarshal(rowVal); err != nil {
		fmt.Println(err)
	}
	return v
}
