# Istruzioni per l'agente che lavora su questo repo

Questo repo e' l'API di **Remotek**, il servizio di assistenza remota di
Infinitek, fork di rustdesk-api v2.7 (lejianwen, MIT). Upstream e' fermo:
questo fork e' il progetto. Cosa e' cambiato rispetto a v2.7: `REMOTEK.md`.

Le regole complete stanno in un repo privato che da qui non si legge: questo
file ne e' l'estratto che serve per lavorare sull'API. Se una regola manca o
non e' chiara, ci si ferma e si chiede; non si indovina.

## Chi decide

Il titolare decide su marchio, licenze, spese e priorita'. Le scelte tecniche
le fa l'agente e le motiva in due righe nella MR. Si scrive in italiano, con
franchezza: un problema si dice subito, un dubbio si dichiara come dubbio,
"fatto" significa verificato, con l'output incollato.

## Cosa non si fa

- Niente accesso a server, VM, DNS o infrastruttura, neanche se raggiungibili.
- Niente merge: l'agente apre la MR verso `remotek`, il titolare approva.
  Aprire la MR a lavoro fatto e verificato e' autorizzato sempre, senza che
  la richiesta lo ripeta; non la si apre solo se la richiesta lo esclude.
- Niente push su `remotek`, niente tag.
- Niente modifiche a `.github/workflows/`, anche se l'accesso GitHub lo
  permetterebbe: le MR `ci:` le fa l'agente locale, perche' le regole della
  CI (azioni fissate per SHA, `permissions:` minimi, niente cron ne' tag
  automatici) stanno nel repo privato che da qui non si legge.
- Niente segreti nel repo, nei log, nei messaggi di commit. Il repo e' pubblico.
- `LICENSE` resta intatto con l'attribuzione a lejianwen.

## Flusso

- Branch di lavoro `<tipo>/<argomento>` da `remotek`; una MR, un argomento,
  massimo 300 righe modificate (esclusi `go.sum` e file generati). Nel cloud
  il push e' ammesso solo sul branch assegnato alla sessione (`claude/...`):
  si usa quello, e il tipo sta nel titolo della MR e nei commit.
- MR verso `labinfinitek/remotek-api`, base `remotek`, mai verso il progetto
  originale. Con `gh`: sempre `--repo labinfinitek/remotek-api`. Nel cloud
  GitHub si usa dal connettore (o da `gh`, se c'e'), con repo e base
  espliciti.
- Descrizione con il template `.github/pull_request_template.md`, tutte le
  sezioni: cosa cambia, perche', come verificato (comandi e output), cosa non
  verificato e perche', cosa serve dal titolare.
- Riga in "Non rilasciato" di `CHANGELOG.md` per `feat`, `fix`, `sec`,
  `build`; riga in `REMOTEK.md` se cambia l'inventario delle modifiche.

## Commit

Conventional Commits: `<tipo>(<ambito>): <oggetto>`, oggetto in italiano,
indicativo presente terza persona ("aggiunge", "corregge"), minuscolo, senza
punto finale, massimo 72 caratteri. Il corpo dice perche', non cosa. Un
commit, un argomento.

Tipi: `feat`, `fix`, `sec` (sicurezza, sempre nel changelog sotto
"Sicurezza"), `refactor` (stesso comportamento, test del contratto verdi
senza modifiche), `test`, `docs`, `ci`, `build` (Dockerfile, toolchain,
dipendenze). Niente `chore`, `style`, `perf`. Ambito = il pacchetto toccato
(`auth`, `ab`, `audit`, `peer`, `admin`, `i18n`, `store`, `config`,
`router`, `docker`, `pannello`). Rottura del contratto col client o di una
variabile `RUSTDESK_API_*`: `!` dopo il tipo e footer `BREAKING CHANGE:`.

I trailer che l'ambiente cloud aggiunge ai commit (`Co-Authored-By: Claude`,
`Claude-Session: <link>`) e la riga finale delle MR restano: legano ogni
commit alla sessione che l'ha prodotto, e il link si apre solo con l'account
del titolare.

## Verifica prima della MR

Dove c'e' Go (sessione cloud) si verifica in locale, e l'output va nella MR:

```bash
go build ./...
go vet ./...
go test -race -shuffle=on ./...
```

`go.mod` chiede Go 1.26 (toolchain go1.26.8): l'ambiente cloud ha un Go piu'
vecchio e `GOTOOLCHAIN=auto`, quindi scarica da solo la versione giusta.

Test Redis: senza `REDIS_ADDR` si saltano. Nel container c'e' `redis-server`:
lo si puo' avviare li' (mai altrove) e lanciare i test con
`REDIS_ADDR=127.0.0.1:6379`. Fa fede la CI, che usa redis 7.4.11.

gitleaks, govulncheck, golangci-lint e zizmor non si lanciano in locale: le
versioni che contano sono quelle fissate nei workflow, e un secondo elenco da
tenere allineato si sbaglia. Prima di dichiarare pronta la MR si legge
l'esito della CI (segreti, go.sum, build/vet/test, test del contratto, lint,
govulncheck, zizmor) e lo si riporta.

`resources/web/` e' il web client di RustDesk incluso da upstream (15 MB,
spento di default): non si aggiorna a pezzi, e gli alert Dependabot che lo
riguardano non si correggono qui. Si segnalano nella MR.

## Regole del codice

- **Il contratto col client non cambia.** Gli endpoint chiamati dal client
  RustDesk rispondono esattamente come oggi: HTTP 4xx con `{"error": msg}`,
  corpi particolari inclusi (`data` come stringa in `/api/ab`, `null`, testo
  `SYSINFO_UPDATED`, 401 = logout forzato, 404 = "server legacy"). Mai
  `err.Error()` interni nel messaggio. Il controllo e' `TestContract` in
  `cmd/contratto_test.go` con i golden in `test/contratto/`: deve restare
  verde **senza toccare i golden**. I golden si registrano solo contro
  l'istanza di riferimento, che da qui non si raggiunge: se un golden va
  cambiato, ci si ferma e lo si scrive nella MR.
- Prima di toccare un modulo, ogni endpoint del client che quel modulo serve
  deve avere il suo passo nel test del contratto; se manca, lo si segnala.
- Ogni chiamata gorm legge `.Error` e lo propaga con `%w`; "non trovato" e'
  `ErrNotFound`, non `Id == 0`. Niente nuove variabili in `global`.
- Log con `log/slog`, livelli con significato, niente `fmt.Print*`, niente
  segreti o dati personali superflui.
- Ogni opzione di configurazione ha un default **sicuro nel codice**,
  documentato nel README, con un test per i default di sicurezza. Nessun
  segreto in `conf/`: l'istanza si configura con env `RUSTDESK_API_*`.
- Test: libreria standard + `go-cmp`; niente testify o mock senza ADR. Test
  che dipendono da Redis: `t.Skip` senza `REDIS_ADDR`.
- Dipendenze: `go.sum` versionato, `-mod=readonly`, nessuna dipendenza nuova
  senza una riga di motivo nella MR; salto di major solo con ADR (lo scrive
  il titolare con l'agente, non si decide in una MR).
- Layout di arrivo: `cmd/`, `internal/{app,http,service,store,model,config,
  i18n}`, `pkg/`, `migrations/`, `test/contratto/`. Si migra un modulo alla
  volta; bonifica e funzione nuova non stanno nella stessa MR.
- Commenti nuovi in italiano; quelli cinesi si sostituiscono solo nel codice
  che si riscrive. Refusi nei nomi (`ouath`, `requstform`) si correggono
  quando si tocca il file.
