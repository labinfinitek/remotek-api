package model

// Esiti di un Evento: in attesa di consegna, consegnato, non consegnato
// entro 24 ore.
const (
	EventoInAttesa   = ""
	EventoConsegnato = "consegnato"
	EventoFallito    = "fallito"
)

// Evento e' un evento delle sessioni in coda verso l'integrazione
// (ADR-0026, versione del database 271). Corpo e' il JSON gia' pronto, che
// si firma e si manda com'e'; EventoId e' l'uuid casuale che chi riceve usa
// per scartare i doppioni. Le ore sono secondi UNIX: Prossimo e' quando si
// riprova, Chiuso quando e' stato consegnato o segnato fallito.
type Evento struct {
	IdModel
	EventoId  string `gorm:"size:36;not null;uniqueIndex"`
	Tipo      string `gorm:"size:64;not null"`
	Corpo     string `gorm:"not null"`
	Tentativi int    `gorm:"not null;default:0"`
	Creato    int64  `gorm:"not null"`
	Prossimo  int64  `gorm:"not null;index:idx_eventi_coda,priority:2"`
	Esito     string `gorm:"size:16;not null;default:'';index:idx_eventi_coda,priority:1"`
	Chiuso    int64  `gorm:"not null;default:0"`
}

// TableName e' eventi.
func (Evento) TableName() string { return "eventi" }
