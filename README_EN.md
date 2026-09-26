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
- **Login log**: every login, from the client and from the panel.
- **Admin panel** on `/_admin/`: [rustdesk-api-web](https://github.com/lejianwen/rustdesk-api-web)
  built into the image at a pinned commit, with the Remotek brand. Users,
  devices, address books, tags, groups, OAuth, logs.
- **Login**: with a password; with a generic **OIDC** provider, configured
  from the panel; with **LDAP** (tested with OpenLDAP and Active Directory),
  configured by file or variables. GitHub and Google login is being retired
  (decision A3): do not configure it on new installations.
- **Languages**: Italian (default) and English.
- **Configurable brand**: name, logo and favicon without touching the code.

## Installation

The image is built from the repository's `Dockerfile`. The release on
`ghcr.io/labinfinitek/remotek-api` will come with the `api-v0.1.0` tag: until
then the image is built locally.

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
`rustdeskapi.db` and `admin-password.txt`. The log goes to stdout
(`docker compose logs`) and to `/app/runtime/log.txt`, inside the container.

### Building the image

```bash
docker build \
  --build-arg VERSION=<version> \
  --build-arg REVISION="$(git rev-parse HEAD)" \
  -t remotek-api .
```

Three stages: static Go binary (CGO for SQLite), rustdesk-api-web panel at the
pinned commit, final Alpine image. Base images are pinned by digest.
Arguments:

| Argument | Default | Use |
|---|---|---|
| `VERSION` | `dev` | written to `resources/version`, returned by `/api/version` |
| `REVISION` | `unknown` | OCI label `org.opencontainers.image.revision` |
| `PANNELLO_COMMIT` | `3998c2a9213fcd047252776d0f0db33e6717026c` | full sha of the rustdesk-api-web commit |
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
`go generate -tags tools ./tools` (needs `swag` in `PATH`).

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
| `app.web-sso` \* | `RUSTDESK_API_APP_WEB_SSO` | `false` | offers the client the login confirmed from the panel (`webauth`) |
| `app.disable-pwd-login` | `RUSTDESK_API_APP_DISABLE_PWD_LOGIN` | `false` | `true` removes password login, OIDC and LDAP remain |
| `admin.title` \* | `RUSTDESK_API_ADMIN_TITLE` | empty | panel title; empty = `brand.name` |
| `admin.hello` | `RUSTDESK_API_ADMIN_HELLO` | empty | panel welcome message (HTML); when not empty, `admin.hello-file` is not read |
| `admin.hello-file` | `RUSTDESK_API_ADMIN_HELLO_FILE` | `./conf/admin/hello.html` | welcome file; `{{username}}` and `{{brand}}` are replaced |
| `admin.id-server-port` | `RUSTDESK_API_ADMIN_ID_SERVER_PORT` | `21116` | `hbbs` port for the panel's commands, sent to `127.0.0.1` on this port minus one |
| `admin.relay-server-port` | `RUSTDESK_API_ADMIN_RELAY_SERVER_PORT` | `21117` | `hbbr` port for the panel's commands, on `127.0.0.1` |
| `gin.api-addr` | `RUSTDESK_API_GIN_API_ADDR` | `0.0.0.0:21114` | listen address |
| `gin.mode` | `RUSTDESK_API_GIN_MODE` | `release` | `release`, `debug`, `test` |
| `gin.resources-path` | `RUSTDESK_API_GIN_RESOURCES_PATH` | `resources` | directory of panel, languages and templates; without the language files startup stops |
| `gin.trust-proxy` \* | `RUSTDESK_API_GIN_TRUST_PROXY` | empty | trusted proxy IPs or CIDRs, comma separated; empty = none, `X-Forwarded-For` and `X-Real-IP` ignored. Behind a reverse proxy it must be set, otherwise captcha and ban count every client as the proxy IP. An invalid value stops startup |
| `gorm.type` | `RUSTDESK_API_GORM_TYPE` | `sqlite` | only `sqlite` (empty is the same); any other value stops startup. The database is `data/rustdeskapi.db` |
| `gorm.max-idle-conns` | `RUSTDESK_API_GORM_MAX_IDLE_CONNS` | `10` | idle connections kept open |
| `gorm.max-open-conns` | `RUSTDESK_API_GORM_MAX_OPEN_CONNS` | `100` | maximum open connections |
| `rustdesk.id-server` | `RUSTDESK_API_RUSTDESK_ID_SERVER` | example address | `host:21116` of `hbbs`, to be set; shown by the panel |
| `rustdesk.relay-server` | `RUSTDESK_API_RUSTDESK_RELAY_SERVER` | example address | `host:21117` of `hbbr`, to be set |
| `rustdesk.api-server` | `RUSTDESK_API_RUSTDESK_API_SERVER` | `http://127.0.0.1:21114` | address of this API as clients and browsers see it; gives the OIDC callback `<api-server>/api/oidc/callback` |
| `rustdesk.key` | `RUSTDESK_API_RUSTDESK_KEY` | empty | public key of `hbbs`; empty = `rustdesk.key-file` is read |
| `rustdesk.key-file` | `RUSTDESK_API_RUSTDESK_KEY_FILE` | `/data/id_ed25519.pub` | key file; if it cannot be read, the key stays empty without an error |
| `rustdesk.personal` | `RUSTDESK_API_RUSTDESK_PERSONAL` | `1` | `1` personal address book on, `0` off |
| `logger.path` | `RUSTDESK_API_LOGGER_PATH` | `./runtime/log.txt` | log file (0600), besides stdout; empty = stdout only. If it cannot be opened, startup stops |
| `logger.level` | `RUSTDESK_API_LOGGER_LEVEL` | `info` | `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`; an invalid value means `debug` |
| `logger.report-caller` | `RUSTDESK_API_LOGGER_REPORT_CALLER` | `true` | source file and line in every log line |
| `proxy.enable` | `RUSTDESK_API_PROXY_ENABLE` | `false` | HTTP proxy for the API's requests to the OAuth/OIDC provider |
| `proxy.host` | `RUSTDESK_API_PROXY_HOST` | `http://127.0.0.1:1080` | proxy address |
| `jwt.key` | `RUSTDESK_API_JWT_KEY` | empty | empty: random session tokens (16 bytes, hex); set: JWT tokens signed with this key. With the official server leave it empty |
| `jwt.expire-duration` | `RUSTDESK_API_JWT_EXPIRE_DURATION` | `168h` | JWT lifetime |
| `ldap.enable` | `RUSTDESK_API_LDAP_ENABLE` | `false` | LDAP login; if LDAP refuses or does not answer, the local user is tried |
| `ldap.url` | `RUSTDESK_API_LDAP_URL` | `ldap://ldap.example.com:389` | `ldap://` or `ldaps://` |
| `ldap.tls-ca-file` | `RUSTDESK_API_LDAP_TLS_CA_FILE` | empty | server CA, with `ldaps://` |
| `ldap.tls-verify` | `RUSTDESK_API_LDAP_TLS_VERIFY` | `false` | with `ldaps://`, `false` does not verify the certificate: set it to `true` |
| `ldap.base-dn` | `RUSTDESK_API_LDAP_BASE_DN` | `dc=example,dc=com` | base DN |
| `ldap.bind-dn` | `RUSTDESK_API_LDAP_BIND_DN` | `cn=admin,dc=example,dc=com` | service user for searches |
| `ldap.bind-password` | `RUSTDESK_API_LDAP_BIND_PASSWORD` | example value | password of the service user: from the variable, not in the file |
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
the `Issuer`, `Scopes` default to `openid,profile,email`, callback URL
`<rustdesk.api-server>/api/oidc/callback`.

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
./apimain -h                                # help
```

The password must be 15 to 32 characters long (characters, not bytes), like
every new password set from the panel or at registration. When the password
is refused, the user does not exist or the update fails, the command exits
with a non-zero code.

**Connecting the client.** In the RustDesk or Remotek client, under
Settings > Network: ID server, relay server, API server (`rustdesk.api-server`)
and key, the same values as the `RUSTDESK_API_RUSTDESK_*` variables.

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

Not renamed: the `RUSTDESK_API_` prefix, the `rustdesk:` section, the
`/api/admin/rustdesk/*` routes, the Go module path.

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
- **No external resources** in the pages the API generates (OAuth/OIDC login
  result). Files under `/brand/` are served with a `Content-Security-Policy`
  that runs no scripts and with `X-Content-Type-Options: nosniff`.
- **LDAP**: with `ldaps://` set `ldap.tls-verify: true`; the default `false`
  does not verify the certificate.
- Report vulnerabilities as described in [SECURITY.md](SECURITY.md), not with
  public issues.

## Origin and licence

Remotek API is based on [rustdesk-api](https://github.com/lejianwen/rustdesk-api)
by lejianwen, version v2.7, released under the MIT licence. This fork is MIT
too: [LICENSE](LICENSE) is unchanged, with the attribution to lejianwen. The
panel is [rustdesk-api-web](https://github.com/lejianwen/rustdesk-api-web), by
the same author, built from source. The changes since v2.7 are listed in
[REMOTEK.md](REMOTEK.md).
