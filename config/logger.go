package config

// Logger e' la sezione logger. Path non si usa piu': se e' impostata, un
// warn all'avvio dice che e' ignorata.
type Logger struct {
	Path         string
	Level        string
	ReportCaller bool `mapstructure:"report-caller"`
}
