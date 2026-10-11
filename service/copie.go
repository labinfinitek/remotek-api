package service

import (
	"log/slog"
	"sync"
	"time"
)

// Rilevazione delle copie di un PC (ADR-0024): un PC copiato senza sysprep
// porta con se' ID, uuid e chiave, e i due heartbeat arrivano sulla stessa
// riga da due indirizzi. Il README ("Copie dello stesso PC") spiega regola e
// limiti. Si segnala soltanto: nessuna richiesta viene bloccata.
const (
	// CambioCopie e' l'intervallo massimo fra due heartbeat dello stesso PC
	// da indirizzi diversi perche' contino come un cambio: il client ne
	// manda uno ogni 15 secondi, un portatile che cambia rete no.
	CambioCopie = 60 * time.Second
	// FinestraCopie e' il periodo in cui si contano i cambi, ed e' anche
	// quanto un PC senza heartbeat resta in memoria.
	FinestraCopie = 10 * time.Minute
	// SogliaCopie e' il numero di cambi nella finestra che fa sospettare
	// una copia del PC.
	SogliaCopie = 10
	// AvvisoCopie e' l'intervallo minimo fra due avvisi nel log per lo
	// stesso PC.
	AvvisoCopie = time.Hour
	// MessaggioCopia e' il messaggio fisso del warn.
	MessaggioCopia = "possibile copia del PC" //nolint:misspell // italiano, testo deciso in ADR-0024
)

// pcVisto e' cio' che la memoria sa di un PC: l'indirizzo dell'ultimo
// heartbeat e quello prima, l'ora dell'ultimo e i cambi nella finestra.
type pcVisto struct {
	indirizzo, precedente string
	ultimo                time.Time
	cambi                 []time.Time
}

// memoriaCopie ricorda i PC riconosciuti con heartbeat negli ultimi
// FinestraCopie, e l'ora dell'ultimo avviso di ogni PC per AvvisoCopie,
// anche se il PC e' uscito dalla memoria: una copia zitta per piu' di
// FinestraCopie non ha un secondo avviso prima dell'ora. E' in processo,
// quindi dopo un riavvio riparte vuota.
type memoriaCopie struct {
	mu     sync.Mutex
	pc     map[string]*pcVisto
	avvisi map[string]time.Time
	pulita time.Time
	log    *slog.Logger // nil: il Logger del servizio
}

var copieViste = nuovaMemoriaCopie()

func nuovaMemoriaCopie() *memoriaCopie {
	return &memoriaCopie{pc: map[string]*pcVisto{}, avvisi: map[string]time.Time{}}
}

// SegnaHeartbeat registra l'heartbeat del PC id, gia' riconosciuto, arrivato
// da indirizzo all'ora data.
func SegnaHeartbeat(id, indirizzo string, ora time.Time) (avviso bool, cambi int) {
	return copieViste.heartbeat(id, indirizzo, ora)
}

// PossibileCopia dice se il PC id, all'ora data, ha almeno SogliaCopie cambi
// di indirizzo negli ultimi FinestraCopie.
func PossibileCopia(id string, ora time.Time) bool {
	return copieViste.possibileCopia(id, ora)
}

// heartbeat dice se ha scritto il warn MessaggioCopia, e con quanti cambi.
func (m *memoriaCopie) heartbeat(id, indirizzo string, ora time.Time) (avviso bool, cambi int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ora.Sub(m.pulita) >= time.Minute {
		for k, p := range m.pc {
			if ora.Sub(p.ultimo) >= FinestraCopie {
				delete(m.pc, k)
			}
		}
		for k, a := range m.avvisi {
			if ora.Sub(a) >= AvvisoCopie {
				delete(m.avvisi, k)
			}
		}
		m.pulita = ora
	}
	p, c := m.pc[id]
	if !c {
		m.pc[id] = &pcVisto{indirizzo: indirizzo, ultimo: ora}
		return false, 0
	}
	if indirizzo != p.indirizzo {
		if ora.Sub(p.ultimo) < CambioCopie {
			p.cambi = append(p.cambi, ora)
		}
		p.precedente, p.indirizzo = p.indirizzo, indirizzo
	}
	p.ultimo = ora
	p.recenti(ora)
	if a, c := m.avvisi[id]; len(p.cambi) >= SogliaCopie && (!c || ora.Sub(a) >= AvvisoCopie) {
		m.avvisi[id] = ora
		m.logger().Warn(MessaggioCopia, "id", id, "indirizzo", p.indirizzo,
			"indirizzo_precedente", p.precedente, "cambi", len(p.cambi))
		return true, len(p.cambi)
	}
	return false, len(p.cambi)
}

func (m *memoriaCopie) possibileCopia(id string, ora time.Time) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, c := m.pc[id]
	if !c {
		return false
	}
	p.recenti(ora)
	return len(p.cambi) >= SogliaCopie
}

// recenti toglie i cambi piu' vecchi di FinestraCopie.
func (p *pcVisto) recenti(ora time.Time) {
	i := 0
	for i < len(p.cambi) && ora.Sub(p.cambi[i]) >= FinestraCopie {
		i++
	}
	p.cambi = p.cambi[i:]
}

func (m *memoriaCopie) logger() *slog.Logger {
	if m.log != nil {
		return m.log
	}
	if Logger != nil {
		return Logger.Slog()
	}
	return slog.Default()
}
