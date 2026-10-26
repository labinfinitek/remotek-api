package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

const DatabaseVersion = 278

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
		// La consegna degli eventi parte con l'API e si ferma con lei.
		ferma := service.AvviaConsegna(global.Config.Integrazione)
		defer ferma()
		if err := http.ApiInit(); err != nil {
			global.Logger.Fatalf("server API fermato: %v", err)
		}
	},
	// Dopo lo stop normale del server e alla fine di reset-admin-pwd,
	// reset-pwd e agente-ai, che ereditano da rootCmd.
	PersistentPostRun: func(_ *cobra.Command, _ []string) {
		chiudiDB()
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

var agenteAiCmd = &cobra.Command{
	Use:     "agente-ai [username] [username del tecnico]",
	Example: "agente-ai agente-mario mario",
	Short:   "Crea l'account di un agente AI",
	Args:    cobra.ExactArgs(2),
	Run: func(_ *cobra.Command, args []string) {
		creaAgente(args[0], args[1])
	},
}

// creaAgente crea l'account dell'agente AI nome, del tecnico di username
// tecnico e nel suo gruppo, mai amministratore, e stampa su stdout, una
// volta sola, la sua password casuale di 24 caratteri; nel log non va.
// Rifiuta con codice 1 se il tecnico non c'e', non e' una persona o e'
// disattivato, o se nome c'e' gia'.
func creaAgente(nome, tecnico string) {
	us := service.AllService.UserService
	if n := utf8.RuneCountInString(nome); n < 2 || n > 32 {
		global.Logger.Fatalf("username rifiutato: servono da 2 a 32 caratteri, questo ne ha %d", n)
	}
	t, err := us.InfoByUsername(tecnico)
	if errors.Is(err, service.ErrNotFound) {
		global.Logger.Fatalf("tecnico %q non trovato", tecnico)
	}
	if err != nil {
		global.Logger.Fatalf("lettura del tecnico %q: %v", tecnico, err)
	}
	if t.AgenteDi != 0 {
		global.Logger.Fatalf("%q e' un agente AI, non una persona", tecnico)
	}
	if t.Status != model.COMMON_STATUS_ENABLE {
		global.Logger.Fatalf("il tecnico %q e' disattivato", tecnico)
	}
	pwd := utils.RandomString(24)
	if pwd == "" {
		global.Logger.Fatalf("password casuale non generata")
	}
	no := false
	a := &model.User{Username: nome, Password: pwd, GroupId: t.GroupId, IsAdmin: &no, Status: model.COMMON_STATUS_ENABLE, AgenteDi: t.Id}
	if err := us.Create(a); err != nil {
		global.Logger.Fatalf("agente %q non creato: %v", nome, err)
	}
	global.Logger.Infof("agente AI %d creato per il tecnico %d", a.Id, t.Id)
	if _, err := fmt.Fprintln(os.Stdout, pwd); err != nil {
		global.Logger.Fatalf("password dell'agente %d non stampata: %v", a.Id, err)
	}
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

var tokenIntegrazioneCmd = &cobra.Command{
	Use:   "token-integrazione",
	Short: "Token di integrazione in sola lettura (n8n)",
}

var tokenIntegrazioneCreaCmd = &cobra.Command{
	Use:     "crea [nome]",
	Example: "token-integrazione crea n8n",
	Short:   "Crea un token di integrazione e lo stampa una volta sola",
	Args:    cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		token, err := service.AllService.TokenIntegrazioneService.Crea(args[0], time.Now())
		if err != nil {
			global.Logger.Fatalf("token di integrazione non creato: %v", err)
		}
		global.Logger.Infof("token di integrazione %s creato, scade tra un anno", args[0])
		if _, err := fmt.Fprintln(os.Stdout, token); err != nil {
			global.Logger.Fatalf("token di integrazione %s non stampato: %v", args[0], err)
		}
	},
}

var tokenIntegrazioneElencoCmd = &cobra.Command{
	Use:   "elenco",
	Short: "Elenca i token di integrazione: nome, creazione, scadenza, revoca, ultimo uso",
	Args:  cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		elenco, err := service.AllService.TokenIntegrazioneService.Elenco()
		if err != nil {
			global.Logger.Fatalf("token di integrazione non letti: %v", err)
		}
		if _, err := fmt.Fprint(os.Stdout, elencoToken(elenco)); err != nil {
			global.Logger.Fatalf("elenco dei token di integrazione non stampato: %v", err)
		}
	},
}

