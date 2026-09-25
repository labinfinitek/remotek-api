package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/spf13/cobra"
	"gorm.io/gorm"

	"github.com/lejianwen/rustdesk-api/v2/config"
	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/http"
	"github.com/lejianwen/rustdesk-api/v2/lib/jwt"
	"github.com/lejianwen/rustdesk-api/v2/lib/lock"
	"github.com/lejianwen/rustdesk-api/v2/lib/logger"
	"github.com/lejianwen/rustdesk-api/v2/lib/orm"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
	"github.com/lejianwen/rustdesk-api/v2/utils"
)

const DatabaseVersion = 265

// fileAdminPassword e' il file in cui il primo avvio scrive la password
// iniziale di admin, nella cartella di rustdeskapi.db (nel container
// /app/data, il volume); nel log va solo il percorso (ADR-0008).
const fileAdminPassword = "data/admin-password.txt"

// @title 管理系统API
// @version 1.0
// @description 接口
// @basePath /api
// @securityDefinitions.apikey token
// @in header
// @name api-token
// @securitydefinitions.apikey BearerAuth
// @in header
// @name Authorization

var rootCmd = &cobra.Command{
	Use:   "apimain",
	Short: "RUSTDESK API SERVER",
	PersistentPreRun: func(_ *cobra.Command, _ []string) {
		InitGlobal()
	},
	Run: func(_ *cobra.Command, _ []string) {
		global.Logger.Info("API SERVER START")
		if err := http.ApiInit(); err != nil {
			global.Logger.Fatalf("server API fermato: %v", err)
		}
	},
}

var resetPwdCmd = &cobra.Command{
	Use:     "reset-admin-pwd [pwd]",
	Example: "reset-admin-pwd <password di 15-32 caratteri>",
	Short:   "Reset Admin Password",
	Args:    cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		reimpostaPassword(1, args[0])
	},
}
var resetUserPwdCmd = &cobra.Command{
	Use:     "reset-pwd [userId] [pwd]",
	Example: "reset-pwd 2 <password di 15-32 caratteri>",
	Short:   "Reset User Password",
	Args:    cobra.ExactArgs(2),
	Run: func(_ *cobra.Command, args []string) {
		uid, err := strconv.ParseUint(args[0], 10, 0)
		if err != nil || uid == 0 {
			global.Logger.Fatalf("userId deve essere un numero intero maggiore di 0, non %q", args[0])
		}
		reimpostaPassword(uint(uid), args[1])
	},
}

// reimpostaPassword da' all'utente id la password pwd se ha da 15 a 32
// caratteri, i limiti del validatore per le password nuove
// (http/request/admin/user.go), contati come li conta lui: caratteri, non
// byte. Se rifiuta o non riesce ferma il processo con codice 1, cosi' uno
// script se ne accorge.
func reimpostaPassword(id uint, pwd string) {
	if n := utf8.RuneCountInString(pwd); n < 15 || n > 32 {
		global.Logger.Fatalf("password rifiutata: servono da 15 a 32 caratteri, questa ne ha %d", n)
	}
	u := &model.User{}
	err := global.DB.First(u, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		global.Logger.Fatalf("utente %d non trovato", id)
	}
	if err != nil {
		global.Logger.Fatalf("lettura dell'utente %d: %v", id, err)
	}
	if err := service.AllService.UserService.UpdatePassword(u, pwd); err != nil {
		global.Logger.Fatalf("password dell'utente %d non aggiornata: %v", id, err)
	}
	global.Logger.Infof("password dell'utente %d reimpostata", id)
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&global.ConfigPath, "config", "c", "./conf/config.yaml", "choose config file")
	rootCmd.AddCommand(resetPwdCmd, resetUserPwdCmd)
}
func main() {
	if err := rootCmd.Execute(); err != nil {
		global.Logger.Error(err)
		os.Exit(1)
	}
}

