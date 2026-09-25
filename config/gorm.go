package config

// TypeSqlite e' l'unico database dell'API (A3): gorm.type vale "sqlite", e
// vuoto vale lo stesso. Un altro valore ferma l'avvio (cmd/apimain.go).
const TypeSqlite = "sqlite"

type Gorm struct {
	Type         string `mapstructure:"type"`
	MaxIdleConns int    `mapstructure:"max-idle-conns"`
	MaxOpenConns int    `mapstructure:"max-open-conns"`
}
