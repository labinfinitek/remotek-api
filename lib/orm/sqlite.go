package orm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// FileSqlite e' il database, relativo alla cartella in cui gira il processo
// (nel container /app/data/rustdeskapi.db, nel volume).
const FileSqlite = "data/rustdeskapi.db"

// dsnSqlite fissa le scelte di ADR-0007 per ogni connessione che il pool
// apre: WAL, synchronous NORMAL, che e' sicuro solo col WAL, fino a 5
// secondi di attesa sul lock di un'altra connessione e chiavi esterne
// controllate. go-sqlite3 legge i parametri dopo "?" e, senza il prefisso
// "file:", li toglie dal percorso.
const dsnSqlite = "?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_foreign_keys=on"

// ErrNonScrivibile: il file o la cartella del database non si scrivono. Col
// DSN lo dice gia' l'apertura quando il passaggio a WAL deve scrivere;
// altrimenti la scrittura di prova, perche' SQLite apre in sola lettura,
// senza errore, un file che l'utente del processo non puo' scrivere, e
// senza la prova l'API partirebbe e sbaglierebbe alla prima scrittura.
// ErrFileNonScrivibile e ErrCartellaNonScrivibile dicono quale dei due,
// quando l'errore di SQLite lo distingue.
var (
	ErrNonScrivibile         = errors.New("scrittura non riuscita")
	ErrFileNonScrivibile     = fmt.Errorf("%w: il file e' in sola lettura per l'utente del processo", ErrNonScrivibile)
	ErrCartellaNonScrivibile = fmt.Errorf("%w: la cartella non e' scrivibile e SQLite non ci crea il journal", ErrNonScrivibile)
)

// ErrNonWAL: dopo l'apertura il database non e' in WAL, e con synchronous
// NORMAL fuori dal WAL una caduta di corrente puo' corromperlo.
// ErrDanneggiato: PRAGMA integrity_check non risponde ok, o SQLite dice il
// database danneggiato (SQLITE_CORRUPT, SQLITE_NOTADB) gia' prima.
var (
	ErrNonWAL      = errors.New("passaggio a WAL non riuscito")
	ErrDanneggiato = errors.New("danno segnalato da SQLite")
)

// sqliteReadonlyDirectory e' SQLITE_READONLY_DIRECTORY, che go-sqlite3 non
// definisce: il file si scriverebbe, la cartella no.
const sqliteReadonlyDirectory = sqlite3.ErrNoExtended(1544)

// ApriSqlite apre il database percorso col DSN di ADR-0007 e una
// connessione sola, lo scrittore unico, senza chiavi esterne nelle
// migrazioni. La usano NewSqlite e i test, che cosi' girano sul database
// della produzione. Gli errori di SQLite che dicono cosa non va nel
// database si classificano come nei controlli di NewSqlite.
func ApriSqlite(percorso string, log logger.Interface) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(percorso+dsnSqlite), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   log,
	})
	if err != nil {
		if errClassificato := classifica(err); errClassificato != nil {
			return nil, errClassificato
		}
		return nil, fmt.Errorf("apertura del database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("connessioni del database: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	return db, nil
}

// NewSqlite apre FileSqlite con ApriSqlite, creando la cartella (0700) se
// manca: un avvio da una cartella nuova non deve fallire per questo. Se la
// cartella non si crea, il database non si apre o non passa i controlli
// (controlla) restituisce l'errore, e chi chiama decide.
func NewSqlite(logwriter logger.Writer) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(FileSqlite), 0o700); err != nil {
		return nil, fmt.Errorf("cartella del database: %w", err)
	}
	db, err := ApriSqlite(FileSqlite, logger.New(
		logwriter, // io writer
		logger.Config{
			SlowThreshold:             time.Second, // Slow SQL threshold
			LogLevel:                  logger.Warn, // Log level
			IgnoreRecordNotFoundError: true,        // Ignore ErrRecordNotFound error for logger
			ParameterizedQueries:      true,        // Don't include params in the SQL log
			Colorful:                  false,
		},
	))
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("connessioni del database: %w", err)
	}
	if err := controlla(sqlDB); err != nil {
		return nil, errors.Join(err, sqlDB.Close())
	}
	return db, nil
}

// controlla fa, in quest'ordine, la scrittura di prova (ErrNonScrivibile),
// il controllo del WAL (ErrNonWAL) e quello di integrita' (ErrDanneggiato),
// che legge tutto il database: 0,24 s a 100.000 righe, 1,58 s a 500.000.
func controlla(sqlDB *sql.DB) error {
	if err := provaScrittura(sqlDB); err != nil {
		return err
	}
	ctx := context.Background()
	var modo string
	if err := sqlDB.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&modo); err != nil {
		return fmt.Errorf("lettura di journal_mode: %w", err)
	}
	if modo != "wal" {
		return fmt.Errorf("%w (journal_mode %s)", ErrNonWAL, modo)
	}
	// Con (1) SQLite si ferma al primo danno e lo restituisce come riga.
	var esito string
	if err := sqlDB.QueryRowContext(ctx, "PRAGMA integrity_check(1)").Scan(&esito); err != nil {
		if errClassificato := classifica(err); errClassificato != nil {
			return errClassificato
		}
		return fmt.Errorf("controllo di integrita': %w", err)
	}
	if esito != "ok" {
		return fmt.Errorf("%w (integrity_check: %s)", ErrDanneggiato, esito)
	}
	return nil
}

// provaScrittura crea una tabella in una transazione e la annulla: la
// scrittura chiede il file aperto in lettura e scrittura e il journal nella
// cartella, e l'annullamento non lascia ne' la tabella ne' il journal.
func provaScrittura(sqlDB *sql.DB) error {
	ctx := context.Background()
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("scrittura di prova: %w", err)
	}
	_, err = tx.ExecContext(ctx, "CREATE TABLE remotek_prova_scrittura (x INTEGER)")
	if errRollback := tx.Rollback(); err == nil && errRollback != nil {
		return fmt.Errorf("annullamento della scrittura di prova: %w", errRollback)
	}
	if err == nil {
		return nil
	}
	if errClassificato := classifica(err); errClassificato != nil {
		return errClassificato
	}
	return fmt.Errorf("%w (%w)", ErrNonScrivibile, err)
}

// classifica riconosce gli errori di SQLite che dicono cosa non va nel
// database, dovunque arrivino (apertura, scrittura di prova, controllo di
// integrita'): file o cartella non scrivibili (ErrNonScrivibile) o
// database danneggiato (ErrDanneggiato). Per gli altri restituisce nil.
func classifica(err error) error {
	var e sqlite3.Error
	switch {
	case !errors.As(err, &e):
		return nil
	case e.ExtendedCode == sqliteReadonlyDirectory:
		return fmt.Errorf("%w (%w)", ErrCartellaNonScrivibile, err)
	case e.ExtendedCode == sqlite3.ErrNoExtended(sqlite3.ErrReadonly):
		return fmt.Errorf("%w (%w)", ErrFileNonScrivibile, err)
	case e.Code == sqlite3.ErrReadonly:
		return fmt.Errorf("%w (%w)", ErrNonScrivibile, err)
	case e.Code == sqlite3.ErrCorrupt || e.Code == sqlite3.ErrNotADB:
		return fmt.Errorf("%w (%w)", ErrDanneggiato, err)
	}
	return nil
}
