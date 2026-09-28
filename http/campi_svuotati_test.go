package http

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// modifica manda corpo a rotta e controlla che il pannello risponda
// successo.
func modifica(t *testing.T, g *gin.Engine, rotta, corpo string) {
	t.Helper()
	rec := richiesta(g, "POST", rotta, "", corpo)
	if got, want := rec.Body.String(), `{"code":0,"message":"success","data":null}`; rec.Code != 200 || got != want {
		t.Fatalf("POST %s: stato %d\n got  %s\n want %s", rotta, rec.Code, got, want)
	}
}

// TestPannelloSvuotaDispositivo prova sul router vero che la modifica di un
// dispositivo dal pannello (POST /api/admin/peer/update) salva vuoti alias,
// nome del PC e gruppo tolti, e lascia com'erano uuid (mandato vuoto),
// utente, ultimo contatto e IP, che il modulo non manda. Prima gorm saltava i
// campi vuoti: il pannello rispondeva successo e l'alias restava.
func TestPannelloSvuotaDispositivo(t *testing.T) {
	g, _, _ := pannello(t, true)
	if err := service.DB.AutoMigrate(&model.Peer{}); err != nil {
		t.Fatal(err)
	}
	pc := &model.Peer{Id: "999000111", Uuid: "dXVpZA==", Alias: "ufficio", Hostname: "PC-COLLAUDO", Os: "windows",
		GroupId: 3, UserId: 7, LastOnlineTime: 1700000000, LastOnlineIp: "10.0.0.1"}
	crea(t, pc)

	const rotta = "/api/admin/peer/update"
	corpo := `{"row_id": ` + strconv.FormatUint(uint64(pc.RowId), 10) +
		`, "id": "999000111", "uuid": "", "alias": "", "hostname": "", "os": "windows", "group_id": 0}`
	modifica(t, g, rotta, corpo)
	dopo := &model.Peer{}
	if err := service.DB.First(dopo, pc.RowId).Error; err != nil {
		t.Fatal(err)
	}
	got := fmt.Sprintf("id=%s uuid=%s alias=%q hostname=%q os=%s gruppo=%d utente=%d contatto=%d ip=%s",
		dopo.Id, dopo.Uuid, dopo.Alias, dopo.Hostname, dopo.Os, dopo.GroupId, dopo.UserId, dopo.LastOnlineTime, dopo.LastOnlineIp)
	want := `id=999000111 uuid=dXVpZA== alias="" hostname="" os=windows gruppo=0 utente=7 contatto=1700000000 ip=10.0.0.1`
	if got != want {
		t.Errorf("dispositivo dopo POST %s:\n got  %s\n want %s", rotta, got, want)
	}
}

// TestPannelloSvuotaComandoServer prova sul router vero che la modifica di
// una voce dei comandi del server dal pannello (POST
// /api/admin/rustdesk/cmdUpdate) salva vuoti alias, opzione e spiegazione
// tolti, e lascia la data di creazione. Prima restavano quelli di prima e il pannello rispondeva
// successo.
func TestPannelloSvuotaComandoServer(t *testing.T) {
	g, _, _ := pannello(t, true)
	if err := service.DB.AutoMigrate(&model.ServerCmd{}); err != nil {
		t.Fatal(err)
	}
	salvato := &model.ServerCmd{Cmd: "blacklist-add", Alias: "ba", Option: "<ip>", Explain: "prima", Target: model.ServerCmdTargetRelayServer}
	crea(t, salvato)
	prima := &model.ServerCmd{}
	if err := service.DB.First(prima, salvato.Id).Error; err != nil {
		t.Fatal(err)
	}

	const rotta = "/api/admin/rustdesk/cmdUpdate"
	corpo := `{"id": ` + strconv.FormatUint(uint64(salvato.Id), 10) +
		`, "cmd": "blacklist-add", "alias": "", "option": "", "explain": "", "target": "` + model.ServerCmdTargetRelayServer + `"}`
	modifica(t, g, rotta, corpo)
	dopo := &model.ServerCmd{}
	if err := service.DB.First(dopo, salvato.Id).Error; err != nil {
		t.Fatal(err)
	}
	got := fmt.Sprintf("cmd=%s alias=%q option=%q explain=%q target=%s creato=%v",
		dopo.Cmd, dopo.Alias, dopo.Option, dopo.Explain, dopo.Target, dopo.CreatedAt)
	want := fmt.Sprintf(`cmd=blacklist-add alias="" option="" explain="" target=%s creato=%v`, model.ServerCmdTargetRelayServer, prima.CreatedAt)
	if got != want {
		t.Errorf("voce dei comandi dopo POST %s:\n got  %s\n want %s", rotta, got, want)
	}
}

