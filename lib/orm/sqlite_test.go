package orm

import (
	"errors"
	"path/filepath"
	"testing"

	"gorm.io/gorm/logger"
)

// TestApriSqlite prova su una connessione del pool le scelte di ADR-0007:
// WAL, synchronous NORMAL (1), busy_timeout di 5 secondi, chiavi esterne
// accese, e il pool di una connessione sola.
func TestApriSqlite(t *testing.T) {
	db, err := ApriSqlite(filepath.Join(t.TempDir(), "prova.db"), logger.Discard)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	conn, err := sqlDB.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for _, p := range []struct{ pragma, atteso string }{
		{"journal_mode", "wal"},
		{"synchronous", "1"},
		{"busy_timeout", "5000"},
		{"foreign_keys", "1"},
	} {
		var valore string
		if err := conn.QueryRowContext(t.Context(), "PRAGMA "+p.pragma).Scan(&valore); err != nil || valore != p.atteso {
			t.Errorf("PRAGMA %s: %q (%v), atteso %q", p.pragma, valore, err, p.atteso)
		}
	}
	if n := sqlDB.Stats().MaxOpenConnections; n != 1 {
		t.Errorf("MaxOpenConnections %d, attesa 1", n)
	}
}

// TestControlla prova i controlli dell'avvio su un database nuovo, che li
// passa, e su uno in memoria, che SQLite non porta in WAL: resta memory, e
// il controllo lo ferma. I database danneggiati li prova cmd sul binario.
func TestControlla(t *testing.T) {
	for _, tc := range []struct {
		caso, percorso string
		atteso         error
	}{
		{"database nuovo", filepath.Join(t.TempDir(), "nuovo.db"), nil},
		{"database in memoria", ":memory:", ErrNonWAL},
	} {
		db, err := ApriSqlite(tc.percorso, logger.Discard)
		if err != nil {
			t.Fatalf("%s: %v", tc.caso, err)
		}
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatal(err)
		}
		if err := controlla(sqlDB); !errors.Is(err, tc.atteso) {
			t.Errorf("%s: %v, atteso %v", tc.caso, err, tc.atteso)
		}
		if err := sqlDB.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
