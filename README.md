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
- **Log di accesso**: ogni login, dal client e dal pannello.
- **Pannello di amministrazione** su `/_admin/`: [rustdesk-api-web](https://github.com/lejianwen/rustdesk-api-web)
  compilato nell'immagine a un commit fissato, con il marchio di Remotek.
  Utenti, dispositivi, rubriche, tag, gruppi, OAuth, registri.
- **Login**: con password; con un provider **OIDC** generico, configurato dal
  pannello; con **LDAP** (upstream lo dichiara provato con OpenLDAP e Active Directory;
  in Remotek non ancora),
  configurato da file o variabili. Il login GitHub e Google e' in dismissione
  (decisione A3): non va configurato su installazioni nuove.
- **Lingue**: italiano (predefinito) e inglese.
- **Marchio configurabile**: nome, logo e favicon senza toccare il codice.

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
`rustdeskapi.db` e `admin-password.txt`. Il log va su stdout
(`docker compose logs`) e in `/app/runtime/log.txt`, dentro il container.

### Costruire l'immagine

```bash
docker build \
  --build-arg VERSION=<versione> \
  --build-arg REVISION="$(git rev-parse HEAD)" \
  -t remotek-api .
```

Tre stadi: binario Go statico (CGO per SQLite), pannello rustdesk-api-web al
commit fissato, immagine finale Alpine. Le immagini di base sono fissate per
digest. Argomenti:

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
`go generate -tags tools ./tools` (serve `swag` nel `PATH`).

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
| `app.web-sso` \* | `RUSTDESK_API_APP_WEB_SSO` | `false` | offre al client il login confermato dal pannello (`webauth`) |
| `app.disable-pwd-login` | `RUSTDESK_API_APP_DISABLE_PWD_LOGIN` | `false` | `true` toglie il login con password, restano OIDC e LDAP |
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
| `gorm.max-idle-conns` | `RUSTDESK_API_GORM_MAX_IDLE_CONNS` | `10` | connessioni inattive tenute aperte |
| `gorm.max-open-conns` | `RUSTDESK_API_GORM_MAX_OPEN_CONNS` | `100` | connessioni aperte al massimo |
| `rustdesk.id-server` | `RUSTDESK_API_RUSTDESK_ID_SERVER` | indirizzo d'esempio | `host:21116` di `hbbs`, da impostare; lo mostra il pannello |
| `rustdesk.relay-server` | `RUSTDESK_API_RUSTDESK_RELAY_SERVER` | indirizzo d'esempio | `host:21117` di `hbbr`, da impostare |
| `rustdesk.api-server` | `RUSTDESK_API_RUSTDESK_API_SERVER` | `http://127.0.0.1:21114` | indirizzo di questa API come lo vedono client e browser; da' la callback OIDC `<api-server>/api/oidc/callback` |
| `rustdesk.key` | `RUSTDESK_API_RUSTDESK_KEY` | vuoto | chiave pubblica di `hbbs`; vuota = si legge `rustdesk.key-file` |
| `rustdesk.key-file` | `RUSTDESK_API_RUSTDESK_KEY_FILE` | `/data/id_ed25519.pub` | file della chiave; se non si legge, la chiave resta vuota senza errore |
| `rustdesk.personal` | `RUSTDESK_API_RUSTDESK_PERSONAL` | `1` | `1` rubrica personale attiva, `0` spenta |
| `logger.path` | `RUSTDESK_API_LOGGER_PATH` | `./runtime/log.txt` | file di log (0600), oltre a stdout; vuoto = solo stdout. Se non si apre, l'avvio si ferma |
| `logger.level` | `RUSTDESK_API_LOGGER_LEVEL` | `info` | `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`; un valore non valido vale `debug` |
| `logger.report-caller` | `RUSTDESK_API_LOGGER_REPORT_CALLER` | `true` | file e riga del codice in ogni riga di log |
| `proxy.enable` | `RUSTDESK_API_PROXY_ENABLE` | `false` | proxy HTTP per le richieste dell'API al provider OAuth/OIDC |
| `proxy.host` | `RUSTDESK_API_PROXY_HOST` | `http://127.0.0.1:1080` | indirizzo del proxy |
| `jwt.key` | `RUSTDESK_API_JWT_KEY` | vuoto | vuota: token di sessione casuali (16 byte, esadecimale); impostata: token JWT firmati con questa chiave. Col server ufficiale lasciala vuota |
| `jwt.expire-duration` | `RUSTDESK_API_JWT_EXPIRE_DURATION` | `168h` | durata dei JWT |
| `ldap.enable` | `RUSTDESK_API_LDAP_ENABLE` | `false` | login LDAP; se LDAP rifiuta o non risponde, si prova l'utente locale |
| `ldap.url` | `RUSTDESK_API_LDAP_URL` | `ldap://ldap.example.com:389` | `ldap://` o `ldaps://` |
| `ldap.tls-ca-file` | `RUSTDESK_API_LDAP_TLS_CA_FILE` | vuoto | CA del server, con `ldaps://` |
| `ldap.tls-verify` | `RUSTDESK_API_LDAP_TLS_VERIFY` | `false` | con `ldaps://`, `false` non verifica il certificato: impostalo a `true` |
| `ldap.base-dn` | `RUSTDESK_API_LDAP_BASE_DN` | `dc=example,dc=com` | DN di base |
| `ldap.bind-dn` | `RUSTDESK_API_LDAP_BIND_DN` | `cn=admin,dc=example,dc=com` | utente di servizio per le ricerche |
| `ldap.bind-password` | `RUSTDESK_API_LDAP_BIND_PASSWORD` | valore d'esempio | password dell'utente di servizio: dalla variabile, non nel file |
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
l'`Issuer`, `Scopes` di default `openid,profile,email`, URL di callback
`<rustdesk.api-server>/api/oidc/callback`.

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
./apimain -h                                # aiuto
```

La password deve avere da 15 a 32 caratteri (caratteri, non byte), come ogni
password nuova impostata dal pannello o in registrazione. Se la password e'
rifiutata, l'utente non c'e' o l'aggiornamento non riesce, il comando esce
con codice diverso da 0.

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
- **LDAP**: con `ldaps://` imposta `ldap.tls-verify: true`; il default `false`
  non verifica il certificato.
- Le vulnerabilita' si segnalano come dice [SECURITY.md](SECURITY.md), non
  con issue pubbliche.

## Origine e licenza

Remotek API e' basato su [rustdesk-api](https://github.com/lejianwen/rustdesk-api)
di lejianwen, versione v2.7, rilasciato con licenza MIT. Anche questo fork e'
MIT: [LICENSE](LICENSE) resta intatto, con l'attribuzione a lejianwen. Il
pannello e' [rustdesk-api-web](https://github.com/lejianwen/rustdesk-api-web),
dello stesso autore, compilato dal sorgente. Le modifiche rispetto a v2.7
sono elencate in [REMOTEK.md](REMOTEK.md).
