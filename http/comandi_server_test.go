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

// TestPannelloModificaComandoServer prova sul router vero la modifica dei
// comandi salvati dal pannello, POST /api/admin/rustdesk/cmdUpdate: con
// l'amministratore cambia la riga, a un utente non amministratore risponde
// come le altre rotte del gruppo e la riga resta com'era. Prima la rotta non
// c'era, rispondeva 404 e la modifica dal pannello non riusciva.
func TestPannelloModificaComandoServer(t *testing.T) {
	for _, admin := range []bool{false, true} {
		t.Run("admin="+strconv.FormatBool(admin), func(t *testing.T) {
			g, _, _ := pannello(t, admin)
			if err := service.DB.AutoMigrate(&model.ServerCmd{}); err != nil {
				t.Fatal(err)
			}
			salvato := &model.ServerCmd{Cmd: "salvato", Explain: "prima", Target: model.ServerCmdTargetIdServer}
			crea(t, salvato)

			const rotta = "/api/admin/rustdesk/cmdUpdate"
			corpo := `{"id": ` + strconv.FormatUint(uint64(salvato.Id), 10) + `, "cmd": "cambiato", "explain": "dopo", "target": "` + model.ServerCmdTargetRelayServer + `"}`
			rec := richiesta(g, "POST", rotta, "", corpo)
			risposta, voluto := `{"code":0,"message":"success","data":null}`, "cambiato/dopo/"+model.ServerCmdTargetRelayServer
			if !admin {
				risposta, voluto = `{"code":403,"message":"Non hai i permessi per questa operazione.","data":null}`, "salvato/prima/"+model.ServerCmdTargetIdServer
			}
			if got := rec.Body.String(); rec.Code != 200 || got != risposta {
				t.Errorf("POST %s: stato %d\n got  %s\n want %s", rotta, rec.Code, got, risposta)
			}
			dopo := &model.ServerCmd{}
			if err := service.DB.First(dopo, salvato.Id).Error; err != nil {
				t.Fatal(err)
			}
			if got := dopo.Cmd + "/" + dopo.Explain + "/" + dopo.Target; got != voluto {
				t.Errorf("voce dopo POST %s: %q, atteso %q", rotta, got, voluto)
			}
		})
	}
}

// TestPannelloCreaComandoIgnoraId prova sul router vero che
// POST /api/admin/rustdesk/cmdCreate crea sempre una voce nuova con l'id
// scelto dal database, anche se il corpo porta un id: quello di una voce
// esistente, che resta com'era, o uno libero. Prima l'id del corpo arrivava
// a DB.Create: con quello di una voce esistente il create falliva sul
// vincolo della chiave, con uno libero la riga nasceva con l'id scelto. Il
// pannello lo mandava con "Aggiungi" dopo "Modifica".
func TestPannelloCreaComandoIgnoraId(t *testing.T) {
	g, _, _ := pannello(t, true)
	if err := service.DB.AutoMigrate(&model.ServerCmd{}); err != nil {
		t.Fatal(err)
	}
	salvato := &model.ServerCmd{Cmd: "salvato", Explain: "prima", Target: model.ServerCmdTargetIdServer}
	crea(t, salvato)
	libero := salvato.Id + 1000

	const rotta = "/api/admin/rustdesk/cmdCreate"
	for _, id := range []uint{salvato.Id, libero} {
		cmd := "nuovo" + strconv.FormatUint(uint64(id), 10)
		corpo := `{"id": ` + strconv.FormatUint(uint64(id), 10) + `, "cmd": "` + cmd + `", "target": "` + model.ServerCmdTargetRelayServer + `"}`
		rec := richiesta(g, "POST", rotta, "", corpo)
		if got, want := rec.Body.String(), `{"code":0,"message":"success","data":null}`; rec.Code != 200 || got != want {
			t.Errorf("POST %s con id %d: stato %d\n got  %s\n want %s", rotta, id, rec.Code, got, want)
			continue
		}
		nuovo := &model.ServerCmd{}
		if err := service.DB.Where("cmd = ?", cmd).First(nuovo).Error; err != nil {
			t.Fatalf("voce %q dopo POST %s: %v", cmd, rotta, err)
		}
		if nuovo.Id == salvato.Id || nuovo.Id == libero {
			t.Errorf("POST %s con id %d: voce nuova con id %d, scelto dal corpo", rotta, id, nuovo.Id)
		}
	}
	dopo := &model.ServerCmd{}
	if err := service.DB.First(dopo, salvato.Id).Error; err != nil {
		t.Fatal(err)
	}
	if got, want := dopo.Cmd+"/"+dopo.Explain+"/"+dopo.Target, "salvato/prima/"+model.ServerCmdTargetIdServer; got != want {
		t.Errorf("voce esistente dopo i create: %q, attesa %q", got, want)
	}
}

// TestPannelloComandoServerTargetNonValido prova sul router vero che
// POST /api/admin/rustdesk/cmdCreate e cmdUpdate rifiutano un target che non
// e' hbbs (21115) ne' hbbr (21117), anche vuoto, con la risposta di sendCmd
// per lo stesso target, e che la tabella resta com'era. Prima la voce si
// salvava lo stesso e poi non si poteva mandare: il pannello, dopo
// "Aggiungi", parte col target vuoto.
func TestPannelloComandoServerTargetNonValido(t *testing.T) {
	g, _, _ := pannello(t, true)
	if err := service.DB.AutoMigrate(&model.ServerCmd{}); err != nil {
		t.Fatal(err)
	}
	salvato := &model.ServerCmd{Cmd: "salvato", Explain: "prima", Target: model.ServerCmdTargetIdServer}
	crea(t, salvato)
	id := strconv.FormatUint(uint64(salvato.Id), 10)

	for _, target := range []string{"", "21116", "hbbs"} {
		invio := richiesta(g, "POST", "/api/admin/rustdesk/sendCmd", "", `{"cmd": "h", "target": "`+target+`"}`)
		if !strings.HasPrefix(invio.Body.String(), `{"code":101,`) {
			t.Fatalf("POST /api/admin/rustdesk/sendCmd col target %q: %s, atteso un rifiuto", target, invio.Body)
		}
		for _, tc := range []struct{ rotta, corpo string }{
			{"/api/admin/rustdesk/cmdCreate", `{"cmd": "nuovo", "target": "` + target + `"}`},
			{"/api/admin/rustdesk/cmdUpdate", `{"id": ` + id + `, "cmd": "cambiato", "explain": "dopo", "target": "` + target + `"}`},
		} {
			rec := richiesta(g, "POST", tc.rotta, "", tc.corpo)
			if rec.Code != invio.Code || rec.Body.String() != invio.Body.String() {
				t.Errorf("POST %s col target %q: stato %d\n got  %s\n want %d %s, come sendCmd", tc.rotta, target, rec.Code, rec.Body, invio.Code, invio.Body)
			}
		}
	}

	var voci []string
	var salvati []*model.ServerCmd
	if err := service.DB.Order("id").Find(&salvati).Error; err != nil {
		t.Fatal(err)
	}
	for _, v := range salvati {
		voci = append(voci, v.Cmd+"/"+v.Explain+"/"+v.Target)
	}
	if want := []string{"salvato/prima/" + model.ServerCmdTargetIdServer}; !slices.Equal(voci, want) {
		t.Errorf("comandi salvati dopo le richieste rifiutate: %q, attesi %q", voci, want)
	}
}
