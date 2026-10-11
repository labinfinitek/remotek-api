package service

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

// copieDiProva restituisce una memoria vuota che scrive il log in registro.
func copieDiProva() (*memoriaCopie, *strings.Builder) {
	registro := &strings.Builder{}
	m := nuovaMemoriaCopie()
	m.log = slog.New(slog.NewJSONHandler(registro, nil))
	return m, registro
}

// alterna manda n heartbeat di id ogni passo, alternando due indirizzi, a
// partire da inizio; restituisce l'ora dell'ultimo.
func alterna(m *memoriaCopie, id string, inizio time.Time, passo time.Duration, n int) time.Time {
	ora := inizio
	for i := range n {
		ora = inizio.Add(time.Duration(i) * passo)
		indirizzo := "198.51.100.1"
		if i%2 == 1 {
			indirizzo = "203.0.113.7"
		}
		m.heartbeat(id, indirizzo, ora)
	}
	return ora
}

var inizioCopie = time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)

// TestCopiaAlternata: due copie che mandano l'heartbeat ogni 15 secondi da
// due indirizzi fanno 10 cambi in 150 secondi; il PC diventa sospetto
// all'undicesimo heartbeat, con un solo WARN, senza uuid ne' chiavi. Una
// sequenza sola, col tempo che va solo avanti.
func TestCopiaAlternata(t *testing.T) {
	m, registro := copieDiProva()
	ora := alterna(m, "999000111", inizioCopie, 15*time.Second, SogliaCopie)
	if m.possibileCopia("999000111", ora) || registro.Len() != 0 {
		t.Fatalf("copia sospetta dopo %d cambi, soglia %d:\n%s", SogliaCopie-1, SogliaCopie, registro)
	}
	// SogliaCopie e' pari: l'ultimo heartbeat era dal secondo indirizzo e
	// il prossimo, dal primo, e' il decimo cambio.
	ora = alterna(m, "999000111", ora.Add(15*time.Second), 15*time.Second, 20)
	if !m.possibileCopia("999000111", ora) {
		t.Fatal("30 heartbeat alternati: copia non sospettata")
	}
	if m.possibileCopia("999000222", ora) {
		t.Error("un PC mai visto e' sospetto")
	}
	log := registro.String()
	if n := strings.Count(log, `"msg":"`+MessaggioCopia+`"`); n != 1 {
		t.Fatalf("%d WARN, atteso 1:\n%s", n, log)
	}
	for _, campo := range []string{`"level":"WARN"`, `"id":"999000111"`, `"indirizzo":"198.51.100.1"`, `"indirizzo_precedente":"203.0.113.7"`, `"cambi":10`} {
		if !strings.Contains(log, campo) {
			t.Errorf("nel WARN manca %s:\n%s", campo, log)
		}
	}
}

// TestNessunaCopia: un solo cambio, o cambi a piu' di 60 secondi l'uno
// dall'altro (un portatile che cambia rete), non fanno una copia.
func TestNessunaCopia(t *testing.T) {
	for _, tc := range []struct {
		nome  string
		passo time.Duration
		n     int
	}{
		{"un solo cambio", 15 * time.Second, 2},
		{"cambi distanti", CambioCopie, 40},
		{"cambi distanti piu' di un minuto", 90 * time.Second, 40},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			m, registro := copieDiProva()
			ora := alterna(m, "999000111", inizioCopie, tc.passo, tc.n)
			if m.possibileCopia("999000111", ora) {
				t.Error("copia sospetta")
			}
			if registro.Len() != 0 {
				t.Errorf("nel log:\n%s", registro)
			}
		})
	}
	m, _ := copieDiProva()
	ora := inizioCopie
	for i := range 30 {
		ora = inizioCopie.Add(time.Duration(i) * 15 * time.Second)
		m.heartbeat("999000111", "198.51.100.1", ora)
	}
	if m.possibileCopia("999000111", ora) {
		t.Error("un solo indirizzo: copia sospetta")
	}
}

// TestSecondoAvviso: una copia che resta accesa ha un secondo WARN solo
// un'ora dopo il primo.
func TestSecondoAvviso(t *testing.T) {
	m, registro := copieDiProva()
	inizio := inizioCopie
	alterna(m, "999000111", inizio, 15*time.Second, 11) // primo WARN a 150 s
	primo := inizio.Add(150 * time.Second)
	alterna(m, "999000111", primo.Add(15*time.Second), 15*time.Second, int((AvvisoCopie-15*time.Second)/(15*time.Second))-1)
	if n := strings.Count(registro.String(), MessaggioCopia); n != 1 {
		t.Fatalf("prima di un'ora %d WARN, atteso 1", n)
	}
	m.heartbeat("999000111", "198.51.100.9", primo.Add(AvvisoCopie))
	if n := strings.Count(registro.String(), MessaggioCopia); n != 2 {
		t.Fatalf("dopo un'ora %d WARN, attesi 2", n)
	}
}