// TestPannelloSvuotaProviderOauth prova sul router vero che la modifica di
// un provider OAuth dal pannello (POST /api/admin/oauth/update) salva vuoti
// issuer e scope tolti, e lascia la data di creazione. Prima restavano
// quelli di prima e il pannello rispondeva successo.
func TestPannelloSvuotaProviderOauth(t *testing.T) {
	g, _, _ := pannello(t, true)
	provider := &model.Oauth{Op: "aziendale", OauthType: model.OauthTypeOidc, ClientId: "id", ClientSecret: "segreto",
		Issuer: "https://idp.esempio.invalid", Scopes: "openid,email"}
	crea(t, provider)
	prima := &model.Oauth{}
	if err := service.DB.First(prima, provider.Id).Error; err != nil {
		t.Fatal(err)
	}

	const rotta = "/api/admin/oauth/update"
	corpo := `{"id": ` + strconv.FormatUint(uint64(provider.Id), 10) +
		`, "op": "aziendale", "oauth_type": "oidc", "client_id": "id", "client_secret": "segreto", "issuer": "", "scopes": ""}`
	modifica(t, g, rotta, corpo)
	dopo := &model.Oauth{}
	if err := service.DB.First(dopo, provider.Id).Error; err != nil {
		t.Fatal(err)
	}
	got := fmt.Sprintf("op=%s id=%s segreto=%s issuer=%q scope=%q creato=%v",
		dopo.Op, dopo.ClientId, dopo.ClientSecret, dopo.Issuer, dopo.Scopes, dopo.CreatedAt)
	want := fmt.Sprintf(`op=aziendale id=id segreto=segreto issuer="" scope="" creato=%v`, prima.CreatedAt)
	if got != want {
		t.Errorf("provider dopo POST %s:\n got  %s\n want %s", rotta, got, want)
	}
}

// TestPannelloSvuotaUtente prova sul router vero che la modifica di un
// utente dal pannello (POST /api/admin/user/update) salva vuoti email,
// nickname, avatar e nota tolti, e lascia com'erano la password, che il
// modulo non manda, e il ruolo, se is_admin non c'e'. Prima restavano quelli
// di prima e il pannello rispondeva successo.
func TestPannelloSvuotaUtente(t *testing.T) {
	g, _, _ := pannello(t, true)
	admin := true
	altro := &model.User{Username: "altro", Email: "altro@esempio.invalid", Password: "hash-di-prima", Nickname: "Altro",
		Avatar: "https://esempio.invalid/a.png", GroupId: 1, IsAdmin: &admin, Status: model.COMMON_STATUS_ENABLE, Remark: "nota"}
	crea(t, altro)

	const rotta = "/api/admin/user/update"
	corpo := `{"id": ` + strconv.FormatUint(uint64(altro.Id), 10) +
		`, "username": "altro", "email": "", "nickname": "", "avatar": "", "group_id": 1, "status": 1, "remark": ""}`
	modifica(t, g, rotta, corpo)
	dopo := &model.User{}
	if err := service.DB.First(dopo, altro.Id).Error; err != nil {
		t.Fatal(err)
	}
	got := fmt.Sprintf("nome=%s email=%q nickname=%q avatar=%q nota=%q password=%s admin=%v stato=%d",
		dopo.Username, dopo.Email, dopo.Nickname, dopo.Avatar, dopo.Remark, dopo.Password, dopo.IsAdmin != nil && *dopo.IsAdmin, dopo.Status)
	want := `nome=altro email="" nickname="" avatar="" nota="" password=hash-di-prima admin=true stato=1`
	if got != want {
		t.Errorf("utente dopo POST %s:\n got  %s\n want %s", rotta, got, want)
	}
}
