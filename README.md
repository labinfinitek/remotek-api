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
  `/api/admin/audit_conn/list`, nella colonna "Nota" del registro delle
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
  compilato nell'immagine dal sorgente in [`pannello/`](pannello/README.md),
  modificato, con il marchio di Remotek.
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
`{"code":0,"message":"success","data":""}` (400 per un corpo che non e'
JSON, 413 per un corpo oltre 1 MiB, vedi Sicurezza): il client non la
guarda, e un blocco scartato non si rimanda.

Dal pannello, solo per gli amministratori, con `audit_conn_id` nella query
(l'id della riga nel registro delle connessioni): `GET /api/admin/audit_conn/terminal/list` elenca i blocchi in
ordine di `seq`, con `data` in base64 (paginato con `page` e `page_size`,
10 per pagina se manca); `GET /api/admin/audit_conn/terminal/verify`
ricalcola la catena dal database e risponde `integra`, `blocchi`, `fine`,
`hash` (quello salvato dell'ultimo blocco, da confrontare con la copia sul
PC) e `primo_errato`, il primo `seq` che non torna (0 se integra).
Cancellare dal pannello una connessione terminale cancella anche la sua
trascrizione.

`GET /api/admin/audit_conn/terminal/export`, con lo stesso `audit_conn_id`
e solo per gli amministratori, risponde un file JSON da scaricare,
`trascrizione-<audit_conn_id>.json`, con in quest'ordine:

| Campo | Contenuto |
|---|---|
| `connessione` | la riga del registro delle connessioni, con i campi dell'elenco del pannello |
| `blocchi` | i blocchi in ordine di `seq`: `seq`, `dir`, `data` (base64), `hash`, `fine`, `created_at` |
| `verifica` | `integra`, `blocchi`, `fine`, `hash`, `primo_errato`, come `verify`, sugli stessi blocchi del file |

I blocchi si leggono a pagine di 16, ognuno una volta, e vanno nel file
man mano; la verifica si calcola sugli stessi blocchi, quindi con una
sessione ancora in corso file e verifica restano coerenti tra loro. Mentre
il file arriva a chi scarica il database resta libero per le altre
richieste. Un id che non c'e' o che non e' di type 4 risponde l'errore del
pannello, senza file; nel log resta una riga info con l'id
dell'amministratore e `audit_conn_id`, senza contenuto. Per ricalcolare la catena dal file, fuori
dall'API: si parte da H(0), 32 byte a zero, e per ogni blocco in ordine si
calcola SHA-256( H(seq-1) ‖ d ‖ data ) con `data` decodificato dal base64
e d il byte `i` o `o`, come sopra; ogni hash deve essere uguale al campo
`hash` del blocco, i `seq` consecutivi da 1, e nessun blocco dopo quello
con `fine`. L'ultimo hash si confronta con la copia sul PC.

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

Tre stadi: binario Go statico (CGO per SQLite), build del pannello dal
sorgente in `pannello/`, immagine finale Alpine. Il build non scarica
sorgenti da GitHub; le immagini di base sono fissate per digest. Argomenti:

| Argomento | Default | Uso |
|---|---|---|
| `VERSION` | `dev` | scritto in `resources/version`, lo restituisce `/api/version` |
| `REVISION` | `unknown` | label OCI `org.opencontainers.image.revision` |
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
| `app.disable-pwd-login` | `RUSTDESK_API_APP_DISABLE_PWD_LOGIN` | `false` | `true` toglie il login con utente e password, anche quello LDAP, che passa di li'; resta OIDC. Restano le eccezioni: gli agenti AI dal client e `app.pwd-login-eccezioni` (Login con password spento) |
| `app.pwd-login-eccezioni` \* | `RUSTDESK_API_APP_PWD_LOGIN_ECCEZIONI` | vuoto | username separati da virgola (spazi tolti) che entrano con la password anche con `app.disable-pwd-login`, dal client e dal pannello; vuoto = nessuno. Con `app.disable-pwd-login` spento non conta |
| `app.firma-obbligatoria` \* | `RUSTDESK_API_APP_FIRMA_OBBLIGATORIA` | `false` | `true`: anche i PC senza chiave devono firmare le richieste senza login (Firma del dispositivo) |
| `admin.title` \* | `RUSTDESK_API_ADMIN_TITLE` | vuoto | titolo del pannello; vuoto = `brand.name` |
| `admin.hello` | `RUSTDESK_API_ADMIN_HELLO` | vuoto | messaggio di benvenuto; il pannello non lo mostra piu', la chiave resta per ora. Se non e' vuoto, `admin.hello-file` non si legge |
| `admin.hello-file` | `RUSTDESK_API_ADMIN_HELLO_FILE` | `./conf/admin/hello.html` | file del benvenuto (`{{username}}` e `{{brand}}` si sostituiscono); il pannello non lo mostra piu', la chiave resta per ora |
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
| `integrazione.url` \* | `RUSTDESK_API_INTEGRAZIONE_URL` | vuoto | URL `https://` a cui vanno gli eventi delle sessioni (Integrazione); vuoto = spenta, nessun evento. Senza credenziali nell'URL; `http://` o credenziali fermano l'avvio. Solo dalla configurazione, mai dal pannello |
| `integrazione.segreto` \* | `RUSTDESK_API_INTEGRAZIONE_SEGRETO` | vuoto | segreto della firma degli eventi, almeno 32 caratteri se `integrazione.url` e' impostato (altrimenti l'avvio si ferma): dalla variabile, non nel file; mai nel log |
| `agente.lasciapassare-chiave` \* | `RUSTDESK_API_AGENTE_LASCIAPASSARE_CHIAVE` | vuoto | file col seme Ed25519 dei lasciapassare degli agenti AI (Lasciapassare dell'agente); vuoto = spento. Impostato, il file deve esistere, avere permessi 600 o piu' stretti e una chiave valida, altrimenti l'avvio si ferma |
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
`<rustdesk.api-server>/api/oidc/callback`. Il segreto del client si scrive
alla creazione e non si rilegge: lista e dettaglio del provider dicono solo
se c'e' (`has_client_secret`); in modifica, lasciato vuoto o di soli
spazi, resta quello salvato, e per cambiarlo si scrive quello nuovo. Gli scope si separano con
la virgola; gli spazi intorno e le voci vuote si tolgono. Serve un provider che nella
risposta del token manda l'`id_token` (OIDC Core): senza, o con un
`id_token` che non si verifica, il login si ferma. L'autoregistrazione
(`auto_register`) e' spenta se non la si accende: un account del provider
senza utente riceve un codice di associazione (sotto, "Aggiungere un
tecnico con il login con password spento") e, con il login con password
acceso, viene anche rimandato ad associarsi dal pannello; accesa, l'utente
nasce al primo login.

Il login OIDC trova l'utente dall'*associazione* (`user_thirds`: provider e
`sub` dell'account), non dall'email. Un `sub` e' associato a un utente solo
(indice unico su `open_id` e `op`, creato all'avvio; con doppioni un error
col numero delle righe doppie, l'avvio continua senza indice e riprova al
prossimo), e un
utente ha al massimo un account per provider: l'associazione dal pannello
rifiuta un account gia' di qualcuno ("Questo account OIDC è già associato a
un altro utente.") e un utente che ne ha gia' uno ("Questo utente ha già un
account associato a questo provider OIDC."). Con `auto_register` acceso, un
account senza associazione si abbina all'utente con la stessa email solo se
il provider manda `email_verified` vero e l'utente non e' un agente AI ne'
disattivato; se l'email e' di un utente che non si abbina il login si ferma
("Questo account OIDC non si può abbinare all'utente con la stessa email"),
se non e' di nessuno nasce un utente nuovo. Un utente disattivato non entra
neanche con OIDC: il login risponde "Utente disattivato." senza token.

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
./apimain token-integrazione crea <nome>    # token di integrazione (Integrazione)
./apimain token-integrazione elenco
./apimain token-integrazione revoca <nome>
./apimain lasciapassare chiave-nuova <file> # chiave dei lasciapassare (Lasciapassare dell'agente)
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

Un agente usa solo il client a riga di comando (ADR-0028). Tra le rotte con
login gli rispondono solo `GET /api/user/info`, `POST /api/currentUser`,
`POST /api/logout`, `GET /api/agente` e `POST /api/agente/lasciapassare`;
le altre (rubriche, PC, utenti, note) rispondono 403 `{"error": msg}`, con
un warn. Il login del pannello (`POST /api/admin/login`, e quello OIDC) lo
rifiuta senza dare un token, e ogni rotta del pannello con login risponde
col code 403, che per il pannello e' un logout, anche a un token gia'
rilasciato. Cosi' un agente non puo' crearsi rubriche o regole, ne'
scrivere in quelle condivise, per allargarsi il confine del lasciapassare.
Il CLI non ha browser ne' TOTP: con `app.disable-pwd-login` acceso un
agente entra lo stesso con la password da `POST /api/login`, mai dal
pannello.

**Login con password spento.** Con `app.disable-pwd-login` i tecnici entrano
solo con OIDC. Con la password entrano ancora gli account agente, solo da
`POST /api/login`, e gli username di `app.pwd-login-eccezioni` (per esempio
l'utente di collaudo e un amministratore di emergenza), da `/api/login` e da
`/api/admin/login`. A tutti gli altri, a uno username che non esiste e a
un'eccezione con la password sbagliata la risposta e' la stessa:
"L'accesso con password è disattivato.", senza dire chi e' un'eccezione;
ogni rifiuto conta per captcha e ban. Anche i tempi non lo dicono: ogni
richiesta fa un controllo bcrypt, vero o su un hash finto, anche per
un'eccezione senza utente locale o con un hash che non e' bcrypt. Con
`ldap.enable` acceso invece i tempi distinguono le eccezioni, perche' solo
loro passano da LDAP prima del database: chi misura le risposte puo'
capire quali nomi sono nell'elenco o sono agenti. Un login per eccezione
riuscito scrive una riga nel log, INFO per un utente e WARN per un
amministratore:

```
login con password per eccezione: utente <username>, motivo <motivo>
login con password per eccezione: utente <username> (amministratore), motivo <motivo>
```

`<motivo>` e' `agente` (account agente, solo da `/api/login`) o `elenco`
(`app.pwd-login-eccezioni`).
**Accesso di emergenza:** se il provider OIDC non risponde, il pannello
mostra il modulo della password su `/_admin/#/login?password=1`, anche
con un solo provider (che senza il parametro apre subito l'OIDC); entra
solo uno username dell'elenco.

**Aggiungere un tecnico con il login con password spento.** Con
`auto_register` spento l'account OIDC di un tecnico nuovo va associato a un
utente da un amministratore:

1. l'amministratore crea l'utente (Sistema, Utenti), senza password da
   comunicare;
2. il tecnico entra con OIDC dal client o dal pannello: la pagina del
   callback dell'API risponde "Il tuo account <provider> non è ancora associato a un
   utente Remotek. Comunica all'amministratore questo codice: K7Q2-M9XD.
   Vale 30 minuti; poi rifai l'accesso.";
3. l'amministratore apre Sistema, Associazioni in attesa, trova il codice
   (con nome ed email che il provider ha mandato, e se l'email e'
   verificata) e sceglie "Associa a" con l'utente del punto 1;
4. il tecnico rifa' l'accesso con OIDC ed entra.

Il codice ha 8 caratteri senza 0/O e 1/I/L, a gruppi di 4; un nuovo accesso
dello stesso account lo rinnova. Le richieste stanno in memoria, al massimo
100 (oltre esce la piu' vecchia), e un riavvio dell'API le perde.
L'associazione rifiuta gli account agente, gli utenti disattivati, un
account gia' associato e un utente che ha gia' un account dello stesso
provider. Nel log una riga INFO, senza `sub` ne' email:

```
associazione OIDC dall'amministratore: utente <id>, provider <op>, amministratore <id>
```

**Collegare il client.** Dal client `remotek-1.4.9-3` Remotek ha server ID,
server relay, server API e chiave bloccati nel binario (ADR-0025): da
Impostazioni > Rete non si cambiano. Per un server di prova si cambiano solo
con un `custom.txt` firmato da Infinitek. Un client RustDesk generico invece
si collega in Impostazioni > Rete: server ID, server relay, server API
(`rustdesk.api-server`) e chiave, gli stessi valori delle variabili
`RUSTDESK_API_RUSTDESK_*`.

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

**Marchio del pannello.** Cio' che e' di chi installa non sta nel codice del
pannello: lo legge, se c'e', da `brand.json` in `brand.dir`, servito su
`/brand/brand.json` accanto al logo. Il file e' facoltativo e ogni campo pure:
senza, il pannello non mostra ragione sociale ne' informativa, il pulsante del
login dice "Accedi con <provider OIDC>" con l'icona di sempre e il tema scuro
usa `logo.svg`. Formato (tutti stringhe):

```json
{
  "ragione_sociale": "Esempio S.r.l.",
  "informativa": "https://esempio.it/privacy",
  "nome_accesso": "Esempio",
  "logo_scuro": "logo-scuro.svg",
  "icona_accesso": "marchio.svg"
}
```

- `ragione_sociale` (massimo 120 caratteri): in fondo al login e nel piede
  del pannello, dopo il nome;
- `informativa`: link all'informativa sulla privacy in fondo al login; solo
  indirizzi `https://`, gli altri si ignorano;
- `nome_accesso` (massimo 60 caratteri) e `icona_accesso`: nome e icona del
  pulsante OIDC del login ("Accedi con <nome_accesso>"); valgono solo con un
  provider OIDC, con piu' provider ognuno tiene il suo nome;
- `logo_scuro`: logo per la barra e il login nel tema scuro (fondo navy).

`logo_scuro` e `icona_accesso` sono nomi di file nella stessa cartella
(`.svg`, `.png` o `.webp`, niente percorsi), con le stesse regole di
`logo.svg`. Un campo non valido vale come assente; il pannello rilegge il file
a ogni apertura.

Non cambiano nome: il prefisso `RUSTDESK_API_`, la sezione `rustdesk:`, il
module path Go.

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
- **Corpo delle richieste**: al massimo 1 MiB, su ogni rotta, anche quelle
  senza login. Con `Content-Length` oltre il limite l'API risponde 413
  `{"error": ...}` (sotto `/api/admin/` `{"code":101,"message":...}`, come
  il resto del pannello) senza leggere il corpo; senza `Content-Length` legge al
  massimo 1 MiB e oltre risponde come a un corpo sbagliato. Nel log un warn
  con metodo, rotta e `request_id`, senza il corpo. Il corpo vero piu'
  grande, un blocco della trascrizione del terminale, e' circa 88 KB.
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
  resta libero senza limite di tempo. Il client che firma aggiunge la
  chiave del PC: vedi Firma del dispositivo.
- Le vulnerabilita' si segnalano come dice [SECURITY.md](SECURITY.md), non
  con issue pubbliche.

## Firma del dispositivo

L'uuid che lega l'ID al PC (Sicurezza, Dispositivi) il client lo manda anche a
`hbbs` in chiaro, su UDP: chi osserva la rete lo legge e puo' scrivere a nome
del PC. Per questo il PC firma le sue richieste senza login (sysinfo,
heartbeat, audit/conn, audit/file, audit/terminal) con la sua chiave Ed25519,
e l'API, registrata la chiave, accetta per quel PC solo richieste firmate. Il
client 1.4.9 non firma e resta con le regole dell'uuid finche' non si
aggiorna.

**Formato** (e' il contratto col client). Intestazione:

```
X-Remotek-Firma: <ts>.<firma>
```

- `ts`: secondi UNIX in decimale (solo cifre), l'ora del PC quando firma;
- `firma`: base64 standard, con `=`, dei 64 byte della firma Ed25519
  "detached" del messaggio.

Il messaggio sono cinque righe unite da `\n`, senza `\n` finale:

```
remotek-api-v1
<metodo in maiuscolo, per esempio POST>
<percorso della richiesta, senza query, per esempio /api/heartbeat>
<ts, lo stesso dell'intestazione, carattere per carattere>
<SHA-256 dei byte del corpo, esadecimale minuscolo>
```

Il corpo e' quello che il client manda, byte per byte: la firma si verifica
sui byte arrivati, prima del JSON. Il percorso e' quello che arriva all'API
(dietro un reverse proxy che toglie un prefisso, quello senza prefisso). Il
sysinfo porta in piu' il campo `pk`: base64 standard dei 32 byte della chiave
pubblica Ed25519 del PC.

**Vettore di prova** (in `http/firma_test.go`, `TestFirmaVettore`): con la
chiave dal seed di 32 byte `00 01 02 ... 1f` e la richiesta

- metodo `POST`, percorso `/api/heartbeat`, ts `1791000000`,
- corpo `{"id":"999000111","uuid":"dXVpZA=="}`

si hanno SHA-256 del corpo
`8ce852e69dbc62f20c29f8fcfb77e1a1922d73c7a185de173bb31139f4089d10`, `pk`
`A6EHv/POEL4dcN0Y50vAmWfk1jCbpQ1fHdyGZBJVMbg=` e intestazione

```
X-Remotek-Firma: 1791000000.otUjpW4BdAQnl0TuN1E/GmD3LAx38zzpYTczckv70y3hYSIVtkA7urCTqftTR9BXPMoDc//+nyxItL73nbrHCw==
```

**Regole**:

1. Un PC con la chiave registrata: la richiesta deve avere l'intestazione,
   una firma valida con quella chiave, un `ts` entro 300 secondi dall'ora del
   server, e la stessa firma non deve essere gia' stata accettata nei 300
   secondi prima. L'uuid deve restare quello salvato, come prima. La memoria
   delle firme accettate e' nel processo: dopo un riavvio dell'API la
   finestra riparte, e una firma accettata appena prima del riavvio si puo'
   ripetere finche' il suo `ts` resta nei 300 secondi.
2. Un PC senza chiave: come prima (uuid), con o senza firma.
3. Registrazione: un sysinfo con `pk` e firma valida per quella `pk` salva la
   chiave solo se il PC e' nuovo (lo crea, come prima) o ha gia' quell'uuid,
   e solo se il PC non ha ancora una chiave. Un PC creato dal pannello senza
   uuid prende l'uuid col primo sysinfo e la chiave col successivo. Una
   chiave salvata non la cambia ne' la cancella nessun sysinfo; si riapre
   solo cancellando il PC dal pannello, come per l'uuid.
4. Un sysinfo con l'uuid giusto ma firmato con una `pk` diversa da quella
   salvata non cambia niente, risponde come a un altro dispositivo e scrive
   nel log un **error** con rotta e ID del PC. Vuol dire un PC reinstallato
   (chiave nuova), oppure qualcuno che conosceva l'uuid e ha registrato la
   sua chiave prima del PC vero. Rimedio: cancellare il PC dal pannello e
   guardare chi si registra (nome, utente e IP della scheda nuova); se non
   e' il PC vero, cancellarlo di nuovo e riavviare il servizio Remotek sul
   PC vero.
5. Una richiesta rifiutata ha la risposta di prima per quella rotta (sysinfo:
   "Il dispositivo non corrisponde a quello registrato."; heartbeat: `{}`;
   audit: successo senza salvare) e nel log un warn con rotta, ID del PC e
   motivo (`firma assente`, `firma non valida`, `firma fuori tempo`, `firma
   ripetuta`, `firma di un PC senza chiave registrata`), mai firma, chiave
   ne' uuid. Un sysinfo accettato di un PC senza chiave, con una `pk` che non
   si registra, lo dice con un warn `chiave non registrata: <motivo>`; con
   un uuid diverso da quello salvato e la firma non obbligatoria la firma
   non si verifica nemmeno.
6. Una chiave registrata, di un PC nuovo o di uno che non l'aveva, scrive
   nel log una riga **info** `chiave del PC registrata` con rotta e ID del
   PC, una volta sola; mai chiave, uuid ne' firma.

**Campi sconosciuti.** Le rotte del dispositivo (sysinfo, heartbeat,
audit/conn, audit/file, audit/terminal) ignorano i campi del corpo che non
conoscono: un client piu' nuovo dell'API puo' mandarne in piu' senza che la
risposta cambi. Il client dal `remotek-1.4.9-3` manda `ms` negli audit dei
file, che l'API non salva. La firma copre il corpo intero, campi ignorati
compresi.

**Opzione** `app.firma-obbligatoria` (`RUSTDESK_API_APP_FIRMA_OBBLIGATORIA`,
default `false`): se vera, anche un PC senza chiave deve firmare. Il suo
sysinfo firmato con `pk` e uuid giusto registra la chiave (un PC nuovo lo
crea); ogni altra sua richiesta e' rifiutata, senza firma (`firma assente`)
o con una firma che non c'e' chiave per verificare (`firma di un PC senza
chiave registrata`), finche' la chiave non e' registrata. Va accesa quando
tutti i PC hanno un client che firma: un client 1.4.9 smette di scrivere.

## Copie dello stesso PC

Un PC copiato senza sysprep (VM clonata, immagine disco) porta con se' ID,
uuid e chiave di Remotek: la copia si registra come il PC vero e la firma
non le distingue, perche' la chiave e' la stessa. `hbbs` alterna i due
indirizzi e l'API riceve heartbeat e audit di tutte e due sulla stessa riga.
L'API lo segnala; non blocca nessuna richiesta e il client non cambia.

**Regola.** Per ogni PC riconosciuto (ID e uuid salvati, firma valida dove
serve) l'API ricorda l'indirizzo e l'ora dell'ultimo heartbeat; l'indirizzo
e' lo stesso di `last_online_ip`. Un *cambio* e' un heartbeat dello stesso
PC da un indirizzo diverso dal precedente, arrivato meno di 60 secondi
dopo: il client ne manda uno ogni 15 secondi, quindi due copie accese fanno
un cambio quasi a ogni heartbeat. Con almeno **10 cambi negli ultimi 10
minuti** il PC e' "possibile copia":

- nella lista dei PC del pannello (`GET /api/admin/peer/list`, campo
  `possibile_copia`) accanto all'ID compare l'etichetta rossa "Possible
  copy", che sparisce appena negli ultimi 10 minuti restano meno di 10
  cambi: al piu' 10 minuti dopo l'ultimo cambio, spesso prima;
- nel log una riga warn `possibile copia del PC` con `id`, i due indirizzi
  piu' recenti (`indirizzo`, `indirizzo_precedente`) e il numero di `cambi`,
  al massimo una all'ora per PC; mai uuid ne' chiavi.

Un PC senza heartbeat da 10 minuti esce dalla memoria; l'ora del suo
ultimo avviso ci resta per un'ora, perche' una copia zitta per un po' non
abbia un secondo avviso prima dell'ora.

**Cosa fare.** Sulla copia si toglie Remotek con la sua configurazione e lo
si reinstalla: prende un ID nuovo. Per le copie future: sysprep prima di
clonare, oppure Remotek installato dopo la copia.

**Limiti.** E' un avviso da verificare, non un blocco:

- una rete con due linee che bilancia ogni connessione su un indirizzo
  diverso da' lo stesso segno senza alcuna copia;
- due copie dietro lo stesso indirizzo pubblico non si vedono;
- la memoria e' nel processo: dopo un riavvio dell'API riparte vuota;
- dietro un proxy senza `gin.trust-proxy` (Configurazione) tutti i PC hanno
  l'indirizzo del proxy e la regola non vede niente.

## Integrazione (eventi)

Il supporto lavora in Zammad: l'API manda a n8n (rete privata) un evento
firmato per ogni fatto di una sessione, e n8n porta nel ticket cio' che
serve; il tecnico scrive il numero del ticket nella nota di sessione (per
esempio `#83100`). L'API non conosce Zammad (ADR-0026).

**Configurazione.** `integrazione.url` e `integrazione.segreto`
(Configurazione). Senza URL non si accoda niente e l'API si comporta come
prima. Con l'URL, all'avvio: deve essere `https://`, senza credenziali, e il
segreto deve avere almeno 32 caratteri, altrimenti l'avvio si ferma.

**Consegna.** Gli eventi si salvano nella tabella `eventi` del database
(versione 271) nella stessa richiesta del client; se non si salvano, un
warn `evento ... non accodato` e al client la risposta di sempre. Un worker
nel processo dell'API, avviato e fermato con lei, li manda uno alla volta in
ordine di creazione tra quelli pronti: `POST` JSON, timeout 10 secondi,
nessun redirect seguito. Consegnato = risposta 2xx. Altrimenti si riprova
dopo 30 secondi, poi 1, 2, 4... minuti fino a un'ora tra un tentativo e
l'altro, per 24 ore dalla creazione; poi una riga ERROR `evento non
consegnato` con tipo e id, e l'evento resta segnato `fallito`. Ogni
tentativo fallito e' un warn, senza l'URL. Consegnati e falliti si
cancellano 7 giorni dopo. La coda sta nel database: dopo un riavvio riparte
da dove era. La consegna e' *almeno una volta*: chi riceve scarta i
doppioni con `X-Remotek-Evento-Id`; un evento che si riprova non ferma gli
altri, quindi l'ordine sicuro e' quello di `ora`.

**Tipi.**

| Tipo | Quando |
|---|---|
| `connessione.aperta` | la connessione e' autorizzata: il PC manda a `POST /api/audit/conn`, senza `action`, tecnico (`peer`) e tipo, che l'evento porta; `inizio` e' l'ora del `new` all'apertura del socket. Una connessione mai autorizzata non ha questo evento; una ripetizione uguale non ne fa un secondo |
| `connessione.chiusa` | il PC registra la chiusura; con `fine` e `durata_s`. Parte anche per le connessioni mai autorizzate (rifiutate o senza clic): allora `tecnico` e' vuoto e `tipo` e' 0 |
| `connessione.nota` | la nota di sessione e' scritta o cambiata (durante la sessione o a fine connessione); una nota uguale a quella salvata non fa evento. Il testo e' quello del registro, troncato a 2000 caratteri |
| `file.trasferiti` | il PC registra un trasferimento di file (`POST /api/audit/file`); con `file.direzione` (`dal_pc`: il PC controllato manda i file, type 0 del registro; `al_pc`: li riceve, type 1) e `file.numero`. Nomi e percorsi restano nel pannello |
| `terminale.chiuso` | arriva il blocco con `fine` della trascrizione di una sessione terminale; con `sessione` e `verifica` (`integra`, `hash` finale), calcolata come la verifica del pannello. Niente blocchi ne' dati. `sessione.fine` e' `null` se il PC registra la chiusura dopo |
| `pc.nuovo` | un sysinfo crea un PC che l'API non conosceva |
| `pc.possibile_copia` | l'API scrive il warn `possibile copia del PC` (Copie dello stesso PC), al massimo uno all'ora per PC; con `copia.cambi`, senza gli indirizzi |
| `agente.lasciapassare` | l'API rilascia un lasciapassare a un agente AI (Lasciapassare dell'agente); con `agente.agente` e `agente.tecnico` (gli username) e `agente.amministratore` (`true`). Mai il lasciapassare ne' il nonce |

**Campi.** Ogni evento ha `schema`, `id`, `tipo`, `ora` e `pc`; oltre a
questi, e nient'altro:

- `connessione.aperta`, `connessione.chiusa`: `sessione` (con
  `amministratore`), e `agente` se la sessione e' verificata col
  lasciapassare (Lasciapassare dell'agente);
- `connessione.nota`: `sessione`;
- `file.trasferiti`: `file`;
- `terminale.chiuso`: `sessione` e `verifica`;
- `pc.nuovo`: nessun altro campo;
- `pc.possibile_copia`: `copia`;
- `agente.lasciapassare`: `agente`.

Per esempio:

```json
{
  "schema": 1,
  "id": "5f0c7f0e-7d1a-4c55-9a43-0b8e8f1f2a10",
  "tipo": "connessione.chiusa",
  "ora": "2026-10-11T09:01:30Z",
  "pc": {"id": "999000111", "nome": "PC-ROSSI", "alias": "Rossi"},
  "sessione": {
    "conn_id": 7, "tipo": 0, "tecnico": "mario",
    "inizio": "2026-10-11T09:00:00Z", "fine": "2026-10-11T09:01:30Z",
    "durata_s": 90, "nota": "#83100", "amministratore": false
  }
}
```

`id` e' un uuid casuale dell'evento, `ora` quando e' nato (RFC 3339, UTC).
`pc.nome` e' l'hostname. `sessione.tipo` e' il type del registro, come nel
pannello (4 = terminale), `tecnico` il nome del tecnico (`from_name`);
`fine` e `durata_s` sono `null` finche' la connessione e' aperta;
`amministratore` (solo in aperta e chiusa) dice se la shell ha i permessi
di amministratore, come l'ha detto il PC.
Gli altri oggetti:

```json
"file": {"direzione": "dal_pc", "numero": 3}
"verifica": {"integra": true, "hash": "9f2c...e01a"}
"copia": {"cambi": 10}
"agente": {"agente": "agente-mario", "tecnico": "mario", "amministratore": true}
```

**Firma.** Ogni richiesta ha gli header `X-Remotek-Evento-Id` (l'`id`) e
`X-Remotek-Evento-Firma: t=<secondi UNIX>,v1=<hex>`, con `v1` =
HMAC-SHA256(segreto, `"<t>."` + corpo esatto ricevuto). Chi riceve ricalcola
`v1` sui byte del corpo, prima di decodificare il JSON, e lo confronta a
tempo costante; scarta `t` troppo lontano dalla sua ora (per esempio oltre 5
minuti: un evento ripetuto ha un `t` nuovo). Vettore di prova: segreto
`segreto-di-prova-di-almeno-32-caratteri`, `t` = `1791000000`, corpo
`{"schema":1,"tipo":"prova"}` danno
`v1=47977e5349c23bd14a1a9d3200bed968a3092152ca7f73b73c6581a5f0b47013`.

```sh
printf '%s.%s' 1791000000 '{"schema":1,"tipo":"prova"}' \
  | openssl dgst -sha256 -hmac 'segreto-di-prova-di-almeno-32-caratteri'
```

**Cosa non esce, mai.** uuid, chiavi, firme, token, guid, `session_id`,
indirizzi IP, contenuto e nomi dei file, trascrizioni del terminale.

### Token di integrazione (sola lettura)

n8n puo' chiedere dettagli all'API con un token di integrazione, che legge
e basta (ADR-0026).

**Comandi** (Primo avvio e amministrazione):

- `token-integrazione crea <nome>` crea il token e lo stampa su stdout una
  volta sola: 32 byte casuali in base64url. Nel database (`token_integrazione`,
  versione del database 272) c'e' solo il suo SHA-256: se si perde, se ne crea un
  altro. Il nome ha da 2 a 64 caratteri tra lettere, cifre, `.`, `-` e `_`,
  e non si ripete. Il token scade dopo un anno.
- `token-integrazione elenco` stampa nome, creazione, scadenza, revoca e
  ultimo uso (RFC 3339, `-` se non c'e'), mai token ne' hash.
- `token-integrazione revoca <nome>` lo revoca subito. Un nome che non c'e'
  esce con codice 1.

**Rotte.** Solo `GET`, con `Authorization: Bearer <token>` e nessun'altra
credenziale. Il token di integrazione non vale su nessun'altra rotta, e un
token di login non vale su queste. Senza token, o con un token sconosciuto,
scaduto o revocato, la risposta e' 401 `{"error": "Unauthorized"}`; un token
scaduto o revocato lascia nel log un warn col suo nome. Il token non va mai
nel log.

| Rotta | Risposta |
|---|---|
| `GET /api/integrazione/pc` | `elenco` dei PC (`id`, `nome` = hostname, `alias`, `ultimo_contatto` RFC 3339 o `null`, `versione`, `possibile_copia`), `totale`, `pagina`, `per_pagina` |
| `GET /api/integrazione/connessioni` | registro delle connessioni in ordine di `id`, con i campi della `sessione` degli eventi piu' `id` (della riga) e `pc`; filtri `pc` (ID del PC) e `da` (RFC 3339, connessioni iniziate da allora) |
| `GET /api/integrazione/connessioni/{id}/verifica` | esito della verifica della trascrizione della connessione `id` (`integra`, `blocchi`, `fine`, `hash`, `primo_errato`), come la verifica del pannello; 404 se la connessione non c'e' o non e' un terminale |

Le due liste vanno a pagine: `pagina` (da 1 a 1000000) e `per_pagina` (da 1
a 1000, default 100); un valore non valido risponde 400. `da` si confronta
come istante, qualunque sia il fuso: `2026-10-11T00:00:00Z` o
`2026-10-11T02:00:00%2B02:00`, col `+` scritto `%2B` nella query
(altrimenti diventa uno spazio e la risposta e' 400). Non escono mai uuid,
chiavi, IP, guid, `session_id`, nomi o percorsi di file, trascrizioni.
Nessuna rotta scrive niente e nessuna esporta le trascrizioni.

## Lasciapassare dell'agente (ADR-0028)

Il terminale dell'agente AI (`remotek-cli`, con l'account dell'agente: vedi
Agenti AI) puo' chiedere i permessi di amministratore su un PC: su un PC
presidiato li accetta il cliente nella finestra, su un PC non presidiato
abilitato dal suo `custom.txt` firmato entra senza clic. Il nome che il
client dichiara nessuno lo verifica, quindi prima di collegarsi il CLI
chiede all'API un *lasciapassare* firmato per quel PC, e il PC lo verifica
con la chiave pubblica dell'API cablata nel suo binario. Il lasciapassare
vale 60 secondi per aprire la sessione, una volta sola, per quel solo PC.

**Configurazione.** `agente.lasciapassare-chiave`
(`RUSTDESK_API_AGENTE_LASCIAPASSARE_CHIAVE`): percorso del file con il seme
Ed25519 di 32 byte in base64, su una riga. Vuoto (il default) la funzione
e' spenta e la rotta lo dice. Impostato, all'avvio il file deve esistere,
non essere leggibile da gruppo e altri (permessi 600 o piu' stretti; nel
container del proprietario 10001) e contenere una chiave valida, altrimenti
l'avvio si ferma con un messaggio che dice cosa non va. All'avvio una riga
INFO `lasciapassare dell'agente acceso, chiave pubblica <base64>`: la
chiave pubblica non e' un segreto e serve a confrontarla con quella del
client. Il seme non va mai nel log.

**Comando.** `./apimain lasciapassare chiave-nuova <file>` genera la coppia
con `crypto/rand`, scrive il seme nel file con permessi 600 e stampa su
stdout la chiave pubblica in base64. Non legge la configurazione ne' il
database, e rifiuta con codice 1 un file che esiste gia': una chiave non si
sovrascrive per sbaglio. Con docker compose, per esempio:
`docker compose exec remotek-api ./apimain lasciapassare chiave-nuova
/app/data/lasciapassare.key`, poi
`RUSTDESK_API_AGENTE_LASCIAPASSARE_CHIAVE=/app/data/lasciapassare.key`.

**Rotta.** `POST /api/agente/lasciapassare`, col token del login
dell'agente (`Authorization: Bearer`), corpo `{"id": "<ID del PC>"}`.
Risponde `{"lasciapassare": "<stringa>"}` solo se la funzione e'
configurata, l'utente del token e' un agente AI attivo, il suo tecnico
esiste ed e' attivo, il PC e' in `peers` ed e' nella rubrica del tecnico:
una sua rubrica, personale o no, o una rubrica condivisa con lui o con il
suo gruppo dalle regole delle rubriche (quelle che il client RustDesk vede
per lui). Un agente lavora solo sui PC su cui lavora il suo tecnico; un PC
fuori dalla rubrica lascia anche un warn `lasciapassare rifiutato` con
agente, tecnico e PC. Le rubriche di un agente AI, o di un proprietario che
non c'e' piu', non contano anche se condivise col tecnico; e un account
agente non puo' scrivere rubriche ne' entrare nel pannello (Agenti AI). Altrimenti 400 `{"error": msg}`,
che dice quale condizione manca (in italiano o in inglese); senza token, o
con l'agente disattivato, 401 come le altre rotte con login. Ogni
lasciapassare va nella tabella `lasciapassare` (versione del database 273):
nonce, PC, agente, tecnico, emesso, scade, `conn_id` (0 finche' non apre
una sessione); mai il lasciapassare intero ne' la firma. Nel log una riga
INFO `lasciapassare rilasciato` con agente, tecnico e PC, mai il
lasciapassare ne' il nonce; per l'integrazione l'evento
`agente.lasciapassare`.

**Formato.**

- payload = JSON compatto, campi in quest'ordine:
  `{"v":1,"pc":"<ID>","agente":"<username>","tecnico":"<username del tecnico>","modo":"amministratore","emesso":<unix>,"scade":<emesso+60>,"nonce":"<16 byte casuali, base64url senza padding>"}`;
- firma = Ed25519(seme, `"remotek-lasciapassare-v1."` + payload);
- lasciapassare = base64url(payload) + `.` + base64url(firma), senza padding.

Vettore di prova (Ed25519 e' deterministica): il seme dei byte da 0 a 31
(`AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=`, chiave pubblica
`A6EHv/POEL4dcN0Y50vAmWfk1jCbpQ1fHdyGZBJVMbg=`) e il payload

```json
{"v":1,"pc":"999000111","agente":"agente-mario","tecnico":"mario","modo":"amministratore","emesso":1791000000,"scade":1791000060,"nonce":"AAECAwQFBgcICQoLDA0ODw"}
```

danno il lasciapassare

```text
eyJ2IjoxLCJwYyI6Ijk5OTAwMDExMSIsImFnZW50ZSI6ImFnZW50ZS1tYXJpbyIsInRlY25pY28iOiJtYXJpbyIsIm1vZG8iOiJhbW1pbmlzdHJhdG9yZSIsImVtZXNzbyI6MTc5MTAwMDAwMCwic2NhZGUiOjE3OTEwMDAwNjAsIm5vbmNlIjoiQUFFQ0F3UUZCZ2NJQ1FvTERBME9EdyJ9.BRkkm77ln4n8PxwF7Jmi3_-LiCHX7xGjLFjiXb3Knpc3kg9Rlv6DNoxForK3F28DLaUF5HWkudV4ZaWexK4YDA
```

**Come lo verifica il PC.** Divide la stringa al punto, decodifica le due
parti (base64url senza padding) e verifica la firma con la chiave pubblica
cablata sui byte `"remotek-lasciapassare-v1."` + payload esattamente come
sono arrivati, prima di decodificare il JSON. Poi accetta solo se `v` e' 1,
`modo` e' `amministratore`, `pc` e' uguale al proprio ID,
`emesso - 30 <= adesso <= scade + 30` (30 secondi di tolleranza per gli
orologi) e il `nonce` non l'ha mai visto prima; altrimenti rifiuta. La
finestra mostra `agente` e `tecnico` del payload, non il nome dichiarato
dal client.

**Collegamento alla connessione.** Il PC, quando apre la sessione col
lasciapassare, manda il suo `nonce` nel campo facoltativo `lasciapassare` di
`POST /api/audit/conn` (rotta del PC, firmata come le altre: Firma del
dispositivo), col `new` o con l'autorizzazione. Se il nonce e' di un
lasciapassare rilasciato per quello stesso PC e non ha ancora aperto una
sessione, l'API lega il lasciapassare alla riga del registro: scrive sul
lasciapassare `audit_conn_id`, l'`id` della riga di `audit_conns`
(versione del database 275), e il `conn_id` della connessione. Il legame
che vale e' la riga: il `conn_id` lo da' il client e riparte a ogni riavvio
del PC, quindi PC e `conn_id` non bastano a dire quale sessione era. Poi
segna la riga (colonne `agente_lasciapassare` e `tecnico_lasciapassare`,
versione 274) con l'agente e il tecnico verificati; una riga INFO lo dice.
Una connessione gia' segnata non si rilega, e una connessione senza
`conn_id` non lega. Un nonce sconosciuto, di un altro PC o gia' usato
lascia un warn con la connessione, il PC e il motivo (mai il nonce); un
errore del database e' un error. La riga si salva come senza il campo, e al
PC va la risposta di sempre. `scade` qui non conta: il PC verifica il
lasciapassare quando apre la sessione, ma il nonce puo' arrivare all'audit
anche minuti dopo, al clic del cliente.

**Permessi di amministratore.** Non li dice il lasciapassare ma il PC: il
campo facoltativo `amministratore` (booleano) di `POST /api/audit/conn`, col
`new` o con l'autorizzazione (dove arriva la scelta del cliente, per
esempio "accetta come utente"), dice se la shell di quella connessione gira
davvero con i permessi di amministratore (SYSTEM). L'API lo scrive com'e'
arrivato nella colonna `amministratore`, anche senza nonce: un tecnico
che usa il terminale da amministratore con le credenziali di Windows e'
amministratore senza essere un agente. Senza i due campi non cambia
niente.

Nel pannello, nel registro delle connessioni, l'etichetta "Agente
verificato" (con agente e tecnico) c'e' solo col nonce legato, e
"Amministratore" solo quando il PC l'ha detto: sono indipendenti. Gli
eventi `connessione.aperta` e `connessione.chiusa` portano
`sessione.amministratore` e, per una sessione verificata, `agente` con
`amministratore` preso dalla riga.

## Origine e licenza

Remotek API e' basato su [rustdesk-api](https://github.com/lejianwen/rustdesk-api)
di lejianwen, versione v2.7, rilasciato con licenza MIT. Anche questo fork e'
MIT: [LICENSE](LICENSE) resta intatto, con l'attribuzione a lejianwen. Il
pannello e' [rustdesk-api-web](https://github.com/lejianwen/rustdesk-api-web),
dello stesso autore, anch'esso MIT, compilato dal sorgente in
[`pannello/`](pannello/README.md), **modificato**: opera derivata con la
stessa licenza; il suo `LICENSE` e' nell'immagine in
`resources/admin/LICENSE`. Le modifiche rispetto a v2.7, pannello
compreso, sono elencate in [REMOTEK.md](REMOTEK.md).