// TestAvvisoDopoUscita: una copia che resta zitta piu' di 10 minuti esce
// dalla memoria ma non perde l'ora dell'avviso: se torna prima dell'ora non
// ha un secondo WARN, dopo l'ora si'. L'ora dell'avviso esce dalla memoria
// alla prima pulizia dopo l'ora.
func TestAvvisoDopoUscita(t *testing.T) {
	m, registro := copieDiProva()
	primo := alterna(m, "999000111", inizioCopie, 15*time.Second, SogliaCopie+1)
	m.heartbeat("999000222", "198.51.100.2", primo.Add(20*time.Minute)) // pulizia
	if _, c := m.pc["999000111"]; c {
		t.Fatal("PC zitto da 20 minuti ancora in memoria")
	}
	ora := alterna(m, "999000111", primo.Add(30*time.Minute), 15*time.Second, 2*SogliaCopie)
	if !m.possibileCopia("999000111", ora) {
		t.Fatal("copia tornata: non sospettata")
	}
	if n := strings.Count(registro.String(), MessaggioCopia); n != 1 {
		t.Fatalf("copia tornata dopo 30 minuti: %d WARN, atteso 1:\n%s", n, registro)
	}
	m.heartbeat("999000222", "198.51.100.2", primo.Add(50*time.Minute)) // pulizia
	alterna(m, "999000111", primo.Add(AvvisoCopie), 15*time.Second, SogliaCopie+1)
	if n := strings.Count(registro.String(), MessaggioCopia); n != 2 {
		t.Fatalf("copia tornata dopo un'ora: %d WARN, attesi 2:\n%s", n, registro)
	}
	m.heartbeat("999000222", "198.51.100.2", primo.Add(3*AvvisoCopie))
	if len(m.avvisi) != 0 || len(m.pc) != 1 {
		t.Errorf("dopo tre ore %d avvisi e %d PC in memoria, attesi 0 e 1", len(m.avvisi), len(m.pc))
	}
}

// TestCopiaSparisce: dopo 10 minuti senza cambi il segno sparisce, anche se
// il PC continua a mandare l'heartbeat da un indirizzo solo.
func TestCopiaSparisce(t *testing.T) {
	m, _ := copieDiProva()
	ultimoCambio := alterna(m, "999000111", inizioCopie, 15*time.Second, 11)
	if !m.possibileCopia("999000111", ultimoCambio) {
		t.Fatal("copia non sospettata")
	}
	if !m.possibileCopia("999000111", ultimoCambio.Add(time.Minute)) {
		t.Error("un minuto dopo l'ultimo cambio il segno e' sparito")
	}
	ora := ultimoCambio
	for ora.Sub(ultimoCambio) < FinestraCopie {
		ora = ora.Add(15 * time.Second)
		m.heartbeat("999000111", "198.51.100.1", ora)
	}
	if m.possibileCopia("999000111", ultimoCambio.Add(FinestraCopie)) {
		t.Error("10 minuti senza cambi: ancora sospetta")
	}
}

// TestPuliziaCopie: un PC senza heartbeat da 10 minuti esce dalla memoria,
// alla prima pulizia, che si fa al piu' una volta al minuto.
func TestPuliziaCopie(t *testing.T) {
	m, _ := copieDiProva()
	m.heartbeat("999000111", "198.51.100.1", inizioCopie)
	m.heartbeat("999000222", "198.51.100.2", inizioCopie.Add(FinestraCopie-30*time.Second))
	if len(m.pc) != 2 {
		t.Fatalf("%d PC in memoria, attesi 2", len(m.pc))
	}
	m.heartbeat("999000222", "198.51.100.2", inizioCopie.Add(FinestraCopie+30*time.Second))
	if _, c := m.pc["999000111"]; c {
		t.Error("PC senza heartbeat da 10 minuti ancora in memoria")
	}
	s := func(n time.Duration) time.Time { return inizioCopie.Add(n * time.Second) }
	m.heartbeat("999000333", "198.51.100.3", s(1200)) // pulizia: 999000222 ha 570 s
	m.heartbeat("999000444", "198.51.100.4", s(1240)) // 999000222 ha 610 s, pulizia di 40 s fa
	if len(m.pc) != 3 {
		t.Errorf("pulizia prima di un minuto dalla precedente: %d PC, attesi 3", len(m.pc))
	}
	m.heartbeat("999000444", "198.51.100.4", s(1260))
	if _, c := m.pc["999000222"]; c || len(m.pc) != 2 {
		t.Errorf("dopo la pulizia %d PC, attesi 2 senza 999000222", len(m.pc))
	}
}
