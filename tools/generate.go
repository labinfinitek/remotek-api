//go:build tools

// Package tools raccoglie le direttive go:generate del progetto fuori dal
// pacchetto radice. Upstream le teneva in due file "package main" senza
// func main nella radice del modulo: bastavano a far fallire
// go build, go vet e go test su ./... (REGOLE 10.2).
//
// Il build tag esclude il file da ogni compilazione: con ./... la cartella
// viene ignorata. Per rigenerare i docs swagger, dalla radice del repo e con
// swag nel PATH:
//
//	go generate -tags tools ./tools
//
// swag ricava il percorso di import dei pacchetti da quello della cartella
// di ricerca: la radice non ha piu' file Go, quindi le cartelle si danno una
// per una (-d), ognuna con file Go propri. Una cartella nuova con tipi citati
// dalle annotazioni va aggiunta a -d.
package tools

//go:generate sh -c "cd .. && swag init -g apimain.go -d ./cmd,./http,./model --output docs/api --instanceName api --exclude http/controller/admin"
//go:generate sh -c "cd .. && swag init -g apimain.go -d ./cmd,./http,./model --output docs/admin --instanceName admin --exclude http/controller/api"
