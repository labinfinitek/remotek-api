package config

import (
	"os"
)

const (
	DefaultIdServerPort    = 21116
	DefaultRelayServerPort = 21117
)

type Rustdesk struct {
	IdServer        string `mapstructure:"id-server"`
	IdServerPort    int    `mapstructure:"-"`
	RelayServer     string `mapstructure:"relay-server"`
	RelayServerPort int    `mapstructure:"-"`
	ApiServer       string `mapstructure:"api-server"`
	Key             string `mapstructure:"key"`
	KeyFile         string `mapstructure:"key-file"`
	Personal        int    `mapstructure:"personal"`
}

func (rd *Rustdesk) LoadKeyFile() {
	// Load key file
	if rd.Key != "" {
		return
	}
	if rd.KeyFile != "" {
		// Load key from file
		b, err := os.ReadFile(rd.KeyFile)
		if err != nil {
			return
		}
		rd.Key = string(b)
		return
	}
}

// Avvisi restituisce una riga per ogni indirizzo vuoto che serve e che
// nessun errore segnalerebbe: senza id-server il pannello da' al client un
// server ID vuoto, e il client RustDesk usa allora i server pubblici; senza
// api-server la callback OIDC e l'URL di webauth restano senza host, e il
// login fallisce dal provider o dal client. relay-server vuoto no: il client
// prende il relay da hbbs.
func (rd *Rustdesk) Avvisi() []string {
	var avvisi []string
	if rd.IdServer == "" {
		avvisi = append(avvisi, "rustdesk.id-server vuoto: il pannello mostra un server ID vuoto e un client "+
			"impostato cosi' usa i server pubblici di RustDesk. Impostalo con RUSTDESK_API_RUSTDESK_ID_SERVER")
	}
	if rd.ApiServer == "" {
		avvisi = append(avvisi, "rustdesk.api-server vuoto: il pannello mostra un server API vuoto e il login "+
			"OIDC e webauth mandano a indirizzi senza host. Impostalo con RUSTDESK_API_RUSTDESK_API_SERVER")
	}
	return avvisi
}
