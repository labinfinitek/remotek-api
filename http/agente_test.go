package http

import (
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/lib/jwt"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// agentiDiProva prepara il router vero con l'amministratore del pannello
// (id 1), i tecnici mario (id 2, nickname "Mario Rossi") e luigi (id 3,
// senza nickname), l'agente di mario (id 4) e quello di luigi (id 5); ogni
// utente ha il token "tok-<username>" per il client, e un Jwt senza chiave
// fa cercare a RustAuth il token nel database.
func agentiDiProva(t *testing.T) *gin.Engine {
	t.Helper()
	g, _, _ := pannello(t, true)
	// Le tabelle che la cancellazione di un utente ripulisce.
	if err := service.DB.AutoMigrate(&model.AddressBook{}, &model.AddressBookCollection{}, &model.AddressBookCollectionRule{}, &model.Peer{}); err != nil {
		t.Fatal(err)
	}
	precJwt := global.Jwt
	global.Jwt = jwt.NewJwt("", time.Hour)
	t.Cleanup(func() { global.Jwt = precJwt })
	no := false
	for _, u := range []*model.User{
		{Username: "mario", Nickname: "Mario Rossi", GroupId: 1, IsAdmin: &no, Status: model.COMMON_STATUS_ENABLE},
		{Username: "luigi", GroupId: 1, IsAdmin: &no, Status: model.COMMON_STATUS_ENABLE},
		{Username: "agente-mario", GroupId: 1, IsAdmin: &no, Status: model.COMMON_STATUS_ENABLE, AgenteDi: 2},
		{Username: "agente-luigi", GroupId: 1, IsAdmin: &no, Status: model.COMMON_STATUS_ENABLE, AgenteDi: 3},
	} {
		crea(t, u)
		crea(t, &model.UserToken{UserId: u.Id, Token: "tok-" + u.Username, ExpiredAt: time.Now().Add(time.Hour).Unix()})
	}
	return g
}

// agenteDi restituisce agente_di di ogni utente, per id.
func agenteDi(t *testing.T) map[uint]uint {
	t.Helper()
	var uu []model.User
	if err := service.DB.Raw("SELECT id, agente_di FROM users ORDER BY id").Scan(&uu).Error; err != nil {
		t.Fatal(err)
	}
	res := map[uint]uint{}
	for _, u := range uu {
		res[u.Id] = u.AgenteDi
	}
	return res
}

// TestAgentePannello prova sul router vero le regole degli agenti AI nella
// creazione e nella modifica di un utente dal pannello: il tecnico e' un
// altro utente che esiste ed e' una persona, un agente non e' mai
// amministratore, chi risponde di agenti non diventa un agente, e una
// modifica senza agente_di (il pannello non lo conosce) lo lascia com'e'.
// Un rifiuto non cambia niente e dice perche'.
func TestAgentePannello(t *testing.T) {
	iniziali := map[uint]uint{1: 0, 2: 0, 3: 0, 4: 2, 5: 3}
	for _, tc := range []struct {
		nome, rotta, corpo, messaggio string
		dopo                          map[uint]uint // agente_di dopo, se la richiesta riesce
	}{
		{"crea, tecnico che non c'e'", "create", `{"username":"nuovo","group_id":1,"status":1,"agente_di":99}`, "Il tecnico di un agente AI deve essere", nil},
		{"crea, tecnico agente", "create", `{"username":"nuovo","group_id":1,"status":1,"agente_di":4}`, "Il tecnico di un agente AI deve essere", nil},
		{"crea, agente amministratore", "create", `{"username":"nuovo","group_id":1,"status":1,"is_admin":true,"agente_di":2}`, "non può essere amministratore", nil},
		{"crea, agente", "create", `{"username":"nuovo","group_id":1,"status":1,"is_admin":false,"agente_di":2}`, "", map[uint]uint{6: 2}},
		{"crea, persona", "create", `{"username":"nuovo","group_id":1,"status":1}`, "", map[uint]uint{6: 0}},
		{"modifica, tecnico di se stesso", "update", `{"id":4,"username":"agente-mario","group_id":1,"status":1,"agente_di":4}`, "Il tecnico di un agente AI deve essere", nil},
		{"modifica, tecnico con agenti diventa agente", "update", `{"id":2,"username":"mario","group_id":1,"status":1,"agente_di":3}`, "non può diventare un agente", nil},
		{"modifica, agente amministratore senza agente_di", "update", `{"id":4,"username":"agente-mario","group_id":1,"status":1,"is_admin":true}`, "non può essere amministratore", nil},
		{"modifica, amministratore diventa agente senza is_admin", "update", `{"id":1,"username":"prova","group_id":1,"status":1,"agente_di":2}`, "non può essere amministratore", nil},
		{"modifica senza agente_di", "update", `{"id":4,"username":"agente-mario","nickname":"Agente","group_id":1,"status":1}`, "", map[uint]uint{}},
		{"modifica, agente di un altro tecnico", "update", `{"id":4,"username":"agente-mario","group_id":1,"status":1,"agente_di":3}`, "", map[uint]uint{4: 3}},
		{"modifica, agente torna persona", "update", `{"id":4,"username":"agente-mario","group_id":1,"status":1,"agente_di":0}`, "", map[uint]uint{4: 0}},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g := agentiDiProva(t)
			rec := alPannello(g, "/api/admin/user/"+tc.rotta, tc.corpo)
			body := rec.Body.String()
			want := maps.Clone(iniziali)
			if tc.dopo == nil {
				if !strings.Contains(body, `"code":101`) || !strings.Contains(body, tc.messaggio) {
					t.Errorf("POST %s: %s, attesi code 101 e %q", tc.rotta, body, tc.messaggio)
				}
			} else {
				if !strings.Contains(body, `"code":0`) {
					t.Errorf("POST %s: %s, atteso successo", tc.rotta, body)
				}
				maps.Copy(want, tc.dopo)
			}
			if got := agenteDi(t); !maps.Equal(got, want) {
				t.Errorf("agente_di dopo: %v, attesi %v", got, want)
			}
		})
	}
}

