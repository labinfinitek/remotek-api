# Remotek API

*Italiano. English: [README_EN.md](README_EN.md).*

## Cos'e'

L'API di **Remotek**, il servizio di assistenza remota di Infinitek: la parte
che tiene utenti, rubriche, dispositivi e registri, accanto ai server ID e
relay (`hbbs` e `hbbr` del [server ufficiale di RustDesk](https://github.com/rustdesk/rustdesk-server),
che non sono in questo repo). E' compatibile col client RustDesk 1.4.9 e col
client Remotek.

E' un fork di [rustdesk-api](https://github.com/lejianwen/rustdesk-api) v2.7
di lejianwen (MIT). Upstream e' fermo: questo fork e' il progetto. Cosa e'
cambiato rispetto a v2.7 sta in [REMOTEK.md](REMOTEK.md), le novita' in
[CHANGELOG.md](CHANGELOG.md).

## Cosa fa oggi

- **Rubrica** del client: rubrica personale e rubriche condivise tra utenti,
  con tag e regole di condivisione.
- **Gruppi** di utenti (normali e condivisi) e gruppi di dispositivi.
- **Dispositivi**: i client mandano le informazioni di sistema, il pannello
  le mostra.
- **Audit**: registro delle connessioni e dei trasferimenti di file mandati
  dai client.
- **Note di sessione**: il tecnico annota la connessione e la nota si salva
  nel registro delle connessioni. Si legge nel JSON di
  `/api/admin/audit_conn/list`, nella colonna "Remark" del registro delle
  connessioni del pannello e nella sua esportazione CSV.
  Durante la sessione la nota si scrive dal client del tecnico; quella di
  fine connessione chiede sul PC del tecnico l'opzione del client
  `allow-ask-for-note` (spenta di fabbrica) e il login. La nota si attacca
  solo a una connessione gia' registrata, al massimo 2000 caratteri (oltre
  si tronca), e il suo testo non va mai nel log.
- **Trascrizione del terminale**: il PC controllato manda all'API, a
  blocchi concatenati da hash, quello che passa nelle sessioni terminale;
  la leggono e la verificano solo gli amministratori (formato qui sotto).
