package service

import (
	"errors"
	"io"
	"net"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/lejianwen/rustdesk-api/v2/config"
	applog "github.com/lejianwen/rustdesk-api/v2/lib/logger"
	"github.com/lejianwen/rustdesk-api/v2/lib/orm"
	"github.com/lejianwen/rustdesk-api/v2/model"
)

// registroDiProva fa scrivere il log dei servizi in un registro, che
// restituisce.
func registroDiProva(t *testing.T) *strings.Builder {
	t.Helper()
	registro := &strings.Builder{}
	prec := Logger
	Logger = applog.Su(registro)
	t.Cleanup(func() { Logger = prec })
	return registro
}

// serverLdapCheRifiuta avvia un server LDAP finto che a ogni bind risponde
// invalidCredentials (49) e ne restituisce l'indirizzo ldap://.
func serverLdapCheRifiuta(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go rispondiAlBind(c)
		}
	}()
	return "ldap://" + ln.Addr().String()
}

// rispondiAlBind legge da c una richiesta LDAP, risponde al suo messageID
// con un BindResponse invalidCredentials e aspetta che il client chiuda.
func rispondiAlBind(c net.Conn) {
	defer func() { _ = c.Close() }()
	// La richiesta e' una SEQUENCE (0x30) con la lunghezza in forma breve o
	// lunga; il suo primo campo e' il messageID, un INTEGER (0x02).
	testa := make([]byte, 2)
	if _, err := io.ReadFull(c, testa); err != nil || testa[0] != 0x30 {
		return
	}
	n := int(testa[1])
	if n&0x80 != 0 {
		lunghezza := make([]byte, n&0x7f)
		if _, err := io.ReadFull(c, lunghezza); err != nil {
			return
		}
		n = 0
		for _, b := range lunghezza {
			n = n<<8 | int(b)
		}
	}
	corpo := make([]byte, n)
	if _, err := io.ReadFull(c, corpo); err != nil || n < 2 || corpo[0] != 0x02 || n < 2+int(corpo[1]) {
		return
	}
	id := corpo[:2+int(corpo[1])]
	// BindResponse, [APPLICATION 1]: resultCode 49, matchedDN e
	// diagnosticMessage vuoti.
	risposta := append(append([]byte{}, id...), 0x61, 0x07, 0x0a, 0x01, 49, 0x04, 0x00, 0x04, 0x00)
	if _, err := c.Write(append([]byte{0x30, byte(len(risposta))}, risposta...)); err != nil {
		return
	}
	_, _ = io.Copy(io.Discard, c)
}

// TestBindLdapRifiutato prova che un bind LDAP rifiutato va nel log dei
// servizi col motivo e che connectAndBind restituisce ErrLdapBindService:
// prima "Bind failed" si stampava su stdout, fuori dal log e senza motivo.
func TestBindLdapRifiutato(t *testing.T) {
	registro := registroDiProva(t)
	cfg := &config.Ldap{Url: serverLdapCheRifiuta(t)}
	conn, err := (&LdapService{}).connectAndBind(cfg, "cn=servizio,dc=esempio,dc=it", "password-sbagliata")
	if conn != nil || !errors.Is(err, ErrLdapBindService) {
		t.Fatalf("connectAndBind: connessione %v, errore %v; attesi nessuna connessione e ErrLdapBindService", conn, err)
	}
	if nelLog := registro.String(); applog.Conta(nelLog, "WARN", "Invalid Credentials") == 0 {
		t.Errorf("nel log manca il bind rifiutato col motivo:\n%s", nelLog)
	}
}

// TestUserAccountControlNonNumerico prova che un userAccountControl che non
// e' un numero rende l'utente disabilitato e va nel log dei servizi a
// livello error, col nome dell'utente e il valore: prima si stampava su
// stdout.
func TestUserAccountControlNonNumerico(t *testing.T) {
	registro := registroDiProva(t)
	cfg := &config.Ldap{User: config.LdapUser{EnableAttr: "userAccountControl", EnableAttrValue: "512"}}
	lu := &LdapUser{Username: "mrossi", EnableAttrValue: "non-numero"}
	if (&LdapService{}).isUserEnabled(cfg, lu) || lu.Enabled {
		t.Errorf("utente con userAccountControl %q: abilitato, atteso disabilitato", lu.EnableAttrValue)
	}
	if nelLog := registro.String(); applog.Conta(nelLog, "ERROR", "mrossi", "non-numero") != 1 {
		t.Errorf("nel log manca l'errore di userAccountControl:\n%s", nelLog)
	}
}

