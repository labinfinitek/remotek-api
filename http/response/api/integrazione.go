package api

// Risposte delle rotte /api/integrazione/ (README, Integrazione): mai
// uuid, chiavi, IP, guid o session_id.

// PcIntegrazione e' un PC come lo legge l'integrazione. UltimoContatto e'
// in RFC 3339, UTC, null se il PC non ha mai mandato un heartbeat.
type PcIntegrazione struct {
	Id             string  `json:"id"`
	Nome           string  `json:"nome"`
	Alias          string  `json:"alias"`
	UltimoContatto *string `json:"ultimo_contatto"`
	Versione       string  `json:"versione"`
	PossibileCopia bool    `json:"possibile_copia"` //nolint:misspell // italiano, nome deciso in ADR-0024
}

// ElencoPcIntegrazione e' una pagina dei PC.
type ElencoPcIntegrazione struct {
	Elenco    []PcIntegrazione `json:"elenco"`
	Totale    int64            `json:"totale"`
	Pagina    int              `json:"pagina"`
	PerPagina int              `json:"per_pagina"`
}

// ConnessioneIntegrazione e' una riga del registro delle connessioni, con i
// campi della sessione degli eventi: Id e' quello della riga, da passare
// alla verifica della trascrizione; Fine e DurataS sono null finche' la
// connessione e' aperta.
type ConnessioneIntegrazione struct {
	Id      uint    `json:"id"`
	Pc      string  `json:"pc"`
	ConnId  int64   `json:"conn_id"`
	Tipo    int     `json:"tipo"`
	Tecnico string  `json:"tecnico"`
	Inizio  string  `json:"inizio"`
	Fine    *string `json:"fine"`
	DurataS *int64  `json:"durata_s"`
	Nota    string  `json:"nota"`
}

// ElencoConnessioniIntegrazione e' una pagina del registro delle connessioni.
type ElencoConnessioniIntegrazione struct {
	Elenco    []ConnessioneIntegrazione `json:"elenco"`
	Totale    int64                     `json:"totale"`
	Pagina    int                       `json:"pagina"`
	PerPagina int                       `json:"per_pagina"`
}

// VerificaIntegrazione e' l'esito della verifica della catena di una
// trascrizione, come in /api/admin/audit_conn/terminal/verify.
type VerificaIntegrazione struct {
	Integra     bool   `json:"integra"`
	Blocchi     int64  `json:"blocchi"`
	Fine        bool   `json:"fine"`
	Hash        string `json:"hash"`
	PrimoErrato int64  `json:"primo_errato"`
}
