package admin

import "github.com/lejianwen/rustdesk-api/v2/model"

// PeerPannello e' una riga della lista dei PC del pannello: il PC col segno
// della copia sospetta (ADR-0024), che non sta nel database e non va al
// client.
type PeerPannello struct {
	*model.Peer
	PossibileCopia bool `json:"possibile_copia"` //nolint:misspell // italiano, nome deciso in ADR-0024
}

// PeerListPannello e' la lista dei PC del pannello, con le pagine di
// model.PeerList.
type PeerListPannello struct {
	Peers []PeerPannello `json:"list"`
	model.Pagination
}
