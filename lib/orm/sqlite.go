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

// ErrNonScrivibile: il database si apre ma la scrittura di prova non riesce.
// SQLite apre in sola lettura, senza errore, un file che l'utente del
// processo non puo' scrivere, e senza la cartella scrivibile non crea il
// journal: senza la prova l'API partirebbe e sbaglierebbe alla prima
// scrittura. ErrFileNonScrivibile e ErrCartellaNonScrivibile dicono quale
// dei due, quando l'errore di SQLite lo distingue.
var (
	ErrNonScrivibile         = errors.New("scrittura di prova non riuscita")
	ErrFileNonScrivibile     = fmt.Errorf("%w: il file e' in sola lettura per l'utente del processo", ErrNonScrivibile)
	ErrCartellaNonScrivibile = fmt.Errorf("%w: la cartella non e' scrivibile e SQLite non ci crea il journal", ErrNonScrivibile)
)

// sqliteReadonlyDirectory e' SQLITE_READONLY_DIRECTORY, che go-sqlite3 non
// definisce: il file si scriverebbe, la cartella no.
const sqliteReadonlyDirectory = sqlite3.ErrNoExtended(1544)

type SqliteConfig struct {
	MaxIdleConns int
	MaxOpenConns int
}

// NewSqlite apre FileSqlite, creando la cartella (0700) se manca: un avvio
// da una cartella nuova non deve fallire per questo. Se la cartella non si
// crea, il database non si apre o la scrittura di prova non riesce
// (ErrNonScrivibile) restituisce l'errore, e chi chiama decide.
func NewSqlite(sqliteConf *SqliteConfig, logwriter logger.Writer) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(FileSqlite), 0o700); err != nil {
		return nil, fmt.Errorf("cartella del database: %w", err)
	}
	db, err := gorm.Open(sqlite.Open(FileSqlite), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger: logger.New(
			logwriter, // io writer
			logger.Config{
				SlowThreshold:             time.Second, // Slow SQL threshold
				LogLevel:                  logger.Warn, // Log level
				IgnoreRecordNotFoundError: true,        // Ignore ErrRecordNotFound error for logger
				ParameterizedQueries:      true,        // Don't include params in the SQL log
				Colorful:                  true,
			},
		),
	})
	if err != nil {
		return nil, fmt.Errorf("apertura del database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("connessioni del database: %w", err)
	}
	// SetMaxIdleConns 设置空闲连接池中连接的最大数量
	sqlDB.SetMaxIdleConns(sqliteConf.MaxIdleConns)

	// SetMaxOpenConns 设置打开数据库连接的最大数量。
	sqlDB.SetMaxOpenConns(sqliteConf.MaxOpenConns)

	if err := provaScrittura(sqlDB); err != nil {
		return nil, errors.Join(err, sqlDB.Close())
	}
	return db, nil
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
	var e sqlite3.Error
	switch {
	case err == nil:
		return nil
	case errors.As(err, &e) && e.ExtendedCode == sqliteReadonlyDirectory:
		return fmt.Errorf("%w (%w)", ErrCartellaNonScrivibile, err)
	case errors.As(err, &e) && e.ExtendedCode == sqlite3.ErrNoExtended(sqlite3.ErrReadonly):
		return fmt.Errorf("%w (%w)", ErrFileNonScrivibile, err)
	default:
		return fmt.Errorf("%w (%w)", ErrNonScrivibile, err)
	}
}