func InitGlobal() {
	// 配置解析
	global.Viper = config.Init(&global.Config, global.ConfigPath)

	// 日志
	global.Logger = logger.New(&logger.Config{
		Path:         global.Config.Logger.Path,
		Level:        global.Config.Logger.Level,
		ReportCaller: global.Config.Logger.ReportCaller,
	})

	global.InitI18n()

	// gorm: solo SQLite (A3). Un altro tipo, per esempio mysql di
	// un'installazione vecchia, ferma l'avvio prima che si crei
	// data/rustdeskapi.db: partire su un database nuovo e vuoto sembrerebbe una
	// perdita di dati.
	if tipo := global.Config.Gorm.Type; tipo != "" && tipo != config.TypeSqlite {
		global.Logger.Fatalf("gorm.type %q non supportato, l'API non parte: l'unico database e' %q. "+
			"Chi usava MySQL o PostgreSQL resta sulla versione precedente o porta i dati su SQLite", tipo, config.TypeSqlite)
	}
	db, err := orm.NewSqlite(&orm.SqliteConfig{
		MaxIdleConns: global.Config.Gorm.MaxIdleConns,
		MaxOpenConns: global.Config.Gorm.MaxOpenConns,
	}, global.Logger)
	if err != nil {
		percorso, errAbs := filepath.Abs(orm.FileSqlite)
		if errAbs != nil {
			percorso = orm.FileSqlite
		}
		if errors.Is(err, orm.ErrNonScrivibile) {
			global.Logger.Fatalf("database %s non scrivibile, l'API non parte: %v. Il file e la cartella %s devono "+
				"essere dell'utente del processo: nell'immagine Docker 10001:10001, quindi sull'host "+
				"chown -R 10001:10001 della cartella dati montata (vedi README)", percorso, err, filepath.Dir(percorso))
		}
		global.Logger.Fatalf("database %s non aperto, l'API non parte: controlla che la cartella %s esista, "+
			"o si possa creare, e sia scrivibile dall'utente del processo (nell'immagine Docker 10001:10001, "+
			"vedi README): %v", percorso, filepath.Dir(percorso), err)
	}
	global.DB = db

	// validator
	global.ApiInitValidator()

	// jwt
	// fmt.Println(global.Config.Jwt.PrivateKey)
	global.Jwt = jwt.NewJwt(global.Config.Jwt.Key, global.Config.Jwt.ExpireDuration)
	// locker
	global.Lock = lock.NewLocal()

	// service
	service.New(&global.Config, global.DB, global.Logger, global.Jwt, global.Lock)

	global.LoginLimiter = utils.NewLoginLimiter(utils.SecurityPolicy{
		CaptchaThreshold: global.Config.App.CaptchaThreshold,
		BanThreshold:     global.Config.App.BanThreshold,
		AttemptsWindow:   10 * time.Minute,
		BanDuration:      30 * time.Minute,
	})
	global.LoginLimiter.RegisterProvider(utils.B64StringCaptchaProvider{})
	DatabaseAutoUpdate()
}

func DatabaseAutoUpdate() {
	version := DatabaseVersion

	db := global.DB

	if !db.Migrator().HasTable(&model.Version{}) {
		Migrate(uint(version))
	} else {
		// 查找最后一个version
		var v model.Version
		db.Last(&v)
		if v.Version < uint(version) {
			Migrate(uint(version))
		}

		// 245迁移
		if v.Version < 245 {
			// oauths 表的 oauth_type 字段设置为 op同样的值
			db.Exec("update oauths set oauth_type = op")
			db.Exec("update oauths set issuer = 'https://accounts.google.com' where op = 'google'")
			db.Exec("update user_thirds set oauth_type = third_type, op = third_type")
			// 通过email迁移旧的google授权
			uts := make([]model.UserThird, 0)
			db.Where("oauth_type = ?", "google").Find(&uts)
			for _, ut := range uts {
				if ut.UserId > 0 {
					db.Model(&model.User{}).Where("id = ?", ut.UserId).Update("email", ut.OpenId)
				}
			}
		}
		if v.Version < 246 {
			db.Exec("update oauths set issuer = 'https://accounts.google.com' where op = 'google' and issuer is null")
		}
	}

}
func Migrate(version uint) {
	global.Logger.Info("Migrating....", version)
	err := global.DB.AutoMigrate(
		&model.Version{},
		&model.User{},
		&model.UserToken{},
		&model.Tag{},
		&model.AddressBook{},
		&model.Peer{},
		&model.Group{},
		&model.UserThird{},
		&model.Oauth{},
		&model.LoginLog{},
		&model.AuditConn{},
		&model.AuditFile{},
		&model.AddressBookCollection{},
		&model.AddressBookCollectionRule{},
		&model.ServerCmd{},
		&model.DeviceGroup{},
	)
	// La riga di Version resta solo se il resto riesce: le tabelle e, al
	// primo avvio (quando e' la prima riga), i gruppi predefiniti e admin,
	// creati nella stessa transazione. Se restasse dopo un errore, al riavvio
	// admin non nascerebbe piu' e reset-admin-pwd non lo troverebbe; invece
	// il processo si ferma e al riavvio il primo avvio si ripete da capo.
	primo := false
	if err == nil {
		err = global.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&model.Version{Version: version}).Error; err != nil {
				return fmt.Errorf("riga di Version: %w", err)
			}
			var vc int64
			if err := tx.Model(&model.Version{}).Count(&vc).Error; err != nil {
				return fmt.Errorf("conteggio delle versioni: %w", err)
			}
			primo = vc == 1
			if primo {
				return primoAvvio(tx)
			}
			return nil
		})
	}
	if err != nil {
		global.Logger.Fatalf("migrazione del database alla versione %d non riuscita, al riavvio si ripete: %v", version, err)
	}
	if primo {
		percorso, err := filepath.Abs(fileAdminPassword)
		if err != nil {
			percorso = fileAdminPassword
		}
		global.Logger.Warnf("password iniziale di admin in %s: cambiala dal pannello e poi cancella il file", percorso)
	}
}

