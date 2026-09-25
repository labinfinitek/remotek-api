package logger

import (
	"fmt"
	"io"
	"os"

	nested "github.com/antonfisher/nested-logrus-formatter"
	log "github.com/sirupsen/logrus"
)

const (
	DebugMode   = "debug"
	ReleaseMode = "release"
)

type Config struct {
	Path         string
	Level        string
	ReportCaller bool
}

func New(c *Config) *log.Logger {
	log.SetFormatter(&nested.Formatter{
		// HideKeys:        true,
		TimestampFormat: "[2006-01-02 15:04:05]",
		NoColors:        true,
		NoFieldsColors:  true,
		//FieldsOrder:     []string{"name", "age"},
	})

	// Il file di log contiene nomi utente e indirizzi IP: 0600, anche se
	// esisteva gia' con permessi piu' larghi. Chmod li porta a 0600 esatti
	// anche con una umask insolita.
	f := c.Path
	var write io.Writer
	if f != "" {
		fwriter, err := os.OpenFile(f, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
		if err == nil {
			err = fwriter.Chmod(0o600)
		}
		if err != nil {
			panic(fmt.Errorf("apertura del file di log %s: %w", f, err))
		}
		write = io.MultiWriter(fwriter, os.Stdout)
	} else {
		write = os.Stdout
	}

	log.SetOutput(write)

	log.SetReportCaller(c.ReportCaller)

	level, err2 := log.ParseLevel(c.Level)
	if err2 != nil {
		level = log.DebugLevel
	}
	log.SetLevel(level)

	return log.StandardLogger()
}
