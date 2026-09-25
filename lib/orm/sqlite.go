package orm

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// FileSqlite e' il database, relativo alla cartella in cui gira il processo
// (nel container /app/data/rustdeskapi.db, nel volume).
const FileSqlite = "data/rustdeskapi.db"

type SqliteConfig struct {
	MaxIdleConns int
	MaxOpenConns int
}

// NewSqlite apre FileSqlite, creando la cartella (0700) se manca: un avvio
// da una cartella nuova non deve fallire per questo. Se la cartella non si
// crea o il database non si apre restituisce l'errore, e chi chiama decide.
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

	return db, nil
}
