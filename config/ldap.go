package config

import "net/url"

type LdapUser struct {
	BaseDn          string `mapstructure:"base-dn"`           // The base DN of the user for searching
	EnableAttr      string `mapstructure:"enable-attr"`       // The attribute name of the user for enabling, in AD it is "userAccountControl", empty means no enable attribute, all users are enabled
	EnableAttrValue string `mapstructure:"enable-attr-value"` // The value of the enable attribute when the user is enabled. If you are using AD, just leave it random str, it will be ignored.
	Filter          string `mapstructure:"filter"`
	Username        string `mapstructure:"username"`
	Email           string `mapstructure:"email"`
	FirstName       string `mapstructure:"first-name"`
	LastName        string `mapstructure:"last-name"`
	Sync            bool   `mapstructure:"sync"`        // Will sync the user's information to the internal database
	AdminGroup      string `mapstructure:"admin-group"` // Which group is the admin group
	AllowGroup      string `mapstructure:"allow-group"` // Which group is allowed to login
}

// type LdapGroup struct {
// 	BaseDn 		string            `mapstructure:"base-dn"` // The base DN of the group for searching
// 	Name        string            `mapstructure:"name"`    // The attribute name of the group
// 	Filter      string            `mapstructure:"filter"`
// 	Admin       string            `mapstructure:"admin"`   // Which group is the admin group
// 	Member      string            `mapstructure:"member"`  // How to get the member of the group: member, uniqueMember, or memberOf (default: member)
// 	Mode        string            `mapstructure:"mode"`
// 	Map         map[string]string `mapstructure:"map"`     // If mode is "map", map the LDAP group to the internal group
// }

type Ldap struct {
	Enable       bool     `mapstructure:"enable"`
	Url          string   `mapstructure:"url"`
	TlsCaFile    string   `mapstructure:"tls-ca-file"`
	TlsVerify    bool     `mapstructure:"tls-verify"`
	BaseDn       string   `mapstructure:"base-dn"`
	BindDn       string   `mapstructure:"bind-dn"`
	BindPassword string   `mapstructure:"bind-password"`
	User         LdapUser `mapstructure:"user"`
	// Group        LdapGroup `mapstructure:"group"`
}

// Avvisi restituisce una riga per ogni scelta di l che, con LDAP acceso,
// espone le credenziali: ldap:// manda in chiaro sulla rete le password del
// bind e degli utenti, ldaps:// con tls-verify false non verifica i
// certificati del server. Le righe non contengono l'url, che puo' avere
// utente e password.
func (l *Ldap) Avvisi() []string {
	if !l.Enable {
		return nil
	}
	u, err := url.Parse(l.Url)
	if err != nil {
		return nil // il login LDAP fallisce comunque, con l'errore nel log
	}
	switch u.Scheme { // url.Parse lo rende minuscolo
	case "ldap":
		return []string{"LDAP: ldap.url usa ldap:// senza TLS, le password del bind e degli utenti " +
			"passano in chiaro sulla rete: usa ldaps://"}
	case "ldaps":
		if !l.TlsVerify {
			return []string{"LDAP: ldap.tls-verify e' false, con ldaps:// i certificati del server non si " +
				"verificano: per una CA interna usa ldap.tls-ca-file"}
		}
	}
	return nil
}
