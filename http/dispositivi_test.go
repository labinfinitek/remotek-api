package http

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// rifiutaLetture fa fallire nel database dei servizi le letture (First,
// Find, Count, Pluck) della tabella tabella la cui WHERE, scritta con
// fmt.Sprint, contiene dove; con dove vuoto tutte. Le scritture e le query
// con Raw riescono: cosi' si vede chi scambia un errore del database per
// "non trovato".
func rifiutaLetture(t *testing.T, tabella, dove string) {
	t.Helper()
	err := service.DB.Callback().Query().Before("gorm:query").Register("rifiuta_letture_"+tabella, func(db *gorm.DB) {
		if db.Statement.Table != tabella {
			return
		}
		if c, ok := db.Statement.Clauses["WHERE"]; !ok || !strings.Contains(fmt.Sprint(c.Expression), dove) {
			return
		}
		_ = db.AddError(errors.New("lettura rifiutata dal test"))
	})
	if err != nil {
		t.Fatal(err)
	}
}

// righe conta con Raw, che rifiutaLetture non ferma, le righe di tabella.
func righe(t *testing.T, tabella string) int64 {
	t.Helper()
	var n int64
	if err := service.DB.Raw("SELECT count(*) FROM " + tabella).Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

// TestCancellazioneDispositivi prova sul router vero la cancellazione in
// blocco dei dispositivi dal pannello. Senza ostacoli se ne vanno il
// dispositivo e il token di sessione del suo uuid, con una connessione sola
// al database. Se gli uuid dei dispositivi non si leggono, o se i token dei
// loro uuid non si cancellano, restano dispositivi e token e la risposta e'
// OperationFailed, con l'errore nel log. Prima nel primo caso cancellava i
// dispositivi e rispondeva successo, nel secondo cancellava i dispositivi e
// rispondeva errore: in entrambi i token dei dispositivi cancellati restavano
// validi.
func TestCancellazioneDispositivi(t *testing.T) {
	for _, tc := range []struct {
		nome, rifiuto string
		ostacolo      func(t *testing.T)
	}{
		{"senza ostacoli", "", func(*testing.T) {}},
		{"uuid non letti", "lettura rifiutata dal test", func(t *testing.T) { rifiutaLetture(t, "peers", "") }},
		{"token non cancellati", "rifiutato dal test", func(t *testing.T) { rifiuta(t, "BEFORE DELETE ON user_tokens") }},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g, utente, registro := pannello(t, true)
			if err := service.DB.AutoMigrate(&model.Peer{}); err != nil {
				t.Fatal(err)
			}
			dispositivo := &model.Peer{Id: "999000111", Uuid: "dXVpZA==", UserId: utente.Id}
			if err := service.DB.Create(dispositivo).Error; err != nil {
				t.Fatal(err)
			}
			err := service.DB.Create(&model.UserToken{UserId: utente.Id, Token: "token-del-dispositivo", DeviceUuid: dispositivo.Uuid,
				ExpiredAt: time.Now().Add(time.Hour).Unix()}).Error
			if err != nil {
				t.Fatal(err)
			}
			tc.ostacolo(t)
			scadenza := unaConnessione(t)

			rotta := "/api/admin/peer/batchDelete"
			rec := alPannello(g, rotta, `{"row_ids": [`+strconv.FormatUint(uint64(dispositivo.RowId), 10)+`]}`)
			if scadenza.Err() != nil {
				t.Fatalf("POST %s: fermo per 5 secondi ad aspettare la connessione del database (stallo); risposta %d %s", rotta, rec.Code, rec.Body)
			}
			// Senza ostacoli resta solo il token del pannello; con un
			// ostacolo resta tutto.
			risposta, dispositivi, token := `{"code":0,"message":"success","data":null}`, int64(0), int64(1)
			if tc.rifiuto != "" {
				risposta, dispositivi, token = `{"code":101,"message":"Operazione non riuscita.","data":null}`, 1, 2
				if nelLog := registro.String(); !strings.Contains(nelLog, "POST "+rotta+": ") || !strings.Contains(nelLog, tc.rifiuto) {
					t.Errorf("POST %s, nel log mancano rotta o errore:\n%s", rotta, nelLog)
				}
			}
			if got := rec.Body.String(); rec.Code != 200 || got != risposta {
				t.Errorf("POST %s: stato %d\n got  %s\n want %s", rotta, rec.Code, got, risposta)
			}
			if n := righe(t, "peers"); n != dispositivi {
				t.Errorf("dispositivi dopo la richiesta: %d, attesi %d", n, dispositivi)
			}
			if n := righe(t, "user_tokens"); n != token {
				t.Errorf("token dopo la richiesta: %d, attesi %d", n, token)
			}
		})
	}
}
