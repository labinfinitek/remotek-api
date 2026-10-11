package config

import (
	"errors"
	"net/url"
	"unicode/utf8"
)

// SegretoIntegrazioneMin e' la lunghezza minima, in caratteri, del segreto
// con cui si firmano gli eventi.
const SegretoIntegrazioneMin = 32

// Integrazione e' la destinazione degli eventi delle sessioni (ADR-0026):
// con Url vuoto non si accoda niente. Url e Segreto vengono solo dalla
// configurazione, mai dal pannello; il segreto non va mai nel log.
type Integrazione struct {
	Url     string `mapstructure:"url"`
	Segreto string `mapstructure:"segreto"`
}

// Attiva dice se gli eventi si accodano.
func (i *Integrazione) Attiva() bool { return i.Url != "" }

// Controlla restituisce un errore se, con Url impostato, Url non e'
// https://, ha credenziali o non ha host, o il segreto ha meno di
// SegretoIntegrazioneMin caratteri. Gli errori non contengono ne' l'URL ne'
// il segreto.
func (i *Integrazione) Controlla() error {
	if !i.Attiva() {
		return nil
	}
	u, err := url.Parse(i.Url)
	switch {
	case err != nil:
		return errors.New("integrazione.url non e' un URL valido")
	case u.Scheme != "https":
		return errors.New("integrazione.url deve iniziare con https://")
	case u.User != nil:
		return errors.New("integrazione.url non deve contenere credenziali")
	case u.Host == "":
		return errors.New("integrazione.url non ha l'host")
	case utf8.RuneCountInString(i.Segreto) < SegretoIntegrazioneMin:
		return errors.New("integrazione.segreto deve avere almeno 32 caratteri")
	}
	return nil
}