- **Log di accesso**: ogni login, dal client e dal pannello.
- **Pannello di amministrazione** su `/_admin/`: [rustdesk-api-web](https://github.com/lejianwen/rustdesk-api-web)
  compilato nell'immagine a un commit fissato, con il marchio di Remotek e
  le modifiche delle patch in [`pannello/`](pannello/README.md).
  Utenti, dispositivi, rubriche, tag, gruppi, OAuth, registri.
- **Login**: con password; con un provider **OIDC** generico, configurato dal
  pannello; con **LDAP** (upstream lo dichiara provato con OpenLDAP e Active Directory;
  in Remotek non ancora),
  configurato da file o variabili. Google resta usabile come provider OIDC
  generico: nel pannello tipo `OIDC`, IdP (l'op) per esempio `google`, Issuer
  `https://accounts.google.com`. I tipi GitHub, Google e LinuxDo che il menu
  del pannello offre ancora non sono piu' supportati (decisione A3).
- **Lingue**: italiano (predefinito) e inglese.
- **Marchio configurabile**: nome, logo e favicon senza toccare il codice.

## Trascrizione delle sessioni terminale

Ogni sessione terminale (type 4 nel registro delle connessioni) si
trascrive dal PC controllato: una copia resta sul PC, una va all'API con
`POST /api/audit/terminal`, senza login, un blocco per richiesta. Questo e'
il formato che il client segue. Corpo JSON:

| Campo | Tipo | Valore |
|---|---|---|
| `id` | stringa | ID del PC controllato |
| `uuid` | stringa | uuid del PC, come in `/api/sysinfo` e `/api/audit/conn` |
| `conn_id` | int64 | lo stesso `conn_id` dell'audit `new` della sessione |
| `seq` | intero | 1 per il primo blocco, +1 a ogni blocco, al massimo 100000 |
| `dir` | stringa | `in` (arrivato da chi controlla) o `out` (uscito dalla shell) |
| `data` | stringa | base64 standard (con `=`) dei byte, al massimo 64 KiB decodificati; vuoto solo con `fine` |
| `hash` | stringa | esadecimale minuscolo di SHA-256( H(seq-1) ‖ d ‖ data ) |
| `fine` | bool | `true` sull'ultimo blocco della sessione |

H(0) sono 32 byte a zero, H(n) i 32 byte grezzi (non l'esadecimale)
dell'hash del blocco n; d e' il byte `i` (0x69) per `in` e `o` (0x6f) per
`out`; data sono i byte decodificati.

Ogni blocco si lega alla riga del registro delle connessioni della
sessione: la piu' recente del PC con quel `conn_id`. Il client fa
ripartire `conn_id` da un valore casuale a ogni avvio del servizio, quindi
dopo un riavvio lo stesso numero puo' tornare: la sessione nuova ha la sua
riga e la sua trascrizione, da `seq` 1, e quella vecchia resta com'era.

L'API controlla, nell'ordine: che `id` e `uuid` siano del PC registrato;
che quella riga ci sia e sia di type 4 (il blocco va mandato dopo l'audit
che autentica la connessione); `dir`; `data` in base64 ed entro 64 KiB;
`data` non vuoto, se `fine` non e' `true` (un blocco vuoto chiude la
sessione senza dati in coda); `seq` non oltre 100000, al massimo 100000
blocchi per sessione; `seq` uguale all'ultimo salvato per quella riga piu'
uno; `hash`; nessun blocco dopo quello con `fine`; al massimo 20 MiB di
dati per sessione. Un blocco che non passa non si salva e nel log resta un
warn con rotta, ID del PC e motivo, senza uuid ne' contenuto. La risposta
e' sempre quella delle altre rotte dell'audit,
`{"code":0,"message":"success","data":""}` (400 solo per un corpo che non
e' JSON): il client non la guarda, e un blocco scartato non si rimanda.

Dal pannello, solo per gli amministratori, con `audit_conn_id` nella query
(l'id della riga nel registro delle connessioni): `GET /api/admin/audit_conn/terminal/list` elenca i blocchi in
ordine di `seq`, con `data` in base64 (paginato con `page` e `page_size`,
10 per pagina se manca); `GET /api/admin/audit_conn/terminal/verify`
ricalcola la catena dal database e risponde `integra`, `blocchi`, `fine`,
`hash` (quello salvato dell'ultimo blocco, da confrontare con la copia sul
PC) e `primo_errato`, il primo `seq` che non torna (0 se integra).
Cancellare dal pannello una connessione terminale cancella anche la sua
trascrizione.

## Installazione

Ogni rilascio (tag `api-vX.Y.Z`) pubblica l'immagine costruita dal
`Dockerfile` come `ghcr.io/labinfinitek/remotek-api:X.Y.Z` (e `:X.Y`), con
attestazione di provenienza, SBOM e inventario delle licenze nella
[release](https://github.com/labinfinitek/remotek-api/releases). Si verifica
con `gh attestation verify oci://ghcr.io/labinfinitek/remotek-api:X.Y.Z --owner labinfinitek`.
Il `docker-compose.yaml` del repo invece costruisce l'immagine dal sorgente.

### Con docker compose

`docker-compose.yaml` costruisce l'immagine dal sorgente come
`ghcr.io/labinfinitek/remotek-api:dev` e la avvia sulla porta 21114.

1. Nel file sostituisci i segnaposti `<...>` delle quattro variabili
   `RUSTDESK_API_RUSTDESK_*`: indirizzi di `hbbs` e `hbbr`, indirizzo con cui
   i client raggiungono questa API, chiave pubblica di `hbbs` (il contenuto
   di `id_ed25519.pub`). Il fuso e' `TZ=Europe/Rome`.
2. Crea la cartella dati, che deve essere di `10001:10001`, l'utente con cui
   gira l'API; altrimenti l'API non parte e dice perche':

   ```bash
   mkdir -p remotek-data && sudo chown 10001:10001 remotek-data
   docker compose up -d --build
   ```

3. Il pannello e' su `http://<host>:21114/_admin/`: vedi [Primo avvio](#primo-avvio-e-amministrazione).

In `remotek-data/` (`/app/data` nel container) stanno il database SQLite
`rustdeskapi.db` e `admin-password.txt`; mentre l'API gira, accanto al
database ci sono anche `rustdeskapi.db-wal` e `rustdeskapi.db-shm` (SQLite in
WAL), che allo stop pulito (SIGTERM, `docker compose stop`) rientrano nel
database e spariscono. Il log va solo su stdout (`docker compose logs`), in
JSON, una riga per evento:

```json
{"time":"2026-09-29T13:33:09.69Z","level":"WARN","source":{"function":"...","file":"...","line":64},"msg":"rustdesk.id-server vuoto: ..."}
```

Ogni risposta ha l'header `X-Request-Id`, un id casuale che l'API sceglie
per la richiesta (un `X-Request-Id` mandato dal chiamante non si usa). Lo
hanno come `request_id` le righe che cominciano con metodo e rotta (per
esempio `POST /api/heartbeat: ...`: errori mandati al client, avvisi dei
gestori, panic) e la riga `richiesta`; le altre righe scritte durante la
richiesta no, per esempio i `Login Fail`, quelle dei servizi e le query SQL
lente o in errore di gorm. A livello
`debug` c'e' anche una riga `richiesta` per ogni richiesta, con metodo, rotta,
stato e durata, senza IP ne' query.

Backup: ad API accesa con `sqlite3 rustdeskapi.db "VACUUM INTO '<file>'"` (o
`.backup` di `sqlite3`), mai copiando il solo `rustdeskapi.db`, a cui
mancherebbero le scritture ancora nel `-wal`; ad API ferma, dopo uno stop
pulito basta `rustdeskapi.db`, altrimenti si copia tutta la cartella. Per
ripristinare, ad API ferma, il backup va al posto di `rustdeskapi.db` e
`-wal` e `-shm`, se ci sono, si cancellano.

### Costruire l'immagine

```bash
docker build \
  --build-arg VERSION=<versione> \
  --build-arg REVISION="$(git rev-parse HEAD)" \
  -t remotek-api .
```

Quattro stadi: binario Go statico (CGO per SQLite), sorgente di
rustdesk-api-web al commit fissato col marchio e le patch di `pannello/`
applicate in ordine di nome (una patch che non si applica ferma la build),
build del pannello, immagine finale Alpine. Le immagini di base sono fissate
per digest. Argomenti:

| Argomento | Default | Uso |
|---|---|---|
| `VERSION` | `dev` | scritto in `resources/version`, lo restituisce `/api/version` |
| `REVISION` | `unknown` | label OCI `org.opencontainers.image.revision` |
| `PANNELLO_COMMIT` | `3998c2a9213fcd047252776d0f0db33e6717026c` | sha completo del commit di rustdesk-api-web |
| `BRAND_NAME` | `Remotek` | titolo statico del pannello; lettere, cifre, spazi e `. _ -` |

L'`HEALTHCHECK` chiama `http://127.0.0.1:21114/api/version`: se cambi
`gin.api-addr`, sovrascrivilo con `--health-cmd`.

### Sviluppo

Serve Go 1.26 (`go.mod`) e un compilatore C, perche' il driver SQLite usa CGO.
Dalla radice del repo:

```bash
go build -o apimain ./cmd   # senza -o fallisce: cmd e' gia' una cartella
./apimain                   # legge ./conf/config.yaml, oppure -c <file>
go test ./...
```

L'API cerca `conf/` e `resources/` nella cartella corrente: da un'altra
cartella usa `-c <percorso>/conf/config.yaml` e
`RUSTDESK_API_GIN_RESOURCES_PATH=<percorso>/resources`. Il pannello non e'
nel repo (lo compila il `Dockerfile` in `resources/admin/`): senza, `/_admin/`
non ha pagine. La documentazione swagger si rigenera con
`go generate -tags tools ./tools` (serve nel `PATH` `swag` v1.16.3, la
versione di `go.mod`).

## Configurazione

La configurazione sta in `conf/config.yaml` (nell'immagine
`/app/conf/config.yaml`) e ogni chiave si puo' sovrascrivere con una variabile
d'ambiente: prefisso `RUSTDESK_API_`, poi la chiave in maiuscolo con `.` e `-`
sostituiti da `_` (`app.captcha-threshold` diventa
`RUSTDESK_API_APP_CAPTCHA_THRESHOLD`). La variabile batte il file, il file
batte il default del codice. Una variabile vale solo per le chiavi presenti
nel file o con un default nel codice (quelle segnate con \*): il
`conf/config.yaml` del repo e dell'immagine le ha tutte, un file proprio piu'
corto no.

Il default e' il valore di `conf/config.yaml`; \* vuol dire che il codice ha lo
stesso default se la chiave manca dal file.

| Chiave | Variabile | Default | Valori e note |
|---|---|---|---|
| `lang` \* | `RUSTDESK_API_LANG` | `it` | `it`, `en`, vuoto (= inglese); un altro valore ferma l'avvio. Lingua delle risposte quando `Accept-Language` non ne sceglie una (il client RustDesk non lo manda) |
| `brand.name` \* | `RUSTDESK_API_BRAND_NAME` | `Remotek` | nome del prodotto; vuoto = `Remotek` |
| `brand.dir` \* | `RUSTDESK_API_BRAND_DIR` | `./resources/brand` | cartella di `logo.svg` e `favicon.svg`, serviti su `/brand/` |
| `app.register` \* | `RUSTDESK_API_APP_REGISTER` | `false` | registrazione libera di nuovi utenti |
| `app.register-status` | `RUSTDESK_API_APP_REGISTER_STATUS` | `1` | stato di chi si registra: `1` attivo, `2` disattivato |
| `app.captcha-threshold` \* | `RUSTDESK_API_APP_CAPTCHA_THRESHOLD` | `3` | login sbagliati da un IP, in 10 minuti, dopo cui serve il captcha; `0` sempre, negativo mai |
| `app.ban-threshold` \* | `RUSTDESK_API_APP_BAN_THRESHOLD` | `10` | login sbagliati da un IP, in 10 minuti, dopo cui ogni sua richiesta e' rifiutata per 30 minuti; `0` mai |
| `app.show-swagger` \* | `RUSTDESK_API_APP_SHOW_SWAGGER` | `0` | `1` pubblica `/swagger/index.html` e `/admin/swagger/index.html` |
| `app.token-expire` | `RUSTDESK_API_APP_TOKEN_EXPIRE` | `168h` | durata di una sessione (durata Go: `72h`, `30m`) |
| `app.web-sso` \* | `RUSTDESK_API_APP_WEB_SSO` | `false` | accende il login del client confermato dal pannello (`webauth`); spento, `/api/oidc/auth` lo rifiuta come un provider che non esiste |
| `app.disable-pwd-login` | `RUSTDESK_API_APP_DISABLE_PWD_LOGIN` | `false` | `true` toglie il login con utente e password, anche quello LDAP, che passa di li'; resta OIDC |
| `admin.title` \* | `RUSTDESK_API_ADMIN_TITLE` | vuoto | titolo del pannello; vuoto = `brand.name` |
| `admin.hello` | `RUSTDESK_API_ADMIN_HELLO` | vuoto | messaggio di benvenuto del pannello (HTML); se non e' vuoto, `admin.hello-file` non si legge |
| `admin.hello-file` | `RUSTDESK_API_ADMIN_HELLO_FILE` | `./conf/admin/hello.html` | file del benvenuto; `{{username}}` e `{{brand}}` si sostituiscono |
| `admin.id-server-port` | `RUSTDESK_API_ADMIN_ID_SERVER_PORT` | `21116` | porta di `hbbs` per i comandi del pannello, mandati a `127.0.0.1` sulla porta meno uno |
| `admin.relay-server-port` | `RUSTDESK_API_ADMIN_RELAY_SERVER_PORT` | `21117` | porta di `hbbr` per i comandi del pannello, su `127.0.0.1` |
| `gin.api-addr` | `RUSTDESK_API_GIN_API_ADDR` | `0.0.0.0:21114` | indirizzo di ascolto |
| `gin.mode` | `RUSTDESK_API_GIN_MODE` | `release` | `release`, `debug`, `test` |
| `gin.resources-path` | `RUSTDESK_API_GIN_RESOURCES_PATH` | `resources` | cartella di pannello, lingue e modelli; senza i file di lingua l'avvio si ferma |
| `gin.trust-proxy` \* | `RUSTDESK_API_GIN_TRUST_PROXY` | vuoto | IP o CIDR dei proxy fidati, separati da virgola; vuoto = nessuno, `X-Forwarded-For` e `X-Real-IP` ignorati. Dietro un reverse proxy va impostato, altrimenti captcha e ban contano ogni client come l'IP del proxy. Un valore non valido ferma l'avvio |
| `gorm.type` | `RUSTDESK_API_GORM_TYPE` | `sqlite` | solo `sqlite` (vuoto vale uguale); un altro valore ferma l'avvio. Il database e' `data/rustdeskapi.db` |
| `rustdesk.id-server` | `RUSTDESK_API_RUSTDESK_ID_SERVER` | vuoto | `host:21116` di `hbbs`, da impostare; lo mostra il pannello. Vuoto: all'avvio un warn; un client RustDesk senza server ID usa i server pubblici di RustDesk |
| `rustdesk.relay-server` | `RUSTDESK_API_RUSTDESK_RELAY_SERVER` | vuoto | `host:21117` di `hbbr`, da impostare; lo mostra il pannello |
| `rustdesk.api-server` | `RUSTDESK_API_RUSTDESK_API_SERVER` | vuoto | indirizzo di questa API come lo vedono client e browser, da impostare; lo mostra il pannello e da' la callback OIDC `<api-server>/api/oidc/callback`. Vuoto: all'avvio un warn, e il login OIDC e `webauth` mandano a indirizzi senza host |
| `rustdesk.key` | `RUSTDESK_API_RUSTDESK_KEY` | vuoto | chiave pubblica di `hbbs`; vuota = si legge `rustdesk.key-file` |
| `rustdesk.key-file` | `RUSTDESK_API_RUSTDESK_KEY_FILE` | `/data/id_ed25519.pub` | file della chiave; se non si legge, la chiave resta vuota senza errore |
| `rustdesk.personal` | `RUSTDESK_API_RUSTDESK_PERSONAL` | `1` | `1` rubrica personale attiva, `0` spenta |
| `logger.path` \* | `RUSTDESK_API_LOGGER_PATH` | vuoto | ignorata: il log va solo su stdout; se impostata, all'avvio un warn lo dice |
| `logger.level` \* | `RUSTDESK_API_LOGGER_LEVEL` | `info` | `debug`, `info`, `warn`, `error`; un altro valore ferma l'avvio |
| `logger.report-caller` | `RUSTDESK_API_LOGGER_REPORT_CALLER` | `true` | `source` (funzione, file e riga del codice) in ogni riga di log |
| `proxy.enable` | `RUSTDESK_API_PROXY_ENABLE` | `false` | proxy HTTP per le richieste dell'API al provider OAuth/OIDC |
| `proxy.host` | `RUSTDESK_API_PROXY_HOST` | `http://127.0.0.1:1080` | indirizzo del proxy |
| `jwt.key` | `RUSTDESK_API_JWT_KEY` | vuoto | vuota: token di sessione casuali (16 byte, esadecimale); impostata: token JWT firmati con questa chiave. Col server ufficiale lasciala vuota |
| `jwt.expire-duration` | `RUSTDESK_API_JWT_EXPIRE_DURATION` | `168h` | durata dei JWT |
| `ldap.enable` | `RUSTDESK_API_LDAP_ENABLE` | `false` | login LDAP; se LDAP rifiuta o non risponde, si prova l'utente locale |
| `ldap.url` | `RUSTDESK_API_LDAP_URL` | `ldap://ldap.example.com:389` | `ldap://` o `ldaps://` |
| `ldap.tls-ca-file` | `RUSTDESK_API_LDAP_TLS_CA_FILE` | vuoto | CA del server, con `ldaps://` |
| `ldap.tls-verify` | `RUSTDESK_API_LDAP_TLS_VERIFY` | `true` | con `ldaps://` verifica il certificato del server; per un certificato interno usa `ldap.tls-ca-file`. `false` non lo verifica, e all'avvio un warn lo dice |
| `ldap.base-dn` | `RUSTDESK_API_LDAP_BASE_DN` | `dc=example,dc=com` | DN di base |
| `ldap.bind-dn` | `RUSTDESK_API_LDAP_BIND_DN` | `cn=admin,dc=example,dc=com` | utente di servizio per le ricerche |
| `ldap.bind-password` | `RUSTDESK_API_LDAP_BIND_PASSWORD` | vuoto | password dell'utente di servizio: dalla variabile, non nel file |
| `ldap.user.base-dn` | `RUSTDESK_API_LDAP_USER_BASE_DN` | `ou=users,dc=example,dc=com` | dove cercare gli utenti |
| `ldap.user.filter` | `RUSTDESK_API_LDAP_USER_FILTER` | `(cn=*)` | filtro aggiunto alla ricerca |
| `ldap.user.username` | `RUSTDESK_API_LDAP_USER_USERNAME` | `uid` | attributo del nome utente (`sAMAccountName` in AD) |
| `ldap.user.email` | `RUSTDESK_API_LDAP_USER_EMAIL` | `mail` | attributo dell'email |
| `ldap.user.first-name` | `RUSTDESK_API_LDAP_USER_FIRST_NAME` | `givenName` | attributo del nome |
| `ldap.user.last-name` | `RUSTDESK_API_LDAP_USER_LAST_NAME` | `sn` | attributo del cognome |
| `ldap.user.enable-attr` | `RUSTDESK_API_LDAP_USER_ENABLE_ATTR` | vuoto | attributo che dice se l'utente e' attivo (`userAccountControl` in AD); vuoto = tutti attivi |
| `ldap.user.enable-attr-value` | `RUSTDESK_API_LDAP_USER_ENABLE_ATTR_VALUE` | vuoto | valore di `enable-attr` per un utente attivo (ignorato in AD) |
| `ldap.user.sync` | `RUSTDESK_API_LDAP_USER_SYNC` | `false` | `true` aggiorna l'utente locale a ogni login, `false` solo quando si crea |
| `ldap.user.admin-group` | `RUSTDESK_API_LDAP_USER_ADMIN_GROUP` | `cn=admin,dc=example,dc=com` | DN del gruppo degli amministratori |
| `ldap.user.allow-group` | `RUSTDESK_API_LDAP_USER_ALLOW_GROUP` | `cn=users,dc=example,dc=com` | DN del gruppo di chi puo' entrare; vuoto = tutti |

Il provider OIDC si configura dal pannello (OAuth, tipo `oidc`): serve
l'`Issuer`, `Scopes` di default `openid,profile,email` (`openid` si chiede
sempre, anche se non e' negli scope salvati), URL di callback
`<rustdesk.api-server>/api/oidc/callback`. Serve un provider che nella
risposta del token manda l'`id_token` (OIDC Core): senza, o con un
`id_token` che non si verifica, il login si ferma. L'autoregistrazione
(`auto_register`) e' spenta se non la si accende: un account del provider
senza utente viene rimandato ad associarsi dal pannello; accesa, l'utente
nasce al primo login.

## Primo avvio e amministrazione

**Password iniziale.** Al primo avvio l'API crea l'utente `admin` con una
password casuale di 20 caratteri e la scrive solo in `data/admin-password.txt`
(permessi 0600, accanto a `rustdeskapi.db`; nel container
`/app/data/admin-password.txt`); il log dice dov'e', non la stampa. Con
docker compose: `sudo cat remotek-data/admin-password.txt`. Entra su
`/_admin/`, cambia la password dal pannello e cancella il file.

**Comandi.** Dal binario (nel container: `docker compose exec remotek-api ./apimain ...`):

```bash
./apimain reset-admin-pwd <password>        # password di admin
./apimain reset-pwd <idUtente> <password>   # password di un altro utente
./apimain agente-ai <username> <tecnico>    # account di un agente AI
./apimain -h                                # aiuto
```

La password deve avere da 15 a 32 caratteri (caratteri, non byte), come ogni
password nuova impostata dal pannello o in registrazione. Se la password e'
rifiutata, l'utente non c'e' o l'aggiornamento non riesce, il comando esce
con codice diverso da 0.

**Agenti AI.** Ogni agente AI ha un account suo, legato al tecnico che ne
risponde (campo `agente_di` dell'utente: l'id del tecnico, 0 per una
persona). `agente-ai <username> <username del tecnico>` lo crea nel gruppo
del tecnico, mai amministratore, e stampa su stdout, una volta sola, la sua
password casuale di 24 caratteri (nel log no); esce con codice 1 se lo
username c'e' gia' o non ha da 2 a 32 caratteri (come nel pannello), se il
tecnico non c'e', non e' una persona o e' disattivato. Dal
pannello (`/api/admin/user/create` e `update`, campo `agente_di`) valgono le
stesse regole: il tecnico e' un altro utente che esiste ed e' una persona,
un agente non e' amministratore, e chi risponde di agenti non diventa un
agente. Un agente segue il suo tecnico: disattivare il tecnico disattiva i
suoi agenti; un agente il cui tecnico e' disattivato non si riattiva; un
tecnico che ha agenti non si cancella, prima si cancellano o si assegnano a
un altro tecnico i suoi agenti. Con `ldap.user.sync` la sincronizzazione non
da' a un agente il ruolo del gruppo `ldap.user.admin-group` (un warn lo
dice). Una modifica senza `agente_di`, come quelle del pannello di oggi, lo
lascia com'e'. Il client a riga di comando chiede con il token del login
`GET /api/agente`, che risponde `{"agente":true,"tecnico":"<nome>"}` (il
nickname del tecnico, o lo username se il nickname e' vuoto) o
`{"agente":false,"tecnico":""}`; `/api/login` e `/api/currentUser` non
cambiano.

**Collegare il client.** Nel client RustDesk o Remotek, in Impostazioni >
Rete: server ID, server relay, server API (`rustdesk.api-server`) e chiave,
gli stessi valori delle variabili `RUSTDESK_API_RUSTDESK_*`.

**Cambiare il marchio.** Nome, logo e favicon si cambiano senza toccare il
codice:

- nome: `brand.name` (`RUSTDESK_API_BRAND_NAME`). E' il titolo del pannello
  se `admin.title` e' vuoto, `{{brand}}` nel benvenuto e il titolo delle
  pagine del login OAuth/OIDC; vale dal riavvio;
- logo e favicon: `logo.svg` e `favicon.svg` in `brand.dir`, serviti su
  `/brand/logo.svg` e `/brand/favicon.svg`. Basta sostituire i file, senza
  ricostruire; nel container si montano, leggibili da 10001:
  `-v /srv/remotek/brand:/app/resources/brand:ro`. Un SVG deve bastare a se
  stesso: niente script, niente risorse di altri siti;
- il titolo che il pannello mostra prima di caricare la configurazione si
  fissa nella build: `docker build --build-arg BRAND_NAME=<nome> ...`.

Non cambiano nome: il prefisso `RUSTDESK_API_`, la sezione `rustdesk:`, le
rotte `/api/admin/rustdesk/*`, il module path Go.

## Sicurezza

- **Processo non root**: nell'immagine l'API gira come `remotek`
  (10001:10001). Una cartella dati non scrivibile da 10001 ferma l'avvio con
  un messaggio che dice il rimedio (`chown -R 10001:10001`).
- **Default sicuri**, anche nel codice se mancano dal file: registrazione
  spenta, login dal pannello (`web-sso`) spento, swagger spento, captcha dopo
  3 login sbagliati e ban dopo 10, nessun proxy fidato.
- **Credenziali**: password iniziale solo nel file 0600, mai nel log; password
  nuove di 15-32 caratteri; senza `jwt.key` token di sessione casuali.
- **Log** 0600: contiene nomi utente e indirizzi IP.
- **Nessuna risorsa esterna** nelle pagine che l'API genera (esito del login
  OAuth/OIDC). I file di `/brand/` escono con una `Content-Security-Policy`
  che non esegue script e con `X-Content-Type-Options: nosniff`.
- **LDAP**: con `ldaps://` il certificato del server si verifica
  (`ldap.tls-verify` vale `true`); per un certificato interno usa
  `ldap.tls-ca-file`. All'avvio un warn segnala `ldap://`, che manda le
  password in chiaro, e `ldap.tls-verify: false`.
- **Dispositivi**: sysinfo, heartbeat e audit arrivano dal client senza
  login; l'API li accetta solo dal dispositivo registrato. Il primo sysinfo
  di un ID lega l'ID all'uuid del PC (su Windows il MachineGuid); un PC
  creato dal pannello si lega al primo uuid che arriva. Da allora, con un
  altro uuid, sysinfo risponde "Il dispositivo non corrisponde a quello
  registrato." e heartbeat e audit non scrivono niente; nel log resta un
  warn con rotta e ID del PC. Se il PC cambia davvero uuid (reinstallazione
  di Windows, sostituzione) il legame si riapre cancellando il PC dal
  pannello mentre e' acceso: per esempio quando nel log arriva il warn col
  suo ID, che l'heartbeat manda ogni 15 secondi. Con il PC acceso e il
  servizio Remotek attivo, il sysinfo rifiutato si riprova da solo circa
  ogni 2 minuti e il primo tentativo dopo la cancellazione ricrea il PC col
  nuovo uuid; fino ad allora (al massimo circa 2 minuti) l'ID lo prende il
  primo che manda un sysinfo con quell'ID. Con il PC spento o il servizio
  fermo nessuno riprova, e l'ID resta libero finche' il PC non torna in
  linea. Riavviare subito il servizio Remotek sul PC (o il PC) chiude prima
  la finestra. Un PC cancellato mentre era legato al suo uuid attuale (per
  esempio per errore) va riavviato subito (il servizio Remotek o il PC): il
  client rimanda il sysinfo solo all'avvio del servizio o quando cambiano
  l'utente di Windows, l'ID o l'indirizzo dell'API, e fino al riavvio l'ID
  resta libero senza limite di tempo.
- Le vulnerabilita' si segnalano come dice [SECURITY.md](SECURITY.md), non
  con issue pubbliche.

## Origine e licenza

Remotek API e' basato su [rustdesk-api](https://github.com/lejianwen/rustdesk-api)
di lejianwen, versione v2.7, rilasciato con licenza MIT. Anche questo fork e'
MIT: [LICENSE](LICENSE) resta intatto, con l'attribuzione a lejianwen. Il
pannello e' [rustdesk-api-web](https://github.com/lejianwen/rustdesk-api-web),
dello stesso autore, anch'esso MIT, compilato dal sorgente e **modificato**
dalle patch in [`pannello/`](pannello/README.md), opera derivata con la
stessa licenza; il suo `LICENSE` e' nell'immagine in
`resources/admin/LICENSE`. Le modifiche rispetto a v2.7, patch del pannello
incluse, sono elencate in [REMOTEK.md](REMOTEK.md).
