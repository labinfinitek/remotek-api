package model

// TokenIntegrazione e' un token con cui l'integrazione (n8n) legge le rotte
// /api/integrazione/ (ADR-0026, versione del database 272). Del token si
// salva solo Hash, lo SHA-256 esadecimale: il token si vede una volta sola,
// quando lo stampa "token-integrazione crea". Le ore sono secondi UNIX;
// Revocato e UltimoUso valgono 0 finche' il token non e' revocato o usato.
type TokenIntegrazione struct {
	IdModel
	Nome      string `gorm:"size:64;not null;uniqueIndex"`
	Hash      string `gorm:"size:64;not null;uniqueIndex"`
	Creato    int64  `gorm:"not null"`
	Scade     int64  `gorm:"not null"`
	Revocato  int64  `gorm:"not null;default:0"`
	UltimoUso int64  `gorm:"not null;default:0"`
}

// TableName e' token_integrazione.
func (TokenIntegrazione) TableName() string { return "token_integrazione" }
