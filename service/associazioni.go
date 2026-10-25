package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/lejianwen/rustdesk-api/v2/model"
)

// Un account OIDC senza utente, con l'autoregistrazione spenta, lascia una
// richiesta di associazione col suo codice: il tecnico lo legge nella pagina
// del callback e l'amministratore, dal pannello, associa la richiesta a un
// utente. Le richieste stanno solo in memoria: un riavvio le perde e il
// tecnico rifa' l'accesso.
const (
	// DurataRichiesta e' quanto vale una richiesta di associazione.
	DurataRichiesta = 30 * time.Minute
	// TettoRichieste e' il numero massimo di richieste tenute: una nuova,
	// oltre il tetto, fa uscire la piu' vecchia.
	TettoRichieste = 100
)

// alfabetoCodice sono i caratteri del codice: niente 0/O e 1/I/L, che al
// telefono si confondono.
const alfabetoCodice = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// Rifiuti dell'associazione dall'amministratore; il testo e' l'ID del
// messaggio.
var (
	// ErrCodiceSconosciuto dice che il codice non c'e' o e' scaduto.
	ErrCodiceSconosciuto = errors.New("OauthBindCodeUnknown")
	// ErrUtenteAgente dice che l'utente scelto e' un account agente.
	ErrUtenteAgente = errors.New("OauthBindAgent")
	// ErrUtenteDisattivato dice che l'utente scelto e' disattivato.
	ErrUtenteDisattivato = errors.New("UserDisabled")
)

// RichiestaAssociazione e' un account del provider Op che ha fatto l'accesso
// senza un'associazione a un utente.
type RichiestaAssociazione struct {
	Codice          string // "K7Q2-M9XD"
	Op              string
	Sub             string
	Nome            string
	Email           string
	EmailVerificata bool
	Creata          time.Time
}

// Richieste tiene le richieste di associazione, dalla piu' vecchia.
type Richieste struct {
	mu   sync.Mutex
	voci []*RichiestaAssociazione
	// Ora e' l'orologio delle scadenze; i test lo spostano.
	Ora func() time.Time
}

// RichiesteAssociazione sono le richieste di associazione dell'API.
var RichiesteAssociazione = &Richieste{Ora: time.Now}

// Svuota toglie tutte le richieste e rimette l'orologio vero.
func (r *Richieste) Svuota() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.voci, r.Ora = nil, time.Now
}

// valide toglie le richieste scadute; va chiamata col lock.
func (r *Richieste) valide() {
	limite := r.Ora().Add(-DurataRichiesta)
	r.voci = slices.DeleteFunc(r.voci, func(v *RichiestaAssociazione) bool { return !v.Creata.After(limite) })
}

// NormalizzaCodice toglie spazi e trattini, porta a maiuscole e rimette il
// trattino a meta': "k7q2 m9xd" diventa "K7Q2-M9XD".
func NormalizzaCodice(codice string) string {
	s := strings.Map(func(c rune) rune {
		if c == '-' || c == ' ' {
			return -1
		}
		return c
	}, strings.ToUpper(strings.TrimSpace(codice)))
	if len(s) != 8 {
		return s
	}
	return s[:4] + "-" + s[4:]
}

// nuovoCodice estrae 8 caratteri di alfabetoCodice, a gruppi di 4.
func nuovoCodice() (string, error) {
	b := make([]byte, 0, 9)
	n := big.NewInt(int64(len(alfabetoCodice)))
	for i := range 8 {
		if i == 4 {
			b = append(b, '-')
		}
		k, err := rand.Int(rand.Reader, n)
		if err != nil {
			return "", fmt.Errorf("codice dell'associazione: %w", err)
		}
		b = append(b, alfabetoCodice[k.Int64()])
	}
	return string(b), nil
}

// Nuova crea la richiesta dell'account ou del provider op e ne restituisce
// il codice. Una richiesta dello stesso account si rinnova, con un codice
// nuovo; oltre TettoRichieste esce la piu' vecchia.
func (r *Richieste) Nuova(op string, ou *model.OauthUser) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.valide()
	r.voci = slices.DeleteFunc(r.voci, func(v *RichiestaAssociazione) bool { return v.Op == op && v.Sub == ou.OpenId })
	var codice string
	for codice == "" || slices.ContainsFunc(r.voci, func(v *RichiestaAssociazione) bool { return v.Codice == codice }) {
		var err error
		if codice, err = nuovoCodice(); err != nil {
			return "", err
		}
	}
	if len(r.voci) >= TettoRichieste {
		r.voci = r.voci[len(r.voci)-TettoRichieste+1:]
	}
	r.voci = append(r.voci, &RichiestaAssociazione{Codice: codice, Op: op, Sub: ou.OpenId, Nome: ou.Name,
		Email: ou.Email, EmailVerificata: ou.VerifiedEmail, Creata: r.Ora()})
	return codice, nil
}

// Elenco restituisce una copia delle richieste valide, dalla piu' recente.
func (r *Richieste) Elenco() []RichiestaAssociazione {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.valide()
	res := make([]RichiestaAssociazione, 0, len(r.voci))
	for _, v := range slices.Backward(r.voci) {
		res = append(res, *v)
	}
	return res
}

// cerca restituisce una copia della richiesta valida col codice, o nil.
func (r *Richieste) cerca(codice string) *RichiestaAssociazione {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.valide()
	codice = NormalizzaCodice(codice)
	for _, v := range r.voci {
		if v.Codice == codice {
			c := *v
			return &c
		}
	}
	return nil
}

// Togli toglie la richiesta dell'account sub del provider op, se c'e'.
func (r *Richieste) Togli(op, sub string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.voci = slices.DeleteFunc(r.voci, func(v *RichiestaAssociazione) bool { return v.Op == op && v.Sub == sub })
}

// AssociaRichiesta associa all'utente userId l'account della richiesta col
// codice, con associa, e toglie la richiesta. Oltre ai rifiuti di associa
// rifiuta un codice sconosciuto o scaduto, un utente che non c'e', un
// account agente e un utente disattivato, letti nella stessa transazione.
func (os *OauthService) AssociaRichiesta(codice string, userId uint) (*RichiestaAssociazione, error) {
	r := RichiesteAssociazione.cerca(codice)
	if r == nil {
		return nil, ErrCodiceSconosciuto
	}
	oauthType, err := os.GetTypeByOp(r.Op)
	if err != nil {
		return nil, err
	}
	ut := &model.UserThird{}
	ut.FromOauthUser(userId, &model.OauthUser{OpenId: r.Sub, Name: r.Nome, Email: r.Email, VerifiedEmail: r.EmailVerificata}, oauthType, r.Op)
	err = DB.Transaction(func(tx *gorm.DB) error {
		u := &model.User{}
		if err := tx.Where("id = ?", userId).First(u).Error; err != nil {
			return fmt.Errorf("utente %d: %w", userId, nonTrovato(err))
		}
		switch {
		case u.AgenteDi != 0:
			return fmt.Errorf("utente %d: %w", userId, ErrUtenteAgente)
		case u.Status != model.COMMON_STATUS_ENABLE:
			return fmt.Errorf("utente %d: %w", userId, ErrUtenteDisattivato)
		}
		return associa(tx, ut)
	})
	if err != nil {
		return nil, err
	}
	RichiesteAssociazione.Togli(r.Op, r.Sub)
	return r, nil
}
