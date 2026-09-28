package http

import (
	"net"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// fintoHbbr ascolta su 127.0.0.1 al posto di hbbr, che global.Config.Admin
// fa puntare qui: a ogni connessione legge cio' che arriva, lo mette in
// ricevuti e risponde risposta. Restituisce il canale dei comandi, che si
// chiude quando il test finisce e il listener si ferma.
func fintoHbbr(t *testing.T, risposta string) <-chan string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	global.Config.Admin.RelayServerPort = l.Addr().(*net.TCPAddr).Port
	ricevuti := make(chan string, 10)
	go func() {
		defer close(ricevuti)
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			buf := make([]byte, 1024)
			n, _ := conn.Read(buf)
			ricevuti <- string(buf[:n])
			_, _ = conn.Write([]byte(risposta))
			_ = conn.Close()
		}
	}()
	t.Cleanup(func() { _ = l.Close() })
	return ricevuti
}

// TestPannelloComandiServerSoloAdmin prova sul router vero che le rotte
// /api/admin/rustdesk/* (comandi a hbbs e hbbr, e l'elenco dei comandi
// salvati) sono solo per gli amministratori (REM-2026-004). A un utente del
// pannello non amministratore rispondono come AdminPrivilege risponde agli
// altri gruppi, qui /api/admin/group/list, e non cambiano niente: sendCmd
// non apre connessioni verso hbbr, cmdCreate e cmdDelete lasciano
// server_cmds com'era. Con l'amministratore funzionano. Prima il gruppo
// stava dietro il solo BackendUserAuth, e qualunque utente del pannello
// mandava comandi al server e creava o cancellava quelli salvati.
func TestPannelloComandiServerSoloAdmin(t *testing.T) {
	const blacklist = "blacklist-add 10.0.0.1"
	for _, admin := range []bool{false, true} {
		t.Run("admin="+strconv.FormatBool(admin), func(t *testing.T) {
			g, _, _ := pannello(t, admin)
			if err := service.DB.AutoMigrate(&model.ServerCmd{}); err != nil {
				t.Fatal(err)
			}
			salvato := &model.ServerCmd{Cmd: "salvato", Explain: "prima", Target: model.ServerCmdTargetIdServer}
			crea(t, salvato)
			ricevuti := fintoHbbr(t, "risposta di hbbr")
			negato := richiesta(g, "GET", "/api/admin/group/list", "", "")
			if !admin && (negato.Code != 200 || !strings.HasPrefix(negato.Body.String(), `{"code":403,`)) {
				t.Fatalf("GET /api/admin/group/list senza essere amministratore: stato %d, corpo %s", negato.Code, negato.Body.String())
			}

			id := strconv.FormatUint(uint64(salvato.Id), 10)
			for _, tc := range []struct {
				metodo, rotta, corpo string
				risposta             string // con l'amministratore; "" se basta code 0
			}{
				{"POST", "/api/admin/rustdesk/sendCmd", `{"cmd": "blacklist-add", "option": "10.0.0.1", "target": "` + model.ServerCmdTargetRelayServer + `"}`,
					`{"code":0,"message":"success","data":"risposta di hbbr"}`},
				{"GET", "/api/admin/rustdesk/cmdList", "", ""},
				{"POST", "/api/admin/rustdesk/cmdCreate", `{"cmd": "nuovo", "target": "` + model.ServerCmdTargetIdServer + `"}`,
					`{"code":0,"message":"success","data":null}`},
				{"POST", "/api/admin/rustdesk/cmdDelete", `{"id": ` + id + `}`,
					`{"code":0,"message":"success","data":null}`},
			} {
				rec := richiesta(g, tc.metodo, tc.rotta, "", tc.corpo)
				switch {
				case !admin:
					if rec.Code != negato.Code || rec.Body.String() != negato.Body.String() {
						t.Errorf("%s %s senza essere amministratore: stato %d, corpo %s; attesi stato %d, corpo %s",
							tc.metodo, tc.rotta, rec.Code, rec.Body.String(), negato.Code, negato.Body.String())
					}
				case tc.risposta == "":
					if rec.Code != 200 || !strings.HasPrefix(rec.Body.String(), `{"code":0,`) {
						t.Errorf("%s %s: stato %d, corpo %s", tc.metodo, tc.rotta, rec.Code, rec.Body.String())
					}
				default:
					if got := rec.Body.String(); rec.Code != 200 || got != tc.risposta {
						t.Errorf("%s %s: stato %d\n got  %s\n want %s", tc.metodo, tc.rotta, rec.Code, got, tc.risposta)
					}
				}
			}

			var salvati []string
			for _, c := range []string{"salvato", "nuovo"} {
				var n int64
				if err := service.DB.Raw("SELECT count(*) FROM server_cmds WHERE cmd = ?", c).Scan(&n).Error; err != nil {
					t.Fatal(err)
				}
				if n > 0 {
					salvati = append(salvati, c)
				}
			}
			var inviati []string
		raccogli:
			for {
				select {
				case c := <-ricevuti:
					inviati = append(inviati, c)
				default:
					break raccogli
				}
			}
			voluti, comandi := []string{"salvato"}, []string(nil)
			if admin {
				voluti, comandi = []string{"nuovo"}, []string{blacklist}
			}
			if !slices.Equal(salvati, voluti) {
				t.Errorf("comandi salvati dopo le richieste: %q, attesi %q", salvati, voluti)
			}
			if !slices.Equal(inviati, comandi) {
				t.Errorf("comandi arrivati a hbbr: %q, attesi %q", inviati, comandi)
			}
		})
	}
}