var tokenIntegrazioneRevocaCmd = &cobra.Command{
	Use:     "revoca [nome]",
	Example: "token-integrazione revoca n8n",
	Short:   "Revoca un token di integrazione",
	Args:    cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		err := service.AllService.TokenIntegrazioneService.Revoca(args[0], time.Now())
		if errors.Is(err, service.ErrNotFound) {
			global.Logger.Fatalf("token di integrazione %q non trovato", args[0])
		}
		if err != nil {
			global.Logger.Fatalf("token di integrazione %q non revocato: %v", args[0], err)
		}
		global.Logger.Infof("token di integrazione %s revocato", args[0])
	},
}

// elencoToken scrive una riga per token, separata da tabulazioni: nome,
// creato, scade, revocato e ultimo uso in RFC 3339 UTC, "-" se manca
// l'ora. Mai token ne' hash.
func elencoToken(elenco []model.TokenIntegrazione) string {
	quando := func(t int64) string {
		if t == 0 {
			return "-"
		}
		return time.Unix(t, 0).UTC().Format(time.RFC3339)
	}
	var b strings.Builder
	b.WriteString("nome\tcreato\tscade\trevocato\tultimo_uso\n")
	for _, t := range elenco {
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%s\n", t.Nome, quando(t.Creato), quando(t.Scade), quando(t.Revocato), quando(t.UltimoUso))
	}
	return b.String()
}

var lasciapassareCmd = &cobra.Command{
	Use:   "lasciapassare",
	Short: "Chiave dei lasciapassare degli agenti AI (ADR-0028)",
}

// lasciapassareChiaveNuovaCmd non passa da InitGlobal ne' dal database:
// la chiave serve prima che l'API la trovi configurata. Un errore lo
// scrive cobra su stderr, e main esce con 1.
var lasciapassareChiaveNuovaCmd = &cobra.Command{
	Use:               "chiave-nuova [file]",
	Example:           "lasciapassare chiave-nuova /app/data/lasciapassare.key",
	Short:             "Genera la chiave dei lasciapassare nel file (600) e stampa la chiave pubblica",
	Args:              cobra.ExactArgs(1),
	PersistentPreRun:  func(_ *cobra.Command, _ []string) {},
	PersistentPostRun: func(_ *cobra.Command, _ []string) {},
	SilenceUsage:      true,
	RunE: func(cmd *cobra.Command, args []string) error {
		pub, err := config.ChiaveNuova(args[0])
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), pub)
		return err
	},
}

func init() {
	lasciapassareCmd.AddCommand(lasciapassareChiaveNuovaCmd)
	rootCmd.PersistentFlags().StringVarP(&global.ConfigPath, "config", "c", "./conf/config.yaml", "choose config file")
	tokenIntegrazioneCmd.AddCommand(tokenIntegrazioneCreaCmd, tokenIntegrazioneElencoCmd, tokenIntegrazioneRevocaCmd)
	rootCmd.AddCommand(resetPwdCmd, resetUserPwdCmd, agenteAiCmd, tokenIntegrazioneCmd, lasciapassareCmd)
}

// main esce con 1 se cobra restituisce un errore. Cobra lo ha gia' scritto su
// stderr; nel log va solo se InitGlobal ha gia' creato il logger: un errore
// degli argomenti arriva prima.
func main() {
	if err := rootCmd.Execute(); err != nil {
		if global.Logger != nil {
			global.Logger.Error(err)
		}
		os.Exit(1)
	}
}

