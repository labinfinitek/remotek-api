package config

import "strings"

// Marchio predefinito: lo vede chi usa il prodotto senza configurazione.
const (
	DefaultBrandName = "Remotek"
	DefaultBrandDir  = "./resources/brand"
)

// Brand e' il marchio del prodotto in un posto solo: Name e' il nome nel
// titolo del pannello, nel benvenuto e nelle pagine del login OAuth; Dir e'
// la cartella di logo.svg e favicon.svg, che l'API serve su /brand/. Per
// cambiare logo basta sostituire i file, anche montandoli nel container.
type Brand struct {
	Name string `mapstructure:"name"`
	Dir  string `mapstructure:"dir"`
}

// Init mette i default al posto dei valori vuoti, per esempio un
// brand.name: "" nel file.
func (b *Brand) Init() {
	b.Name = strings.TrimSpace(b.Name)
	if b.Name == "" {
		b.Name = DefaultBrandName
	}
	if b.Dir == "" {
		b.Dir = DefaultBrandDir
	}
}
