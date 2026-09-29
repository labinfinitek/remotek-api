// Package logger e' il log dell'API: log/slog in JSON su stdout, una riga
// per evento con time, level, msg e, con report-caller, source. Logger ha i
// metodi in stile printf che il codice usava con logrus.
package logger

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"
)

type Config struct {
	Level        string
	ReportCaller bool
}

// livelli sono i soli valori di logger.level; vuoto vale info.
var livelli = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

// Logger scrive righe JSON con un slog.Handler.
type Logger struct {
	h slog.Handler
}

// New restituisce il logger JSON su stdout. Con un livello non valido
// restituisce comunque un logger, a info, e l'errore: chi chiama ferma
// l'avvio scrivendolo con quel logger.
func New(c *Config) (*Logger, error) {
	return NewSu(os.Stdout, c)
}

// NewSu e' New con le righe scritte in w.
func NewSu(w io.Writer, c *Config) (*Logger, error) {
	livello, ok := livelli[strings.ToLower(c.Level)]
	var err error
	if c.Level == "" {
		livello = slog.LevelInfo
	} else if !ok {
		livello = slog.LevelInfo
		err = fmt.Errorf("logger.level %q non valido: i valori ammessi sono debug, info, warn, error", c.Level)
	}
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{AddSource: c.ReportCaller, Level: livello})
	return &Logger{h: h}, err
}

// Su restituisce un logger JSON su w a livello info, senza source: lo usano
// i test per leggere le righe con Righe.
func Su(w io.Writer) *Logger {
	l, _ := NewSu(w, &Config{Level: "info"})
	return l
}

// Slog restituisce il logger slog sottostante, per le righe con attributi.
func (l *Logger) Slog() *slog.Logger { return slog.New(l.h) }

// chiaveId e' la chiave del request-id nel contesto della richiesta.
type chiaveId struct{}

// ConId restituisce ctx col request-id id, che Per mette nelle righe.
func ConId(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, chiaveId{}, id)
}

// Per restituisce il logger per la richiesta di ctx: le sue righe hanno
// request_id, se ctx ne ha uno (ConId), altrimenti sono quelle di l.
func (l *Logger) Per(ctx context.Context) *Logger {
	id, ok := ctx.Value(chiaveId{}).(string)
	if !ok {
		return l
	}
	return &Logger{h: l.h.WithAttrs([]slog.Attr{slog.String("request_id", id)})}
}

// scrivi scrive msg al livello dato; source e' chi ha chiamato il metodo di
// Logger, non Logger stesso.
func (l *Logger) scrivi(livello slog.Level, msg string) {
	ctx := context.Background()
	if !l.h.Enabled(ctx, livello) {
		return
	}
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:]) // runtime.Callers, scrivi, il metodo di Logger
	r := slog.NewRecord(time.Now(), livello, msg, pcs[0])
	_ = l.h.Handle(ctx, r)
}

func (l *Logger) Debugf(format string, args ...any) {
	l.scrivi(slog.LevelDebug, fmt.Sprintf(format, args...))
}
func (l *Logger) Info(args ...any) { l.scrivi(slog.LevelInfo, fmt.Sprint(args...)) }
func (l *Logger) Infof(format string, args ...any) {
	l.scrivi(slog.LevelInfo, fmt.Sprintf(format, args...))
}
func (l *Logger) Warn(args ...any) { l.scrivi(slog.LevelWarn, fmt.Sprint(args...)) }
func (l *Logger) Warnf(format string, args ...any) {
	l.scrivi(slog.LevelWarn, fmt.Sprintf(format, args...))
}
func (l *Logger) Error(args ...any) { l.scrivi(slog.LevelError, fmt.Sprint(args...)) }
func (l *Logger) Errorf(format string, args ...any) {
	l.scrivi(slog.LevelError, fmt.Sprintf(format, args...))
}

// Fatalf scrive a error e termina il processo con codice 1.
func (l *Logger) Fatalf(format string, args ...any) {
	l.scrivi(slog.LevelError, fmt.Sprintf(format, args...))
	os.Exit(1)
}

// Printf e' il gorm logger.Writer: gorm lo chiama solo per le righe che
// passano il suo livello (Warn), cioe' SQL lento o in errore, quindi a warn.
func (l *Logger) Printf(format string, args ...any) {
	l.scrivi(slog.LevelWarn, fmt.Sprintf(format, args...))
}

// Riga e' una riga del log letta da Righe.
type Riga struct {
	Time      time.Time `json:"time"`
	Level     string    `json:"level"`
	Msg       string    `json:"msg"`
	RequestId string    `json:"request_id"`
}

// Righe legge le righe JSON di testo; le altre (per esempio quelle di gin in
// modalita' debug) le salta.
func Righe(testo string) []Riga {
	var righe []Riga
	s := bufio.NewScanner(strings.NewReader(testo))
	s.Buffer(nil, 1<<20)
	for s.Scan() {
		var r Riga
		if strings.HasPrefix(s.Text(), "{") && json.Unmarshal(s.Bytes(), &r) == nil {
			righe = append(righe, r)
		}
	}
	return righe
}

// Conta dice quante righe di testo hanno il livello dato (DEBUG, INFO, WARN,
// ERROR) e un msg che contiene tutte le parti.
func Conta(testo, livello string, parti ...string) int {
	n := 0
	for _, r := range Righe(testo) {
		if r.Level != livello {
			continue
		}
		tutte := true
		for _, p := range parti {
			tutte = tutte && strings.Contains(r.Msg, p)
		}
		if tutte {
			n++
		}
	}
	return n
}