func InitGlobal() {
	// 配置解析
	global.Viper = config.Init(&global.Config, global.ConfigPath)

	// Log: prima del resto, che ci scrive
	l, err := logger.New(&logger.Config{
		Level:        global.Config.Logger.Level,
		ReportCaller: global.Config.Logger.ReportCaller,
	})
	global.Logger = l
	slog.SetDefault(l.Slog())
	if err != nil {
		global.Logger.Fatalf("%v, l'API non parte", err)
	}
	if global.Config.Logger.Path != "" {
		global.Logger.Warnf("logger.path %q ignorata: il log va solo su stdout", global.Config.Logger.Path)
	}

	global.InitI18n()

	for _, a := range global.Config.Ldap.Avvisi() {
		global.Logger.Warn(a)
	}
	for _, a := range global.Config.Rustdesk.Avvisi() {
		global.Logger.Warn(a)
	}
	// Prima del database: con l'integrazione configurata male non si crea
	// niente. Il messaggio non ha ne' l'URL ne' il segreto.
	if err := global.Config.Integrazione.Controlla(); err != nil {
		global.Logger.Fatalf("%v, l'API non parte", err)
	}
	// Anche la chiave dei lasciapassare (ADR-0028): nel log solo la
	// pubblica, mai il seme.
	if err := global.Config.Agente.Carica(); err != nil {
		global.Logger.Fatalf("%v, l'API non parte", err)
	}
	if pub := global.Config.Agente.Pubblica(); pub != "" {
		global.Logger.Infof("lasciapassare dell'agente acceso, chiave pubblica %s", pub)
	}

	// gorm: solo SQLite (A3). Un altro tipo, per esempio mysql di
	// un'installazione vecchia, ferma l'avvio prima che si crei
	// data/rustdeskapi.db: partire su un database nuovo e vuoto sembrerebbe una
	// perdita di dati.
	if tipo := global.Config.Gorm.Type; tipo != "" && tipo != config.TypeSqlite {
		global.Logger.Fatalf("gorm.type %q non supportato, l'API non parte: l'unico database e' %q. "+
			"Chi usava MySQL o PostgreSQL resta sulla versione precedente o porta i dati su SQLite", tipo, config.TypeSqlite)
	}
	db, err := orm.NewSqlite(global.Logger)
	if err != nil {
		percorso, errAbs := filepath.Abs(orm.FileSqlite)
		if errAbs != nil {
			percorso = orm.FileSqlite
		}
		switch {
		case errors.Is(err, orm.ErrNonScrivibile):
			global.Logger.Fatalf("database %s non scrivibile, l'API non parte: %v. Il file e la cartella %s devono "+
				"essere dell'utente del processo: nell'immagine Docker 10001:10001, quindi sull'host "+
				"chown -R 10001:10001 della cartella dati montata (vedi README)", percorso, err, filepath.Dir(percorso))
		case errors.Is(err, orm.ErrNonWAL):
			global.Logger.Fatalf("database %s non in WAL, l'API non parte: %v. Con synchronous NORMAL fuori dal WAL "+
				"una caduta di corrente puo' corrompere il database", percorso, err)
		case errors.Is(err, orm.ErrDanneggiato):
			global.Logger.Fatalf("database %s danneggiato, l'API non parte: %v. Ripristina l'ultimo backup "+
				"(vedi README)", percorso, err)
		default:
			global.Logger.Fatalf("database %s non aperto, l'API non parte: controlla che la cartella %s esista, "+
				"o si possa creare, e sia scrivibile dall'utente del processo (nell'immagine Docker 10001:10001, "+
				"vedi README): %v", percorso, filepath.Dir(percorso), err)
		}
	}
	global.DB = db

	// validator
	global.ApiInitValidator()

	// jwt
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
	if err := togliVincoloDispositivi(); err != nil {
		global.Logger.Fatalf("migrazione del database non riuscita, al riavvio si ripete: %v", err)
	}
	DatabaseAutoUpdate()
	indiceUnicoDispositivi()
	indiceUnicoAssociazioni()
	avvisaProviderNonSupportati()
}

// chiudiDB chiude il database aperto da InitGlobal. L'ultima connessione
// che si chiude fa il checkpoint del WAL e cancella -wal e -shm: dopo uno
// stop pulito in data/ resta il solo rustdeskapi.db. Chi esce con Fatalf
// non chiude, come una caduta: SQLite riprende il WAL alla prossima
// apertura.
func chiudiDB() {
	sqlDB, err := global.DB.DB()
	if err == nil {
		err = sqlDB.Close()
	}
	if err != nil {
		global.Logger.Errorf("chiusura del database: %v", err)
	}
}

// avvisaProviderNonSupportati scrive un warn per ogni provider OAuth che il
// login ignora perche' non e' oidc, come github, google e linuxdo (A3): resta
// nel database e nel pannello, dove si cancella o si rifa' come OIDC.
func avvisaProviderNonSupportati() {
	provider, err := service.AllService.OauthService.NonSupportati()
	if err != nil {
		global.Logger.Errorf("controllo dei provider OAuth non riuscito, l'avvio continua: %v", err)
		return
	}
	for _, p := range provider {
		global.Logger.Warnf("provider OAuth %q di tipo %q ignorato: il login usa solo il tipo oidc. "+
			"Dal pannello cancellalo o rifallo di tipo OIDC (Google: vedi README)", p.Op, p.OauthType)
	}
}

