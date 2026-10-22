package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/lejianwen/rustdesk-api/v2/config"
	"github.com/lejianwen/rustdesk-api/v2/model"
)

// Tempi della consegna degli eventi (README, Integrazione).
const (
	attesaPrima       = 30 * time.Second
	attesaMax         = time.Hour
	durataTentativi   = 24 * time.Hour
	conservazione     = 7 * 24 * time.Hour
	timeoutConsegna   = 10 * time.Second
	intervalloCoda    = 5 * time.Second
	intervalloPulizia = time.Hour
)

// Consegna manda gli eventi della coda all'URL dell'integrazione, uno alla
// volta, in ordine di creazione tra quelli pronti. Ora e' l'orologio, finto
// nei test; intervallo e' l'attesa di gira quando non c'e' niente da
// mandare, piu' corta nei test.
type Consegna struct {
	url, segreto string
	client       *http.Client
	Ora          func() time.Time
	pulita       time.Time
	intervallo   time.Duration
}

// NuovaConsegna prepara la consegna verso c: POST con timeout di 10
// secondi, senza seguire i redirect, che valgono come non consegnato.
func NuovaConsegna(c config.Integrazione) *Consegna {
	return &Consegna{url: c.Url, segreto: c.Segreto, Ora: time.Now, intervallo: intervalloCoda, client: &http.Client{
		Timeout:       timeoutConsegna,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

// AvviaConsegna fa girare la consegna verso c in una goroutine e
// restituisce la funzione che la ferma e ne aspetta la fine; con
// l'integrazione spenta non avvia niente. La coda e' nel database: dopo un
// riavvio riparte da dove era.
func AvviaConsegna(c config.Integrazione) (ferma func()) {
	if !c.Attiva() {
		return func() {}
	}
	ctx, cancel := context.WithCancel(context.Background())
	fatto := make(chan struct{})
	k := NuovaConsegna(c)
	go func() {
		defer close(fatto)
		k.gira(ctx)
	}()
	Logger.Info("consegna degli eventi dell'integrazione avviata")
	return func() {
		cancel()
		<-fatto
	}
}

// gira manda gli eventi pronti finche' ce ne sono, poi aspetta
// k.intervallo, finche' ctx non e' chiuso. Un guasto della coda scrive una
// riga ERROR quando comincia o cambia messaggio, e una INFO quando finisce,
// non una riga a ogni giro.
func (k *Consegna) gira(ctx context.Context) {
	guasto := ""
	for {
		for ctx.Err() == nil {
			mandato, err := k.Passo(ctx)
			guasto = segnalaGuasto(guasto, err)
			if !mandato || err != nil {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(k.intervallo):
		}
	}
}

// segnalaGuasto scrive la riga del guasto della coda err, se diverso da
// quello di prima, o quella del ritorno se err e' nil dopo un guasto;
// restituisce il guasto attuale, "" se non ce n'e'.
func segnalaGuasto(prima string, err error) string {
	adesso := ""
	if err != nil {
		adesso = err.Error()
	}
	switch {
	case adesso == prima:
	case adesso != "":
		Logger.Errorf("coda degli eventi: %s", adesso)
	default:
		Logger.Info("coda degli eventi: di nuovo raggiungibile")
	}
	return adesso
}

// Passo toglie dalla coda, una volta all'ora, gli eventi conclusi da piu' di
// 7 giorni, e manda il primo evento pronto; dice se ne ha mandato uno.
func (k *Consegna) Passo(ctx context.Context) (bool, error) {
	adesso := k.Ora()
	if adesso.Sub(k.pulita) >= intervalloPulizia {
		err := DB.Where("esito <> ? AND chiuso < ?", model.EventoInAttesa, adesso.Add(-conservazione).Unix()).
			Delete(&model.Evento{}).Error
		if err != nil {
			return false, fmt.Errorf("pulizia della coda: %w", err)
		}
		k.pulita = adesso
	}
	e := &model.Evento{}
	err := DB.Where("esito = ? AND prossimo <= ?", model.EventoInAttesa, adesso.Unix()).Order("id").First(e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("prossimo evento: %w", err)
	}
	motivo := k.manda(ctx, e, adesso)
	if ctx.Err() != nil {
		return false, nil // fermata: il tentativo non conta
	}
	return true, k.esito(e, motivo, adesso)
}

// manda fa il POST dell'evento e; restituisce "" se la risposta e' 2xx,
// altrimenti il motivo, senza l'URL.
func (k *Consegna) manda(ctx context.Context, e *model.Evento, adesso time.Time) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, k.url, strings.NewReader(e.Corpo))
	if err != nil {
		return "richiesta non costruita"
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Remotek-Evento-Id", e.EventoId)
	req.Header.Set("X-Remotek-Evento-Firma", Firma(k.segreto, adesso.Unix(), []byte(e.Corpo)))
	res, err := k.client.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err // l'URL resta fuori dal log
		}
		return err.Error()
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
	_ = res.Body.Close()
	if res.StatusCode/100 == 2 {
		return ""
	}
	return "stato " + strconv.Itoa(res.StatusCode)
}

// esito salva l'esito del tentativo: consegnato, da riprovare con
// l'attesa successiva, o fallito se l'evento ha 24 ore, con la riga ERROR
// "evento non consegnato".
func (k *Consegna) esito(e *model.Evento, motivo string, adesso time.Time) error {
	n := e.Tentativi + 1
	agg := map[string]any{"tentativi": n}
	limite := time.Unix(e.Creato, 0).Add(durataTentativi)
	switch {
	case motivo == "":
		agg["esito"], agg["chiuso"] = model.EventoConsegnato, adesso.Unix()
	case !adesso.Before(limite):
		agg["esito"], agg["chiuso"] = model.EventoFallito, adesso.Unix()
	default:
		agg["prossimo"] = min(adesso.Add(attesa(n)).Unix(), limite.Unix())
	}
	if err := DB.Model(e).Updates(agg).Error; err != nil {
		return fmt.Errorf("esito dell'evento %s: %w", e.EventoId, err)
	}
	switch {
	case agg["esito"] == model.EventoFallito:
		Logger.Errorf("evento non consegnato: tipo %s, id %s, %d tentativi in 24 ore, ultimo: %s", e.Tipo, e.EventoId, n, motivo)
	case motivo != "":
		Logger.Warnf("evento %s di tipo %s non consegnato al tentativo %d (%s): si riprova", e.EventoId, e.Tipo, n, motivo)
	}
	return nil
}

// attesa e' l'attesa dopo l'n-esimo tentativo fallito: 30 secondi,
// raddoppiando, al massimo un'ora.
func attesa(n int) time.Duration {
	d := attesaPrima
	for i := 1; i < n && d < attesaMax; i++ {
		d *= 2
	}
	return min(d, attesaMax)
}
