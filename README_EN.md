# Remotek API

*English. Italiano: [README.md](README.md).*

## What it is

The API of **Remotek**, Infinitek's remote support service: the part that
keeps users, address books, devices and logs, next to the ID and relay
servers (`hbbs` and `hbbr` from the [official RustDesk server](https://github.com/rustdesk/rustdesk-server),
which are not in this repository). It is compatible with the RustDesk 1.4.9
client and with the Remotek client.

It is a fork of [rustdesk-api](https://github.com/lejianwen/rustdesk-api) v2.7
by lejianwen (MIT). Upstream is unmaintained: this fork is the project. What
changed since v2.7 is in [REMOTEK.md](REMOTEK.md), news in
[CHANGELOG.md](CHANGELOG.md).

## What it does today

- **Address book** for the client: personal address book and address books
  shared between users, with tags and sharing rules.
- **Groups** of users (regular and shared) and device groups.
- **Devices**: clients send their system information, the panel shows it.
- **Audit**: log of connections and file transfers sent by the clients.
- **Session notes**: the technician annotates the connection and the note
  is saved in the connection log. It can be read in the JSON of
  `/api/admin/audit_conn/list`, in the "Remark" column of the panel's
  connection log and in its CSV export. During the session the note is
  written from the technician's client; the end-of-connection note needs
  the client option `allow-ask-for-note` (off by default) on the
  technician's PC, and the login. A note only attaches to a connection
  already registered, at most 2000 characters (longer ones are cut), and
  its text never goes to the log.
- **Terminal transcripts**: the controlled PC sends the API what goes
  through terminal sessions, in hash-chained blocks; only administrators
  read and verify them (format below).
- **Login log**: every login, from the client and from the panel.
- **Admin panel** on `/_admin/`: [rustdesk-api-web](https://github.com/lejianwen/rustdesk-api-web)
  built into the image from the source in [`pannello/`](pannello/README.md),
  modified, with the Remotek brand. Users, devices, address books, tags,
  groups, OAuth, logs.
- **Login**: with a password; with a generic **OIDC** provider, configured
  from the panel; with **LDAP** (upstream reports it tested with OpenLDAP and Active Directory;
  not yet tested in Remotek),
  configured by file or variables. Google can still be used as a generic OIDC
  provider: in the panel pick type `OIDC`, IdP (the op) e.g. `google`, Issuer
  `https://accounts.google.com`. The GitHub, Google and LinuxDo types still in
  the panel menu are no longer supported (decision A3).
- **Languages**: Italian (default) and English.
- **Configurable brand**: name, logo and favicon without touching the code.

## Terminal session transcripts

Every terminal session (type 4 in the connection log) is transcribed by the
controlled PC: one copy stays on the PC, one is sent to the API with
`POST /api/audit/terminal`, without login, one block per request. This is
the format the client follows. JSON body:

| Field | Type | Value |
|---|---|---|
| `id` | string | ID of the controlled PC |
| `uuid` | string | uuid of the PC, as in `/api/sysinfo` and `/api/audit/conn` |
| `conn_id` | int64 | the same `conn_id` as the session's `new` audit |
| `seq` | integer | 1 for the first block, +1 for each block, at most 100000 |
| `dir` | string | `in` (from the controlling side) or `out` (from the shell) |
| `data` | string | standard base64 (with `=`) of the bytes, at most 64 KiB decoded; empty only with `fine` |
| `hash` | string | lowercase hex of SHA-256( H(seq-1) ‖ d ‖ data ) |
| `fine` | bool | `true` on the last block of the session |

H(0) is 32 zero bytes, H(n) the 32 raw bytes (not the hex) of block n's
hash; d is the byte `i` (0x69) for `in` and `o` (0x6f) for `out`; data is
the decoded bytes.

Each block is tied to the session's row in the connection log: the most
recent one of the PC with that `conn_id`. The client restarts `conn_id`
from a random value every time its service starts, so after a restart the
same number can come back: the new session gets its own row and its own
transcript, from `seq` 1, and the old one stays as it was.

The API checks, in this order: `id` and `uuid` belong to the registered PC;
that row exists and is type 4 (send the block after the audit that
authenticates the connection); `dir`; `data` is base64 and within 64 KiB;
`data` is not empty unless `fine` is `true` (an empty block closes the
session with no trailing data); `seq` is at most 100000, at most 100000
blocks per session; `seq` equals the last saved one for that row plus one;
`hash`; no block after the one with `fine`; at most 20 MiB of data per
session. A block that fails is not saved and the log gets a warn with
route, PC ID and reason, without uuid or content. The response is always
the one of the other audit routes,
`{"code":0,"message":"success","data":""}` (400 for a body that is not
JSON, 413 for a body over 1 MiB, see Security): the client ignores it, and
a rejected block is not resent.

From the admin panel, administrators only, with `audit_conn_id` in the
query (the id of the row in the connection log): `GET /api/admin/audit_conn/terminal/list` lists the blocks by
`seq`, with `data` in base64 (paginated with `page` and `page_size`, 10 per
page by default); `GET /api/admin/audit_conn/terminal/verify` recomputes
the chain from the database and returns `integra`, `blocchi`, `fine`,
`hash` (the stored hash of the last block, to compare with the PC's copy)
and `primo_errato`, the first `seq` that does not match (0 if intact).
Deleting a terminal connection from the panel also deletes its transcript.

`GET /api/admin/audit_conn/terminal/export`, with the same `audit_conn_id`
and administrators only, returns a JSON file to download,
`trascrizione-<audit_conn_id>.json`, with in this order:

| Field | Content |
|---|---|
| `connessione` | the row of the connection log, with the fields of the panel's list |
| `blocchi` | the blocks by `seq`: `seq`, `dir`, `data` (base64), `hash`, `fine`, `created_at` |
| `verifica` | `integra`, `blocchi`, `fine`, `hash`, `primo_errato`, as `verify`, on the same blocks as the file |

The blocks are read in pages of 16, each once, and written to the file as
they are read; the check is computed on the same blocks, so with a session
still running file and check stay consistent with each other. While the
file reaches the downloader the database stays free for other requests. An
id that does not exist or is not type 4 returns the panel's error, with no
file; the log keeps an info line with the administrator's id and
`audit_conn_id`, without content. To recompute the chain from the file,
outside the API: start from H(0), 32 zero bytes, and for each block in
order compute SHA-256( H(seq-1) ‖ d ‖ data ) with `data` decoded from
base64 and d the byte `i` or `o`, as above; each hash must equal the
block's `hash` field, the `seq` values must run from 1 without gaps, and no
block may follow the one with `fine`. Compare the last hash with the copy
on the PC.

## Installation

Each release (tag `api-vX.Y.Z`) publishes the image built from the
`Dockerfile` as `ghcr.io/labinfinitek/remotek-api:X.Y.Z` (and `:X.Y`), with
a provenance attestation, SBOM and licence inventory in the
[release](https://github.com/labinfinitek/remotek-api/releases). Verify it
with `gh attestation verify oci://ghcr.io/labinfinitek/remotek-api:X.Y.Z --owner labinfinitek`.
The repository's `docker-compose.yaml` builds the image from source instead.

### With docker compose

`docker-compose.yaml` builds the image from source as
`ghcr.io/labinfinitek/remotek-api:dev` and runs it on port 21114.

1. In the file, replace the `<...>` placeholders of the four
   `RUSTDESK_API_RUSTDESK_*` variables: addresses of `hbbs` and `hbbr`, the
   address clients use to reach this API, the public key of `hbbs` (the
   content of `id_ed25519.pub`). The time zone is `TZ=Europe/Rome`.
2. Create the data directory, which must belong to `10001:10001`, the user
   the API runs as; otherwise the API does not start and says why:

   ```bash
   mkdir -p remotek-data && sudo chown 10001:10001 remotek-data
   docker compose up -d --build
   ```

3. The panel is at `http://<host>:21114/_admin/`: see [First start](#first-start-and-administration).

`remotek-data/` (`/app/data` in the container) holds the SQLite database
`rustdeskapi.db` and `admin-password.txt`; while the API runs,
`rustdeskapi.db-wal` and `rustdeskapi.db-shm` (SQLite in WAL mode) sit next
to the database, and on a clean stop (SIGTERM, `docker compose stop`) they
are folded back into it and removed. The log goes to stdout only
(`docker compose logs`), as JSON, one line per event:

```json
{"time":"2026-09-29T13:33:09.69Z","level":"WARN","source":{"function":"...","file":"...","line":64},"msg":"rustdesk.id-server vuoto: ..."}
```

Every response has the `X-Request-Id` header, a random id the API picks for
the request (an `X-Request-Id` sent by the caller is not used). It appears as
`request_id` in the lines that start with method and route (for example
`POST /api/heartbeat: ...`: errors sent to the client, handler warnings,
panics) and in the `richiesta` line; other lines written while handling the
request do not carry it, for example `Login Fail`, service-layer lines and
gorm's slow or failed SQL queries. At `debug` level there
is also one `richiesta` line per request, with method, route, status and
duration, without IP or query.

Backup: with the API running use `sqlite3 rustdeskapi.db "VACUUM INTO '<file>'"`
(or `sqlite3`'s `.backup`), never a copy of `rustdeskapi.db` alone, which
would miss the writes still in the `-wal`; with the API stopped, after a
clean stop `rustdeskapi.db` is enough, otherwise copy the whole directory. To
restore, with the API stopped, put the backup in place of `rustdeskapi.db`
and delete `-wal` and `-shm` if present.

### Building the image

```bash
docker build \
  --build-arg VERSION=<version> \
  --build-arg REVISION="$(git rev-parse HEAD)" \
  -t remotek-api .
```

Three stages: static Go binary (CGO for SQLite), panel build from the
source in `pannello/`, final Alpine image. The build downloads no source
from GitHub; base images are pinned by digest. Arguments:

| Argument | Default | Use |
|---|---|---|
| `VERSION` | `dev` | written to `resources/version`, returned by `/api/version` |
| `REVISION` | `unknown` | OCI label `org.opencontainers.image.revision` |
| `BRAND_NAME` | `Remotek` | static title of the panel; letters, digits, spaces and `. _ -` |

The `HEALTHCHECK` calls `http://127.0.0.1:21114/api/version`: if you change
`gin.api-addr`, override it with `--health-cmd`.

### Development

You need Go 1.26 (`go.mod`) and a C compiler, because the SQLite driver uses
CGO. From the repository root:

```bash
go build -o apimain ./cmd   # without -o it fails: cmd is already a directory
./apimain                   # reads ./conf/config.yaml, or -c <file>
go test ./...
```

The API looks for `conf/` and `resources/` in the current directory: from
another directory use `-c <path>/conf/config.yaml` and
`RUSTDESK_API_GIN_RESOURCES_PATH=<path>/resources`. The panel is not in the
repository (the `Dockerfile` builds it into `resources/admin/`): without it,
`/_admin/` has no pages. The swagger docs are regenerated with
`go generate -tags tools ./tools` (needs `swag` v1.16.3, the `go.mod`
version, in `PATH`).

## Configuration

The configuration lives in `conf/config.yaml` (`/app/conf/config.yaml` in the
image) and every key can be overridden by an environment variable: prefix
`RUSTDESK_API_`, then the key in upper case with `.` and `-` replaced by `_`
(`app.captcha-threshold` becomes `RUSTDESK_API_APP_CAPTCHA_THRESHOLD`). The
variable beats the file, the file beats the code default. A variable only
works for keys that are in the file or have a code default (those marked \*):
the `conf/config.yaml` of the repository and of the image has them all, a
shorter file of your own does not.

The default is the value in `conf/config.yaml`; \* means the code has the same
default when the key is missing from the file.

| Key | Variable | Default | Values and notes |
|---|---|---|---|
| `lang` \* | `RUSTDESK_API_LANG` | `it` | `it`, `en`, empty (= English); any other value stops startup. Language of the answers when `Accept-Language` does not pick one (the RustDesk client does not send it) |
| `brand.name` \* | `RUSTDESK_API_BRAND_NAME` | `Remotek` | product name; empty = `Remotek` |
| `brand.dir` \* | `RUSTDESK_API_BRAND_DIR` | `./resources/brand` | directory of `logo.svg` and `favicon.svg`, served on `/brand/` |
| `app.register` \* | `RUSTDESK_API_APP_REGISTER` | `false` | open registration of new users |
| `app.register-status` | `RUSTDESK_API_APP_REGISTER_STATUS` | `1` | status of new registrations: `1` enabled, `2` disabled |
| `app.captcha-threshold` \* | `RUSTDESK_API_APP_CAPTCHA_THRESHOLD` | `3` | failed logins from one IP, within 10 minutes, after which the captcha is required; `0` always, negative never |
| `app.ban-threshold` \* | `RUSTDESK_API_APP_BAN_THRESHOLD` | `10` | failed logins from one IP, within 10 minutes, after which every request from it is refused for 30 minutes; `0` never |
| `app.show-swagger` \* | `RUSTDESK_API_APP_SHOW_SWAGGER` | `0` | `1` publishes `/swagger/index.html` and `/admin/swagger/index.html` |
| `app.token-expire` | `RUSTDESK_API_APP_TOKEN_EXPIRE` | `168h` | session lifetime (Go duration: `72h`, `30m`) |
| `app.web-sso` \* | `RUSTDESK_API_APP_WEB_SSO` | `false` | turns on the client login confirmed from the panel (`webauth`); off, `/api/oidc/auth` rejects it like a provider that does not exist |
| `app.disable-pwd-login` | `RUSTDESK_API_APP_DISABLE_PWD_LOGIN` | `false` | `true` removes username and password login, LDAP included, since it goes through it; OIDC remains. The exceptions remain: AI agents from the client and `app.pwd-login-eccezioni` (Password login off) |
| `app.pwd-login-eccezioni` \* | `RUSTDESK_API_APP_PWD_LOGIN_ECCEZIONI` | empty | comma-separated usernames (spaces trimmed) that log in with the password even with `app.disable-pwd-login`, from the client and the panel; empty = nobody. Ignored while `app.disable-pwd-login` is off |
| `app.firma-obbligatoria` \* | `RUSTDESK_API_APP_FIRMA_OBBLIGATORIA` | `false` | `true`: PCs without a key must sign their no-login requests too (Device signature) |
| `admin.title` \* | `RUSTDESK_API_ADMIN_TITLE` | empty | panel title; empty = `brand.name` |
| `admin.hello` | `RUSTDESK_API_ADMIN_HELLO` | empty | welcome message; the panel no longer shows it, the key stays for now. When not empty, `admin.hello-file` is not read |
| `admin.hello-file` | `RUSTDESK_API_ADMIN_HELLO_FILE` | `./conf/admin/hello.html` | welcome file (`{{username}}` and `{{brand}}` are replaced); the panel no longer shows it, the key stays for now |
| `gin.api-addr` | `RUSTDESK_API_GIN_API_ADDR` | `0.0.0.0:21114` | listen address |
| `gin.mode` | `RUSTDESK_API_GIN_MODE` | `release` | `release`, `debug`, `test` |
| `gin.resources-path` | `RUSTDESK_API_GIN_RESOURCES_PATH` | `resources` | directory of panel, languages and templates; without the language files startup stops |
| `gin.trust-proxy` \* | `RUSTDESK_API_GIN_TRUST_PROXY` | empty | trusted proxy IPs or CIDRs, comma separated; empty = none, `X-Forwarded-For` and `X-Real-IP` ignored. Behind a reverse proxy it must be set, otherwise captcha and ban count every client as the proxy IP. An invalid value stops startup |
| `gorm.type` | `RUSTDESK_API_GORM_TYPE` | `sqlite` | only `sqlite` (empty is the same); any other value stops startup. The database is `data/rustdeskapi.db` |
| `rustdesk.id-server` | `RUSTDESK_API_RUSTDESK_ID_SERVER` | empty | `host:21116` of `hbbs`, to be set; shown by the panel. Empty: a warning at startup; a RustDesk client without an ID server uses RustDesk's public servers |
| `rustdesk.relay-server` | `RUSTDESK_API_RUSTDESK_RELAY_SERVER` | empty | `host:21117` of `hbbr`, to be set; shown by the panel |
| `rustdesk.api-server` | `RUSTDESK_API_RUSTDESK_API_SERVER` | empty | address of this API as clients and browsers see it, to be set; shown by the panel and gives the OIDC callback `<api-server>/api/oidc/callback`. Empty: a warning at startup, and OIDC login and `webauth` send to addresses without a host |
| `rustdesk.key` | `RUSTDESK_API_RUSTDESK_KEY` | empty | public key of `hbbs`; empty = `rustdesk.key-file` is read |
| `rustdesk.key-file` | `RUSTDESK_API_RUSTDESK_KEY_FILE` | `/data/id_ed25519.pub` | key file; if it cannot be read, the key stays empty without an error |
| `rustdesk.personal` | `RUSTDESK_API_RUSTDESK_PERSONAL` | `1` | `1` personal address book on, `0` off |
| `integrazione.url` \* | `RUSTDESK_API_INTEGRAZIONE_URL` | empty | `https://` URL that receives the session events (Integration); empty = off, no events. No credentials in the URL; `http://` or credentials stop the start-up. Configuration only, never from the panel |
| `integrazione.segreto` \* | `RUSTDESK_API_INTEGRAZIONE_SEGRETO` | empty | event signing secret, at least 32 characters when `integrazione.url` is set (otherwise start-up stops): from the variable, not the file; never logged |
| `agente.lasciapassare-chiave` \* | `RUSTDESK_API_AGENTE_LASCIAPASSARE_CHIAVE` | empty | file with the Ed25519 seed of the AI agents' passes (Agent pass); empty = off. When set, the file must exist, have permissions 600 or stricter and a valid key, otherwise startup stops |
| `logger.path` \* | `RUSTDESK_API_LOGGER_PATH` | empty | ignored: the log goes to stdout only; if set, a warn at startup says so |
| `logger.level` \* | `RUSTDESK_API_LOGGER_LEVEL` | `info` | `debug`, `info`, `warn`, `error`; any other value stops startup |
| `logger.report-caller` | `RUSTDESK_API_LOGGER_REPORT_CALLER` | `true` | `source` (function, file and line of the code) in every log line |
| `proxy.enable` | `RUSTDESK_API_PROXY_ENABLE` | `false` | HTTP proxy for the API's requests to the OAuth/OIDC provider |
| `proxy.host` | `RUSTDESK_API_PROXY_HOST` | `http://127.0.0.1:1080` | proxy address |
| `jwt.key` | `RUSTDESK_API_JWT_KEY` | empty | empty: random session tokens (16 bytes, hex); set: JWT tokens signed with this key. With the official server leave it empty |
| `jwt.expire-duration` | `RUSTDESK_API_JWT_EXPIRE_DURATION` | `168h` | JWT lifetime |
| `ldap.enable` | `RUSTDESK_API_LDAP_ENABLE` | `false` | LDAP login; if LDAP refuses or does not answer, the local user is tried |
| `ldap.url` | `RUSTDESK_API_LDAP_URL` | `ldap://ldap.example.com:389` | `ldap://` or `ldaps://` |
| `ldap.tls-ca-file` | `RUSTDESK_API_LDAP_TLS_CA_FILE` | empty | server CA, with `ldaps://` |
| `ldap.tls-verify` | `RUSTDESK_API_LDAP_TLS_VERIFY` | `true` | with `ldaps://` the server certificate is verified; for an internal certificate use `ldap.tls-ca-file`. `false` does not verify it, and a warning at startup says so |
| `ldap.base-dn` | `RUSTDESK_API_LDAP_BASE_DN` | `dc=example,dc=com` | base DN |
| `ldap.bind-dn` | `RUSTDESK_API_LDAP_BIND_DN` | `cn=admin,dc=example,dc=com` | service user for searches |
| `ldap.bind-password` | `RUSTDESK_API_LDAP_BIND_PASSWORD` | empty | password of the service user: from the variable, not in the file |
| `ldap.user.base-dn` | `RUSTDESK_API_LDAP_USER_BASE_DN` | `ou=users,dc=example,dc=com` | where to search for users |
| `ldap.user.filter` | `RUSTDESK_API_LDAP_USER_FILTER` | `(cn=*)` | filter added to the search |
| `ldap.user.username` | `RUSTDESK_API_LDAP_USER_USERNAME` | `uid` | username attribute (`sAMAccountName` in AD) |
| `ldap.user.email` | `RUSTDESK_API_LDAP_USER_EMAIL` | `mail` | email attribute |
| `ldap.user.first-name` | `RUSTDESK_API_LDAP_USER_FIRST_NAME` | `givenName` | first name attribute |
| `ldap.user.last-name` | `RUSTDESK_API_LDAP_USER_LAST_NAME` | `sn` | last name attribute |
| `ldap.user.enable-attr` | `RUSTDESK_API_LDAP_USER_ENABLE_ATTR` | empty | attribute that says whether the user is enabled (`userAccountControl` in AD); empty = all enabled |
| `ldap.user.enable-attr-value` | `RUSTDESK_API_LDAP_USER_ENABLE_ATTR_VALUE` | empty | value of `enable-attr` for an enabled user (ignored in AD) |
| `ldap.user.sync` | `RUSTDESK_API_LDAP_USER_SYNC` | `false` | `true` updates the local user at every login, `false` only when it is created |
| `ldap.user.admin-group` | `RUSTDESK_API_LDAP_USER_ADMIN_GROUP` | `cn=admin,dc=example,dc=com` | DN of the administrators' group |
| `ldap.user.allow-group` | `RUSTDESK_API_LDAP_USER_ALLOW_GROUP` | `cn=users,dc=example,dc=com` | DN of the group allowed to log in; empty = everyone |

The OIDC provider is configured from the panel (OAuth, type `oidc`): it needs
the `Issuer`, `Scopes` default to `openid,profile,email` (`openid` is always
requested, even when it is not in the saved scopes), callback URL
`<rustdesk.api-server>/api/oidc/callback`. The client secret is written
at creation and cannot be read back: the provider list and detail only say
whether it is set (`has_client_secret`); when editing, left empty, the
saved one stays, and to change it you write the new one. Scopes are
comma-separated; surrounding spaces and empty entries are dropped. The provider must send the
`id_token` in the token response (OIDC Core): without it, or with an
`id_token` that does not verify, the login stops. Auto-registration
(`auto_register`) is off unless you turn it on: a provider account without a
user is sent to link itself from the panel; when on, the user is created at
the first login.

The OIDC login finds the user through the *binding* (`user_thirds`: provider
and the account's `sub`), not through the email. A `sub` is bound to one
user only (unique index on `open_id` and `op`, created at startup; with
duplicates an error with the number of duplicate rows, startup goes on
without the index and retries at the next one), and a user has at most one account per
provider: binding from the panel refuses an account that already belongs to
someone ("This OIDC account is already bound to another user.") and a user
who already has one ("This user already has an account bound to this OIDC
provider."). With `auto_register` on, an account without a binding is
matched to the user with the same email only if the provider sends
`email_verified` true and the user is neither an AI agent nor disabled; if
the email belongs to a user that cannot be matched the login stops ("This
OIDC account cannot be matched to the user with the same email"), if it
belongs to nobody a new user is created. A disabled user cannot log in with
OIDC either: the login answers "User disabled." without a token.

## First start and administration

**Initial password.** On first start the API creates the `admin` user with a
random 20-character password and writes it only to `data/admin-password.txt`
(permissions 0600, next to `rustdeskapi.db`; `/app/data/admin-password.txt`
in the container); the log says where it is, it does not print it. With
docker compose: `sudo cat remotek-data/admin-password.txt`. Log in on
`/_admin/`, change the password from the panel and delete the file.

**Commands.** From the binary (in the container: `docker compose exec remotek-api ./apimain ...`):

```bash
./apimain reset-admin-pwd <password>        # admin password
./apimain reset-pwd <userId> <password>     # password of another user
./apimain agente-ai <username> <technician> # account of an AI agent
./apimain token-integrazione crea <name>    # integration token (Integration)
./apimain token-integrazione elenco
./apimain token-integrazione revoca <name>
./apimain lasciapassare chiave-nuova <file> # key of the agent passes (Agent pass)
./apimain -h                                # help
```

The password must be 15 to 32 characters long (characters, not bytes), like
every new password set from the panel or at registration. When the password
is refused, the user does not exist or the update fails, the command exits
with a non-zero code.

**AI agents.** Every AI agent has its own account, tied to the technician
responsible for it (the user's `agente_di` field: the technician's id, 0 for
a person). `agente-ai <username> <technician's username>` creates it in the
technician's group, never as administrator, and prints its random
24-character password on stdout, once (not in the log); it exits with code 1
if the username already exists or is not 2 to 32 characters long (as in the
panel), or if the technician does not exist, is not a person or is
disabled. The same rules apply from the panel (`/api/admin/user/create`
and `update`, field `agente_di`): the technician is another existing user
and a person, an agent is not an administrator, and a user responsible for
agents does not become an agent. An agent follows its technician: disabling
the technician disables their agents; an agent whose technician is disabled
cannot be enabled; a technician with agents cannot be deleted until their
agents are deleted or assigned to another technician. With `ldap.user.sync`
the sync does not give an agent the role of the `ldap.user.admin-group`
group (a warn says so). An update without `agente_di`, like those of
today's panel, leaves it as it is. The command-line client asks, with the
login token, `GET /api/agente`, which answers
`{"agente":true,"tecnico":"<name>"}` (the technician's nickname, or the
username if the nickname is empty) or `{"agente":false,"tecnico":""}`;
`/api/login` and `/api/currentUser` do not change.

An agent only uses the command-line client (ADR-0028). Among the login
routes only `GET /api/user/info`, `POST /api/currentUser`,
`POST /api/logout`, `GET /api/agente` and `POST /api/agente/lasciapassare`
answer it; the others (address books, PCs, users, notes) answer 403
`{"error": msg}`, with a warn. The panel login (`POST /api/admin/login`, and
the OIDC one) refuses it without a token, and every panel login route
answers code 403, which for the panel is a logout, even to a token issued
before. So an
agent cannot create address books or rules, nor write into shared ones, to
widen the pass boundary. The CLI has no browser nor TOTP: with
`app.disable-pwd-login` on, an agent still logs in with the password on
`POST /api/login`, never on the panel.

**Password login off.** With `app.disable-pwd-login` technicians log in
only with OIDC. Still logging in with the password are the agent accounts,
only on `POST /api/login`, and the usernames in `app.pwd-login-eccezioni`
(for example the acceptance-test user and an emergency administrator), on
`/api/login` and `/api/admin/login`. Everyone else, a username that does
not exist and an exception with the wrong password get the same answer:
"Password login disabled.", without telling who is an exception; every
refusal counts for captcha and ban. A successful login by exception writes
an INFO line `login con password per eccezione` with username and reason
(`agente` or `elenco`), a warn if the user is an administrator.
**Emergency access:** if the OIDC provider does not answer, the panel shows
the password form at `/_admin/#/login?password=1`, even with a single
provider (which without the parameter opens OIDC at once); only a username
in the list gets in.

**Connecting the client.** From client `remotek-1.4.9-3`, Remotek has the
ID server, relay server, API server and key locked in the binary (ADR-0025):
Settings > Network does not change them. For a test server they change only
with a `custom.txt` signed by Infinitek. A generic RustDesk client instead
connects under Settings > Network: ID server, relay server, API server
(`rustdesk.api-server`) and key, the same values as the
`RUSTDESK_API_RUSTDESK_*` variables.

**Changing the brand.** Name, logo and favicon change without touching the
code:

- name: `brand.name` (`RUSTDESK_API_BRAND_NAME`). It is the panel title when
  `admin.title` is empty, `{{brand}}` in the welcome message and the title of
  the OAuth/OIDC login pages; it applies after a restart;
- logo and favicon: `logo.svg` and `favicon.svg` in `brand.dir`, served on
  `/brand/logo.svg` and `/brand/favicon.svg`. Replace the files, no rebuild
  needed; in the container mount them, readable by 10001:
  `-v /srv/remotek/brand:/app/resources/brand:ro`. An SVG must be
  self-contained: no scripts, nothing from other sites;
- the title the panel shows before it loads its configuration is set at
  build time: `docker build --build-arg BRAND_NAME=<name> ...`.

**Panel branding.** What belongs to whoever installs it is not in the panel
code: the panel reads it, if present, from `brand.json` in `brand.dir`, served
on `/brand/brand.json` next to the logo. The file is optional and so is every
field: without it the panel shows no company name and no privacy notice, the
login button says "Sign in with <OIDC provider>" with the usual icon and the
dark theme uses `logo.svg`. Format (all strings):

```json
{
  "ragione_sociale": "Example Ltd",
  "informativa": "https://example.com/privacy",
  "nome_accesso": "Example",
  "logo_scuro": "logo-dark.svg",
  "icona_accesso": "mark.svg"
}
```

- `ragione_sociale` (company name, at most 120 characters): at the bottom of
  the login page and in the panel footer, after the product name;
- `informativa` (privacy notice): link at the bottom of the login page; only
  `https://` addresses, others are ignored;
- `nome_accesso` (at most 60 characters) and `icona_accesso`: name and icon of
  the OIDC login button ("Sign in with <nome_accesso>"); they apply only with
  one OIDC provider, with more providers each keeps its own name;
- `logo_scuro`: logo for the top bar and the login page in the dark theme
  (navy background).

`logo_scuro` and `icona_accesso` are file names in the same directory
(`.svg`, `.png` or `.webp`, no paths), with the same rules as `logo.svg`. An
invalid field counts as missing; the panel reads the file again every time it
opens.

Not renamed: the `RUSTDESK_API_` prefix, the `rustdesk:` section, the Go
module path.

## Security

- **Non-root process**: in the image the API runs as `remotek`
  (10001:10001). A data directory that 10001 cannot write stops startup with a
  message that gives the fix (`chown -R 10001:10001`).
- **Secure defaults**, also in the code when missing from the file:
  registration off, login from the panel (`web-sso`) off, swagger off, captcha
  after 3 failed logins and ban after 10, no trusted proxy.
- **Credentials**: initial password only in the 0600 file, never in the log;
  new passwords of 15-32 characters; without `jwt.key`, random session tokens.
- **Log** 0600: it contains usernames and IP addresses.
- **Request body**: at most 1 MiB, on every route, including those without
  login. With a `Content-Length` over the limit the API answers 413
  `{"error": ...}` (under `/api/admin/` `{"code":101,"message":...}`, like
  the rest of the panel) without reading the body; without `Content-Length` it
  reads at most 1 MiB and beyond that answers as for a malformed body. The
  log gets a warn with method, route and `request_id`, without the body.
  The largest real body, a terminal transcript block, is about 88 KB.
- **No external resources** in the pages the API generates (OAuth/OIDC login
  result). Files under `/brand/` are served with a `Content-Security-Policy`
  that runs no scripts and with `X-Content-Type-Options: nosniff`.
- **LDAP**: with `ldaps://` the server certificate is verified
  (`ldap.tls-verify` is `true`); for an internal certificate use
  `ldap.tls-ca-file`. At startup a warning flags `ldap://`, which sends
  passwords in clear text, and `ldap.tls-verify: false`.
- **Devices**: sysinfo, heartbeat and audit come from the client without
  login; the API accepts them only from the registered device. The first
  sysinfo of an ID binds the ID to the PC's uuid (on Windows the
  MachineGuid); a PC created from the panel binds to the first uuid that
  arrives. From then on, with another uuid, sysinfo answers "The device
  does not match the registered one." and heartbeat and audit write
  nothing; the log keeps a warning with the route and the PC's ID. If the
  PC really changes uuid (Windows reinstalled, hardware replaced) the
  binding is reopened by deleting the PC from the panel while it is switched
  on: for example when the log shows the warning with its ID, which the
  heartbeat sends every 15 seconds. With the PC switched on and the Remotek
  service running, the refused sysinfo is retried on its own about every 2
  minutes and the first attempt after the deletion creates the PC again
  with the new uuid; until then (at most about 2 minutes) the ID goes to
  whoever first sends a sysinfo with it. With the PC switched off or the
  service stopped nothing retries, and the ID stays free until the PC is
  back online. Restarting the Remotek service on the PC (or rebooting it)
  right away closes the window sooner. A PC deleted while bound to its
  current uuid (by mistake, for example) must be restarted right away (the
  Remotek service or the PC): the client sends sysinfo again only when the
  service starts or the Windows user, the ID or the API address changes,
  and until the restart the ID stays free with no time limit. A signing
  client adds the PC key: see Device signature.
- Report vulnerabilities as described in [SECURITY.md](SECURITY.md), not with
  public issues.

## Device signature

The uuid that binds the ID to the PC (Security, Devices) is also sent by the
client to `hbbs` in clear text, over UDP: anyone watching the network can read
it and write on behalf of the PC. So the PC signs its requests that need no
login (sysinfo, heartbeat, audit/conn, audit/file, audit/terminal) with its
Ed25519 key, and once the key is registered the API accepts only signed
requests for that PC. Client 1.4.9 does not sign and keeps the uuid rules
until it is updated.

**Format** (this is the contract with the client). Header:

```
X-Remotek-Firma: <ts>.<firma>
```

- `ts`: UNIX seconds in decimal (digits only), the PC's time when signing;
- `firma`: standard base64, with `=`, of the 64 bytes of the "detached"
  Ed25519 signature of the message.

The message is five lines joined by `\n`, with no trailing `\n`:

```
remotek-api-v1
<method in upper case, for example POST>
<request path, without query, for example /api/heartbeat>
<ts, the same as in the header, character for character>
<SHA-256 of the body bytes, lower-case hex>
```

The body is what the client sends, byte for byte: the signature is checked on
the bytes received, before the JSON. The path is the one that reaches the API
(behind a reverse proxy that strips a prefix, the one without the prefix).
sysinfo also carries the `pk` field: standard base64 of the 32 bytes of the
PC's Ed25519 public key.

**Test vector** (in `http/firma_test.go`, `TestFirmaVettore`): with the key
from the 32-byte seed `00 01 02 ... 1f` and the request

- method `POST`, path `/api/heartbeat`, ts `1791000000`,
- body `{"id":"999000111","uuid":"dXVpZA=="}`

the body SHA-256 is
`8ce852e69dbc62f20c29f8fcfb77e1a1922d73c7a185de173bb31139f4089d10`, `pk` is
`A6EHv/POEL4dcN0Y50vAmWfk1jCbpQ1fHdyGZBJVMbg=` and the header is

```
X-Remotek-Firma: 1791000000.otUjpW4BdAQnl0TuN1E/GmD3LAx38zzpYTczckv70y3hYSIVtkA7urCTqftTR9BXPMoDc//+nyxItL73nbrHCw==
```

**Rules**:

1. A PC with a registered key: the request must carry the header, a valid
   signature with that key, a `ts` within 300 seconds of the server's time,
   and the same signature must not have been accepted in the previous 300
   seconds. The uuid must still be the saved one, as before. The memory of
   accepted signatures lives in the process: after an API restart the window
   starts again, and a signature accepted just before the restart can be
   replayed while its `ts` is within the 300 seconds.
2. A PC without a key: as before (uuid), with or without a signature.
3. Registration: a sysinfo with `pk` and a valid signature for that `pk`
   saves the key only if the PC is new (it creates it, as before) or already
   has that uuid, and only if the PC has no key yet. A PC created from the
   panel without a uuid takes the uuid with the first sysinfo and the key
   with the next one. No sysinfo changes or deletes a saved key; it is
   reopened only by deleting the PC from the panel, as for the uuid.
4. A sysinfo with the right uuid but signed with a `pk` different from the
   saved one changes nothing, gets the answer for another device and writes
   an **error** to the log with route and PC ID. It means a reinstalled PC
   (new key), or someone who knew the uuid and registered their key before
   the real PC. Remedy: delete the PC from the panel and check who registers
   (name, user and IP of the new record); if it is not the real PC, delete it
   again and restart the Remotek service on the real PC.
5. A rejected request gets the same answer as before for that route
   (sysinfo: "The device does not match the registered one."; heartbeat:
   `{}`; audit: success without saving) and a warn in the log with route, PC
   ID and reason (`firma assente`, `firma non valida`, `firma fuori tempo`,
   `firma ripetuta`, `firma di un PC senza chiave registrata`), never the
   signature, key or uuid. An accepted sysinfo of a PC without a key, with a
   `pk` that is not registered, says so with a warn `chiave non registrata:
   <reason>`; with a uuid different from the saved one and the signature
   not mandatory, the signature is not even checked.
6. A registered key, of a new PC or of one that had none, writes an **info**
   line `chiave del PC registrata` to the log with route and PC ID, once;
   never the key, uuid or signature.

**Unknown fields.** The device routes (sysinfo, heartbeat, audit/conn,
audit/file, audit/terminal) ignore the body fields they do not know: a
client newer than the API can send more of them and the answer does not
change. The client from `remotek-1.4.9-3` sends `ms` in file audits, which
the API does not save. The signature covers the whole body, ignored fields
included.

**Option** `app.firma-obbligatoria` (`RUSTDESK_API_APP_FIRMA_OBBLIGATORIA`,
default `false`): when true, a PC without a key must sign too. Its sysinfo
signed with `pk` and the right uuid registers the key (a new PC is created);
any other request from it is rejected, unsigned (`firma assente`) or with a
signature there is no key to check (`firma di un PC senza chiave
registrata`), until the key is registered. Turn it on when every PC has a
signing client: a 1.4.9 client stops writing.

## Copies of the same PC

A PC copied without sysprep (cloned VM, disk image) carries Remotek's ID,
uuid and key: the copy registers as the real PC and the signature does not
tell them apart, because the key is the same. `hbbs` alternates the two
addresses and the API receives heartbeats and audits of both on the same
row. The API reports it; it blocks no request and the client is unchanged.

**Rule.** For every recognised PC (saved ID and uuid, valid signature where
required) the API remembers the address and time of the last heartbeat; the
address is the same as `last_online_ip`. A *change* is a heartbeat of the
same PC from an address different from the previous one, arriving less than
60 seconds later: the client sends one every 15 seconds, so two running
copies make a change at almost every heartbeat. With at least **10 changes
in the last 10 minutes** the PC is a "possible copy":

- in the panel's PC list (`GET /api/admin/peer/list`, field
  `possibile_copia`) a red "Possible copy" label appears next to the ID,
  and disappears as soon as fewer than 10 changes are left in the last 10
  minutes: at most 10 minutes after the last change, often earlier;
- the log gets a warn line `possibile copia del PC` with `id`, the two most
  recent addresses (`indirizzo`, `indirizzo_precedente`) and the number of
  `cambi`, at most once an hour per PC; never uuid or keys.

A PC without heartbeats for 10 minutes leaves the memory; the time of its
last warning stays there for an hour, so that a copy silent for a while
gets no second warning before the hour.

**What to do.** On the copy, uninstall Remotek removing its configuration
and install it again: it gets a new ID. For future copies: sysprep before
cloning, or install Remotek after the copy.

**Limits.** It is a warning to check, not a block:

- a network with two lines that balances every connection on a different
  address gives the same sign without any copy;
- two copies behind the same public address are not seen;
- the memory lives in the process: after an API restart it starts empty;
- behind a proxy without `gin.trust-proxy` (Configuration) every PC has the
  proxy's address and the rule sees nothing.

## Integration (events)

Support works in Zammad: the API sends n8n (private network) one signed
event for each fact of a session, and n8n brings what is needed into the
ticket; the technician writes the ticket number in the session note (for
example `#83100`). The API knows nothing about Zammad (ADR-0026).

**Configuration.** `integrazione.url` and `integrazione.segreto`
(Configuration). Without a URL nothing is queued and the API behaves as
before. With a URL, at start-up: it must be `https://`, without
credentials, and the secret must have at least 32 characters, otherwise
start-up stops.

**Delivery.** Events are stored in the `eventi` table (database version
271) within the client's request; if storing fails, a warn `evento ... non
accodato` is logged and the client gets the usual response. A worker in
the API process, started and stopped with it, sends them one at a time in
creation order among those that are due: JSON `POST`, 10 second timeout,
redirects not followed. Delivered = 2xx response. Otherwise it retries
after 30 seconds, then 1, 2, 4... minutes up to one hour between attempts,
for 24 hours from creation; then an ERROR line `evento non consegnato` with
type and id, and the event stays marked `fallito`. Each failed attempt is
a warn, without the URL. Delivered and failed events are deleted after 7
days. The queue lives in the database: after a restart it resumes where it
was. Delivery is *at least once*: the receiver drops duplicates by
`X-Remotek-Evento-Id`; an event being retried does not hold back the
others, so the reliable order is `ora`.

**Types.**

| Type | When |
|---|---|
| `connessione.aperta` | the connection is authorised: the PC sends `POST /api/audit/conn`, without `action`, with technician (`peer`) and type, which the event carries; `inizio` is the time of the `new` sent when the socket opened. A connection never authorised has no such event; an identical repeat makes no second one |
| `connessione.chiusa` | the PC records the close; with `fine` and `durata_s`. Also sent for connections never authorised (refused or not clicked): then `tecnico` is empty and `tipo` is 0 |
| `connessione.nota` | the session note is written or changed (during the session or at the end); a note equal to the stored one makes no event. The text is the one in the log, truncated to 2000 characters |
| `file.trasferiti` | the PC records a file transfer (`POST /api/audit/file`); with `file.direzione` (`dal_pc`: the controlled PC sends the files, log type 0; `al_pc`: it receives them, type 1) and `file.numero`. Names and paths stay in the panel |
| `terminale.chiuso` | the block with `fine` of a terminal session transcript arrives; with `sessione` and `verifica` (`integra`, final `hash`), computed as the panel's verification. No blocks or data. `sessione.fine` is `null` if the PC records the close later |
| `pc.nuovo` | a sysinfo creates a PC the API did not know |
| `pc.possibile_copia` | the API writes the `possibile copia del PC` warn (Copies of the same PC), at most one per hour per PC; with `copia.cambi`, without the addresses |
| `agente.lasciapassare` | the API issues a pass to an AI agent (Agent pass); with `agente.agente` and `agente.tecnico` (the usernames) and `agente.amministratore` (`true`). Never the pass nor the nonce |

**Fields.** Every event has `schema`, `id`, `tipo`, `ora` and `pc`; on top
of these, and nothing else:

- `connessione.aperta`, `connessione.chiusa`: `sessione` (with
  `amministratore`), and `agente` if the session is verified with the pass
  (Agent pass);
- `connessione.nota`: `sessione`;
- `file.trasferiti`: `file`;
- `terminale.chiuso`: `sessione` and `verifica`;
- `pc.nuovo`: no other field;
- `pc.possibile_copia`: `copia`;
- `agente.lasciapassare`: `agente`.

For example:

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

`id` is a random event uuid, `ora` when it was created (RFC 3339, UTC).
`pc.nome` is the hostname. `sessione.tipo` is the log's type, as in the
panel (4 = terminal), `tecnico` the technician's name (`from_name`);
`fine` and `durata_s` are `null` while the connection is open;
`amministratore` (only in aperta and chiusa) says whether the shell has
administrator rights, as the PC said.
The other objects:

```json
"file": {"direzione": "dal_pc", "numero": 3}
"verifica": {"integra": true, "hash": "9f2c...e01a"}
"copia": {"cambi": 10}
"agente": {"agente": "agente-mario", "tecnico": "mario", "amministratore": true}
```

**Signature.** Every request carries `X-Remotek-Evento-Id` (the `id`) and
`X-Remotek-Evento-Firma: t=<UNIX seconds>,v1=<hex>`, with `v1` =
HMAC-SHA256(secret, `"<t>."` + exact body received). The receiver
recomputes `v1` over the body bytes, before decoding the JSON, compares it
in constant time, and rejects a `t` too far from its own clock (for
example over 5 minutes: a retried event has a new `t`). Test vector:
secret `segreto-di-prova-di-almeno-32-caratteri`, `t` = `1791000000`, body
`{"schema":1,"tipo":"prova"}` give
`v1=47977e5349c23bd14a1a9d3200bed968a3092152ca7f73b73c6581a5f0b47013`.

```sh
printf '%s.%s' 1791000000 '{"schema":1,"tipo":"prova"}' \
  | openssl dgst -sha256 -hmac 'segreto-di-prova-di-almeno-32-caratteri'
```

**What never leaves.** uuids, keys, signatures, tokens, guids,
`session_id`, IP addresses, file contents and names, terminal transcripts.

### Integration tokens (read-only)

n8n can ask the API for details with an integration token, which can only
read (ADR-0026).

**Commands** (First start and administration):

- `token-integrazione crea <name>` creates the token and prints it on
  stdout once: 32 random bytes in base64url. The database
  (`token_integrazione`, database version 272) keeps only its SHA-256: if it
  is lost, create another. The name has 2 to 64 characters among letters,
  digits, `.`, `-` and `_`, and is unique. The token expires after one year.
- `token-integrazione elenco` prints name, creation, expiry, revocation and
  last use (RFC 3339, `-` when absent), never tokens or hashes.
- `token-integrazione revoca <name>` revokes it at once. An unknown name
  exits with code 1.

**Routes.** `GET` only, with `Authorization: Bearer <token>` and no other
credential. The integration token is valid on no other route, and a login
token is not valid on these. Without a token, or with an unknown, expired
or revoked one, the answer is 401 `{"error": "Unauthorized"}`; an expired
or revoked token leaves a warn with its name in the log. The token never
goes to the log.

| Route | Response |
|---|---|
| `GET /api/integrazione/pc` | `elenco` of PCs (`id`, `nome` = hostname, `alias`, `ultimo_contatto` RFC 3339 or `null`, `versione`, `possibile_copia`), `totale`, `pagina`, `per_pagina` |
| `GET /api/integrazione/connessioni` | connection log in `id` order, with the fields of the events' `sessione` plus `id` (of the row) and `pc`; filters `pc` (PC ID) and `da` (RFC 3339, connections started since then) |
| `GET /api/integrazione/connessioni/{id}/verifica` | result of the transcript verification of connection `id` (`integra`, `blocchi`, `fine`, `hash`, `primo_errato`), as the panel's verification; 404 if the connection does not exist or is not a terminal |

Both lists are paged: `pagina` (1 to 1000000) and `per_pagina` (1 to
1000, default 100); an invalid value answers 400. `da` is compared as an
instant, whatever the time zone: `2026-10-11T00:00:00Z` or
`2026-10-11T02:00:00%2B02:00`, with the `+` written `%2B` in the query
(otherwise it becomes a space and the answer is 400). Never returned: uuids, keys,
IPs, guids, `session_id`, file names or paths, transcripts. No route
writes anything and none exports transcripts.

## Agent pass (ADR-0028)

The AI agent terminal (`remotek-cli`, with the agent's account: see AI
agents) can ask for administrator rights on a PC: on an attended PC the
customer accepts them in the window, on an unattended PC enabled by its
signed `custom.txt` it gets in without a click. Nobody checks the name the
client declares, so before connecting the CLI asks the API for a signed
*lasciapassare* (pass) for that PC, and the PC verifies it with the API
public key built into its binary. The pass is valid for 60 seconds to OPEN
the session, once, for that PC only.

**Configuration.** `agente.lasciapassare-chiave`
(`RUSTDESK_API_AGENTE_LASCIAPASSARE_CHIAVE`): path of the file with the
32-byte Ed25519 seed in base64, on one line. Empty (the default) the
feature is off and the route says so. When set, at startup the file must
exist, not be readable by group and others (permissions 600 or stricter; in
the container owned by 10001) and hold a valid key, otherwise startup stops
with a message saying what is wrong. At startup an INFO line
`lasciapassare dell'agente acceso, chiave pubblica <base64>`: the public key
is not a secret and is there to compare it with the client's. The seed
never goes into the log.

**Command.** `./apimain lasciapassare chiave-nuova <file>` generates the
pair with `crypto/rand`, writes the seed to the file with permissions 600
and prints the public key in base64 on stdout. It reads neither the
configuration nor the database, and refuses with exit code 1 a file that
already exists: a key is not overwritten by mistake. With docker compose,
for example: `docker compose exec remotek-api ./apimain lasciapassare
chiave-nuova /app/data/lasciapassare.key`, then
`RUSTDESK_API_AGENTE_LASCIAPASSARE_CHIAVE=/app/data/lasciapassare.key`.

**Route.** `POST /api/agente/lasciapassare`, with the agent's login token
(`Authorization: Bearer`), body `{"id": "<PC ID>"}`. It answers
`{"lasciapassare": "<string>"}` only if the feature is configured, the
token's user is an enabled AI agent, its technician exists and is enabled,
the PC is in `peers` and it is in the technician's address book: one of
their address books, personal or not, or one shared with them or with
their group by the address book rules (those the RustDesk client shows
them). An agent only works on the PCs its technician works on; a PC
outside the address book also leaves a `lasciapassare rifiutato` warn with
agent, technician and PC. Address books of an AI agent, or of an owner who
no longer exists, do not count even if shared with the technician; and an
agent account can neither write address books nor enter the panel (AI
agents). Otherwise 400 `{"error": msg}`, saying which
condition is missing (in Italian or English); without a token, or with the
agent disabled, 401 like the other login routes. Every pass goes into the
`lasciapassare` table (database version 273): nonce, PC, agent,
technician, issued, expiry, `conn_id` (0 until it opens a session); never
the whole pass nor the signature. In the log an INFO line `lasciapassare
rilasciato` with agent, technician and PC, never the pass nor the nonce;
for the integration the `agente.lasciapassare` event.

**Format.**

- payload = compact JSON, fields in this order:
  `{"v":1,"pc":"<ID>","agente":"<username>","tecnico":"<technician username>","modo":"amministratore","emesso":<unix>,"scade":<emesso+60>,"nonce":"<16 random bytes, base64url without padding>"}`;
- signature = Ed25519(seed, `"remotek-lasciapassare-v1."` + payload);
- pass = base64url(payload) + `.` + base64url(signature), without padding.

Test vector (Ed25519 is deterministic): the seed of bytes 0 to 31
(`AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=`, public key
`A6EHv/POEL4dcN0Y50vAmWfk1jCbpQ1fHdyGZBJVMbg=`) and the payload

```json
{"v":1,"pc":"999000111","agente":"agente-mario","tecnico":"mario","modo":"amministratore","emesso":1791000000,"scade":1791000060,"nonce":"AAECAwQFBgcICQoLDA0ODw"}
```

give the pass

```text
eyJ2IjoxLCJwYyI6Ijk5OTAwMDExMSIsImFnZW50ZSI6ImFnZW50ZS1tYXJpbyIsInRlY25pY28iOiJtYXJpbyIsIm1vZG8iOiJhbW1pbmlzdHJhdG9yZSIsImVtZXNzbyI6MTc5MTAwMDAwMCwic2NhZGUiOjE3OTEwMDAwNjAsIm5vbmNlIjoiQUFFQ0F3UUZCZ2NJQ1FvTERBME9EdyJ9.BRkkm77ln4n8PxwF7Jmi3_-LiCHX7xGjLFjiXb3Knpc3kg9Rlv6DNoxForK3F28DLaUF5HWkudV4ZaWexK4YDA
```

**How the PC verifies it.** It splits the string at the dot, decodes the
two parts (base64url without padding) and verifies the signature with the
built-in public key over the bytes `"remotek-lasciapassare-v1."` + payload
exactly as received, before decoding the JSON. Then it accepts only if `v`
is 1, `modo` is `amministratore`, `pc` equals its own ID,
`emesso - 30 <= now <= scade + 30` (30 seconds of clock tolerance) and it
has never seen the `nonce` before; otherwise it refuses. The window shows
`agente` and `tecnico` from the payload, not the name declared by the
client.

**Link to the connection.** When the PC opens the session with the pass,
it sends its `nonce` in the optional `lasciapassare` field of
`POST /api/audit/conn` (the PC's route, signed like the others: Device
signature), with the `new` or with the authorization. If the nonce belongs
to a pass issued for that same PC that has not opened a session yet, the
API links the pass to the log row: it writes on the pass `audit_conn_id`,
the `id` of the `audit_conns` row (database version 275), and the
connection's `conn_id`. The link that counts is the row: the client gives
`conn_id` and restarts it at every PC reboot, so PC and `conn_id` do not
tell which session it was. Then it marks the row (columns
`agente_lasciapassare` and `tecnico_lasciapassare`, version 274) with the
verified agent and technician; an INFO line says so. A connection already
marked is not linked again, and a connection without `conn_id` does not
link. An unknown nonce, one of another PC or one already used leaves a
warn with the connection, the PC and the reason (never the nonce); a
database error is an error. `scade` does not count here: the PC verifies
the pass when it opens the session, but the nonce can reach the audit
minutes later, at the customer's click. The row is saved as without the
field; the PC gets the usual
answer.

**Administrator rights.** The pass does not say them, the PC does: the
optional `amministratore` field (boolean) of `POST /api/audit/conn`, with
the `new` or with the authorization (where the customer's choice arrives,
for example "accept as user"), says whether that connection's shell really
runs with administrator rights (SYSTEM). The API writes it as received in
the `amministratore` column, even without a nonce: a technician using the
administrator terminal with Windows credentials is an administrator
without being an agent. Without the two fields nothing changes.

In the panel, in the connection log, the "Verified agent" label (with
agent and technician) appears only with a linked nonce, and
"Administrator" only when the PC said so: they are independent. The
`connessione.aperta` and `connessione.chiusa` events carry
`sessione.amministratore` and, for a verified session, `agente` with
`amministratore` taken from the row.

## Origin and licence

Remotek API is based on [rustdesk-api](https://github.com/lejianwen/rustdesk-api)
by lejianwen, version v2.7, released under the MIT licence. This fork is MIT
too: [LICENSE](LICENSE) is unchanged, with the attribution to lejianwen. The
panel is [rustdesk-api-web](https://github.com/lejianwen/rustdesk-api-web), by
the same author, also MIT, built from the source in
[`pannello/`](pannello/README.md), **modified**: a derivative work under the
same licence; its `LICENSE` is in the image at `resources/admin/LICENSE`.
The changes since v2.7, panel included, are listed in
[REMOTEK.md](REMOTEK.md).