// togliVincoloDispositivi toglie da peers la chiave esterna fk_peers_user
// verso users, che l'upstream creava fino al 2024-10-14 (gorm senza
// DisableForeignKeyConstraintWhenMigrating e Peer con User): con
// foreign_keys acceso (ADR-0007) un dispositivo senza utente, user_id 0,
// non si salverebbe. E' l'unico vincolo mai creato: nessun altro modello
// aveva associazioni. DropConstraint ricrea la tabella senza indici, che
// AutoMigrate rimette nella stessa transazione.
func togliVincoloDispositivi() error {
	if !global.DB.Migrator().HasConstraint(&model.Peer{}, "fk_peers_user") {
		return nil
	}
	err := global.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Migrator().DropConstraint(&model.Peer{}, "fk_peers_user"); err != nil {
			return err
		}
		return tx.AutoMigrate(&model.Peer{})
	})
	if err != nil {
		return fmt.Errorf("chiave esterna fk_peers_user di peers: %w", err)
	}
	global.Logger.Info("tolta da peers la chiave esterna fk_peers_user delle versioni di upstream fino al 2024-10-14")
	return nil
}

// indiceUnicoDispositivi crea all'avvio l'indice unico su peers.id, se non
// c'e' e peers non ha ID doppi (PeerService.CreaIndiceIdUnico). Non sta nel tag
// del modello: AutoMigrate fallirebbe su un database con doppioni, e Migrate
// fermerebbe l'avvio. Con doppioni scrive un error col loro numero, senza
// ID, uuid o nomi, e l'avvio continua; il passo si ripete a ogni avvio
// finche' l'indice non c'e'. Nessuna riga si cancella: quali tenere lo
// decide chi usa il pannello. Anche un errore del database va nel log e
// lascia andare avanti l'avvio senza l'indice, come prima.
func indiceUnicoDispositivi() {
	creato, doppi, err := service.AllService.PeerService.CreaIndiceIdUnico()
	switch {
	case err != nil:
		global.Logger.Errorf("indice unico %s non creato, l'avvio continua: %v", service.IndiceIdUnico, err)
	case doppi > 0:
		global.Logger.Errorf("peers ha %d ID di PC su piu' righe: l'indice unico %s non si crea e un ID doppio "+
			"si lega al dispositivo della prima riga. Dal pannello, in Dispositivi, cerca ogni ID doppio e cancella le righe "+
			"in piu' (anche quelle con ID vuoto); al prossimo avvio l'indice si crea", doppi, service.IndiceIdUnico)
	case creato:
		global.Logger.Infof("creato l'indice unico %s: un ID di PC compare una volta sola in peers", service.IndiceIdUnico)
	}
}

// indiceUnicoAssociazioni crea all'avvio l'indice unico su
// user_thirds(open_id, op), come indiceUnicoDispositivi: con doppioni (un
// account del provider legato a piu' utenti) un error col numero di righe,
// senza sub ne' utenti, e l'avvio continua senza indice; il passo si ripete a
// ogni avvio finche' l'indice non c'e'. Nessuna riga si cancella.
func indiceUnicoAssociazioni() {
	creato, doppi, err := service.AllService.OauthService.CreaIndiceOpenIdUnico()
	switch {
	case err != nil:
		global.Logger.Errorf("indice unico %s non creato, l'avvio continua: %v", service.IndiceOpenIdUnico, err)
	case doppi > 0:
		global.Logger.Errorf("user_thirds ha %d righe che associano lo stesso account OIDC a piu' utenti: l'indice unico %s "+
			"non c'e' e l'avvio continua senza; riprova al prossimo avvio. Per ogni account doppio, l'utente che non deve averlo "+
			"toglie l'associazione dal suo Profilo nel pannello, o si cancella l'utente. Nel README: \"Configurazione\", il "+
			"paragrafo sull'associazione OIDC (user_thirds)", doppi, service.IndiceOpenIdUnico)
	case creato:
		global.Logger.Infof("creato l'indice unico %s: un account OIDC e' legato a un utente solo", service.IndiceOpenIdUnico)
	}
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
			db.Exec("update user_thirds set oauth_type = third_type, op = third_type")
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
		&model.AuditTerminal{},
		&model.AddressBookCollection{},
		&model.AddressBookCollectionRule{},
		&model.DeviceGroup{},
		&model.Evento{},
		&model.TokenIntegrazione{},
		&model.Lasciapassare{},
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
