package logger

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestRigaJSON: ogni riga e' un oggetto JSON con time, level e msg e, con
// ReportCaller, la source di chi ha chiamato, non di Logger.
func TestRigaJSON(t *testing.T) {
	var out strings.Builder
	l, err := NewSu(&out, &Config{Level: "info", ReportCaller: true})
	if err != nil {
		t.Fatal(err)
	}
	l.Warnf("prova %d con \"virgolette\"", 1)
	var riga struct {
		Time, Level, Msg string
		Source           struct{ File string }
	}
	if err := json.Unmarshal([]byte(out.String()), &riga); err != nil {
		t.Fatalf("riga non JSON: %v\n%s", err, out.String())
	}
	if riga.Time == "" || riga.Level != "WARN" || riga.Msg != `prova 1 con "virgolette"` {
		t.Errorf("riga %+v, attesi time, level WARN e il messaggio", riga)
	}
	if !strings.HasSuffix(riga.Source.File, "logger_test.go") {
		t.Errorf("source %q, atteso questo file", riga.Source.File)
	}
}

// TestLivelli: vuoto vale info; i quattro livelli filtrano; un valore non
// valido, anche uno che logrus accettava, e' un errore che nomina i valori
// ammessi, e il logger restituito scrive a info.
func TestLivelli(t *testing.T) {
	for _, tc := range []struct {
		livello string
		righe   []string
	}{
		{"", []string{"INFO", "WARN", "WARN", "ERROR"}},
		{"debug", []string{"DEBUG", "INFO", "WARN", "WARN", "ERROR"}},
		{"INFO", []string{"INFO", "WARN", "WARN", "ERROR"}},
		{"warn", []string{"WARN", "WARN", "ERROR"}},
		{"error", []string{"ERROR"}},
	} {
		var out strings.Builder
		l, err := NewSu(&out, &Config{Level: tc.livello})
		if err != nil {
			t.Fatalf("livello %q: %v", tc.livello, err)
		}
		l.Debugf("d")
		l.Info("i")
		l.Warn("w")
		l.Printf("sql %s", "lento") // gorm: a warn
		l.Errorf("e")
		var livelli []string
		for _, r := range Righe(out.String()) {
			livelli = append(livelli, r.Level)
		}
		if strings.Join(livelli, " ") != strings.Join(tc.righe, " ") {
			t.Errorf("livello %q: righe %v, attese %v", tc.livello, livelli, tc.righe)
		}
	}
	for _, livello := range []string{"trace", "fatal", "panic", "warning", "info+2"} {
		var out strings.Builder
		l, err := NewSu(&out, &Config{Level: livello})
		if err == nil || !strings.Contains(err.Error(), "debug, info, warn, error") {
			t.Errorf("livello %q: errore %v, atteso uno che nomina i valori ammessi", livello, err)
		}
		l.Debugf("d")
		l.Info("i")
		if n := len(Righe(out.String())); n != 1 {
			t.Errorf("livello %q: %d righe, attesa 1 (info)", livello, n)
		}
	}
}