// TestAgenteClient prova sul router vero GET /api/agente: dice all'agente
// il nome del suo tecnico (il nickname, o lo username se e' vuoto), a una
// persona che non e' un agente, e senza token risponde 401 come le altre
// rotte con login.
func TestAgenteClient(t *testing.T) {
	g := agentiDiProva(t)
	for _, tc := range []struct {
		token, want string
		codice      int
	}{
		{"tok-agente-mario", `{"agente":true,"tecnico":"Mario Rossi"}`, 200},
		{"tok-agente-luigi", `{"agente":true,"tecnico":"luigi"}`, 200},
		{"tok-mario", `{"agente":false,"tecnico":""}`, 200},
		{"", "", 401},
	} {
		rec := daTecnico(g, "GET", "/api/agente", tc.token, "")
		if rec.Code != tc.codice || (tc.want != "" && rec.Body.String() != tc.want) {
			t.Errorf("GET /api/agente col token %q: %d %s, attesi %d %s", tc.token, rec.Code, rec.Body, tc.codice, tc.want)
		}
	}
}

// stati restituisce lo stato di ogni utente, per id.
func stati(t *testing.T) map[uint]model.StatusCode {
	t.Helper()
	var uu []model.User
	if err := service.DB.Raw("SELECT id, status FROM users ORDER BY id").Scan(&uu).Error; err != nil {
		t.Fatal(err)
	}
	res := map[uint]model.StatusCode{}
	for _, u := range uu {
		res[u.Id] = u.Status
	}
	return res
}

// TestAgenteSegueTecnico prova sul router vero che un agente AI segue il
// suo tecnico: disattivare il tecnico dal pannello disattiva i suoi agenti
// e non quelli degli altri; un tecnico con agenti non si cancella finche'
// ha agenti; un agente del tecnico disattivato non si riattiva, ma si
// modifica restando disattivato.
func TestAgenteSegueTecnico(t *testing.T) {
	const attivo, spento = model.COMMON_STATUS_ENABLE, model.COMMON_STATUS_DISABLED
	g := agentiDiProva(t)

	if rec := alPannello(g, "/api/admin/user/update", `{"id":2,"username":"mario","group_id":1,"status":2}`); !strings.Contains(rec.Body.String(), `"code":0`) {
		t.Fatalf("disattivazione di mario: %s", rec.Body)
	}
	if want, got := map[uint]model.StatusCode{1: attivo, 2: spento, 3: attivo, 4: spento, 5: attivo}, stati(t); !maps.Equal(got, want) {
		t.Errorf("stati dopo la disattivazione di mario: %v, attesi %v", got, want)
	}

	rec := alPannello(g, "/api/admin/user/delete", `{"id":3}`)
	if body := rec.Body.String(); !strings.Contains(body, `"code":101`) || !strings.Contains(body, "cancellali o assegnali") {
		t.Errorf("cancellazione di luigi, che ha un agente: %s", body)
	}
	if n := righe(t, "users"); n != 5 {
		t.Errorf("utenti dopo la cancellazione rifiutata: %d, attesi 5", n)
	}
	for _, corpo := range []string{`{"id":5}`, `{"id":3}`} {
		if rec := alPannello(g, "/api/admin/user/delete", corpo); !strings.Contains(rec.Body.String(), `"code":0`) {
			t.Errorf("cancellazione %s: %s", corpo, rec.Body)
		}
	}

	rec = alPannello(g, "/api/admin/user/update", `{"id":4,"username":"agente-mario","group_id":1,"status":1}`)
	if body := rec.Body.String(); !strings.Contains(body, `"code":101`) || !strings.Contains(body, "è disattivato") {
		t.Errorf("riattivazione dell'agente di mario, disattivato: %s", body)
	}
	if rec := alPannello(g, "/api/admin/user/update", `{"id":4,"username":"agente-mario","nickname":"Agente","group_id":1,"status":2}`); !strings.Contains(rec.Body.String(), `"code":0`) {
		t.Errorf("modifica dell'agente di mario, restando disattivato: %s", rec.Body)
	}
	if want, got := map[uint]model.StatusCode{1: attivo, 2: spento, 4: spento}, stati(t); !maps.Equal(got, want) {
		t.Errorf("stati alla fine: %v, attesi %v", got, want)
	}
}