// primoAvvio crea i due gruppi predefiniti e admin, con una password casuale
// di 20 caratteri che va solo in fileAdminPassword. Il file si scrive prima
// degli insert: se non si puo', non si crea niente, e al riavvio admin
// nasce con l'id 1 che cerca reset-admin-pwd.
func primoAvvio(tx *gorm.DB) error {
	pwd := utils.RandomString(20)
	if pwd == "" {
		return errors.New("password iniziale di admin vuota")
	}
	hash, err := utils.EncryptPassword(pwd)
	if err != nil {
		return fmt.Errorf("hash della password iniziale di admin: %w", err)
	}
	if err := scriviPasswordIniziale(pwd); err != nil {
		return fmt.Errorf("file della password iniziale di admin: %w", err)
	}
	localizer := global.Localizer("")
	defaultGroup, _ := localizer.LocalizeMessage(&i18n.Message{ID: "DefaultGroup"})
	shareGroup, _ := localizer.LocalizeMessage(&i18n.Message{ID: "ShareGroup"})
	group := &model.Group{Name: defaultGroup, Type: model.GroupTypeDefault}
	if err := tx.Create(group).Error; err != nil {
		return fmt.Errorf("gruppo predefinito: %w", err)
	}
	if err := tx.Create(&model.Group{Name: shareGroup, Type: model.GroupTypeShare}).Error; err != nil {
		return fmt.Errorf("gruppo di condivisione: %w", err)
	}
	isAdmin := true
	admin := &model.User{
		Username: "admin",
		Nickname: "Admin",
		Password: hash,
		Status:   model.COMMON_STATUS_ENABLE,
		IsAdmin:  &isAdmin,
		GroupId:  group.Id,
	}
	if err := tx.Create(admin).Error; err != nil {
		return fmt.Errorf("utente admin: %w", err)
	}
	return nil
}

// scriviPasswordIniziale scrive pwd, e nient'altro, in fileAdminPassword,
// creando la cartella se manca. Il file ha permessi 0600 anche se esisteva
// gia': si cancella e si ricrea, perche' os.WriteFile non cambia i permessi
// di un file che c'e', e O_EXCL non segue un link simbolico messo al suo
// posto. Chmod li porta a 0600 esatti anche con una umask insolita, prima
// di scrivere.
func scriviPasswordIniziale(pwd string) error {
	if err := os.MkdirAll(filepath.Dir(fileAdminPassword), 0o700); err != nil {
		return err
	}
	if err := os.Remove(fileAdminPassword); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	f, err := os.OpenFile(fileAdminPassword, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if err = f.Chmod(0o600); err == nil {
		if _, err = f.WriteString(pwd); err == nil {
			err = f.Sync()
		}
	}
	return errors.Join(err, f.Close())
}