// TestLdapUtenteLocaleNonLetto prova il passaggio da un utente LDAP
// autenticato all'utente locale quando il database non legge gli utenti:
// mapToLocalUser restituisce l'errore del database e non crea nessun utente.
// Prima la lettura fallita valeva "utente che non c'e'": nasceva l'utente e
// tornava un utente vuoto senza errore, che il login prendeva per una
// password sbagliata e contava per captcha e ban.
func TestLdapUtenteLocaleNonLetto(t *testing.T) {
	db, err := orm.ApriSqlite(filepath.Join(t.TempDir(), "api.db"), logger.Discard)
	if err == nil {
		err = db.AutoMigrate(&model.User{})
	}
	if err == nil {
		err = db.Callback().Query().Before("gorm:query").Register("rifiuta_utenti", func(tx *gorm.DB) {
			if tx.Statement.Table == "users" {
				_ = tx.AddError(errors.New("lettura rifiutata dal test"))
			}
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	prec := DB
	DB = db
	t.Cleanup(func() { DB = prec })

	u, err := (&LdapService{}).mapToLocalUser(&config.Ldap{}, &LdapUser{Username: "mrossi", Enabled: true})
	if u != nil || err == nil || !strings.Contains(err.Error(), "lettura rifiutata dal test") {
		t.Errorf("mapToLocalUser con gli utenti non letti: utente %+v, errore %v; attesi nessun utente e l'errore del database", u, err)
	}
	var n int64
	if err := db.Raw("SELECT count(*) FROM users").Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("utenti dopo mapToLocalUser: %d, atteso nessuno", n)
	}
}

// TestLdapAgenteNonAdmin prova che la sincronizzazione LDAP non rende
// amministratore un agente AI anche se e' nel gruppo degli amministratori:
// il ruolo resta com'e' e un warn lo dice; una persona nello stesso gruppo
// diventa amministratore come prima.
func TestLdapAgenteNonAdmin(t *testing.T) {
	registro := registroDiProva(t)
	db, err := orm.ApriSqlite(filepath.Join(t.TempDir(), "api.db"), logger.Discard)
	if err == nil {
		err = db.AutoMigrate(&model.User{})
	}
	if err != nil {
		t.Fatal(err)
	}
	prec := DB
	DB = db
	t.Cleanup(func() { DB = prec })
	no := false
	for _, u := range []*model.User{
		{Username: "mario", IsAdmin: &no, Status: model.COMMON_STATUS_ENABLE},
		{Username: "agente-mario", IsAdmin: &no, Status: model.COMMON_STATUS_ENABLE, AgenteDi: 1},
	} {
		if err := db.Create(u).Error; err != nil {
			t.Fatal(err)
		}
	}
	cfg := &config.Ldap{User: config.LdapUser{AdminGroup: "cn=admin,dc=esempio,dc=it", Sync: true}}
	for _, nome := range []string{"agente-mario", "mario"} {
		lu := &LdapUser{Username: nome, Enabled: true, MemberOf: []string{"cn=admin,dc=esempio,dc=it"}}
		if _, err := (&LdapService{}).mapToLocalUser(cfg, lu); err != nil {
			t.Fatalf("mapToLocalUser di %s: %v", nome, err)
		}
	}
	var admin []string
	if err := db.Raw("SELECT username FROM users WHERE is_admin ORDER BY id").Scan(&admin).Error; err != nil {
		t.Fatal(err)
	}
	if len(admin) != 1 || admin[0] != "mario" {
		t.Errorf("amministratori dopo la sincronizzazione: %v, atteso solo mario", admin)
	}
	if nelLog := registro.String(); applog.Conta(nelLog, "WARN", "utente 2 e' un agente AI") != 1 {
		t.Errorf("nel log manca il warn sull'agente:\n%s", nelLog)
	}
}
