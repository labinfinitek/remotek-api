# Changelog

Formato: Keep a Changelog 1.1.0, in italiano. Versioni: SemVer 2.0.0 (`api-vX.Y.Z`).
Una riga per cambiamento visibile a chi usa o installa il prodotto; la sezione
"Sicurezza" e' obbligatoria per ogni correzione di sicurezza.

## [Non rilasciato]
Base upstream: rustdesk-api v2.7.

### Sicurezza
- Le risposte alle rotte del client RustDesk non contengono piu' il testo
  di un errore interno (REGOLE 8). In 36 punti (login, sysinfo, audit,
  utenti e dispositivi del gruppo, rubrica) l'errore si attaccava al
  messaggio: un corpo JSON rotto a `/api/login` riceveva
  `Parametri non validi.invalid character ...`, un errore del database
  nella rubrica il testo del driver. Ora la risposta, con la stessa forma e
  lo stesso stato HTTP, porta solo il messaggio tradotto, per esempio
  "Parametri non validi." o "Operazione non riuscita."; `/api/users` e
  `/api/peers`, che con parametri non validi rispondevano col solo testo
  dell'errore, rispondono "Parametri non validi.". L'errore va nel log a
  livello warn, con metodo e rotta. Lo stesso vale per la sezione
  dell'utente del pannello (`/api/admin/my/*`, 42 punti: rubriche,
  collezioni e loro regole, tag, dispositivi, log di accesso, condivisioni),
  dove un corpo JSON rotto riceveva il testo del parser dopo il messaggio e
  la cancellazione di un log di accesso o di un tag il solo testo
  dell'errore del database: la risposta resta 200 con `code` 101 e porta
  solo il messaggio. Lo stesso vale per l'amministrazione nel pannello (il
  resto di `/api/admin/*`, 121 punti), dove un errore del database, di
  bcrypt o della rete verso il server finiva nella risposta: un nome
  utente che esiste gia' riceve "Il nome utente esiste già." invece di
  `Operazione non riuscita.UsernameExists`, una password oltre i 72 byte
  di bcrypt "Operazione non riuscita." senza il testo di bcrypt. Resta da
  sistemare solo `/api/oidc/*`.
- Un errore interno che un controller passa come messaggio non arriva piu'
  al client ne' al pannello (REGOLE 8): per esempio l'errore di rete di un
  provider OIDC irraggiungibile, con il suo indirizzo e la sua risposta, da
  `/api/oidc/auth`, `/api/admin/oidc/auth` e dall'associazione di un account
  OAuth nel pannello. La risposta porta "Errore di sistema." (`SystemError`)
  e il testo va nel log a livello warn. Un messaggio che manca in una lingua
  esce in inglese, non piu' come ID.
- Password iniziale di admin (ADR-0008): 20 caratteri casuali invece di 8, e
  non piu' nel log, dove finiva a livello info. Sta nel file
  `data/admin-password.txt`, accanto a `rustdeskapi.db` (nel container
  `/app/data/admin-password.txt`, nel volume), con permessi 0600; il log dice
  solo dov'e'. Va cambiata dal pannello e il file va cancellato. Se il primo
  avvio non riesce a scrivere il file o a creare admin, il processo si ferma
  senza segnare la versione del database e al riavvio riprova da capo: prima
  restava un database senza admin, che `reset-admin-pwd` non trovava. Anche
  un errore nell'aggiornamento delle tabelle ora ferma il processo, invece
  di finire nel log.
- Password di almeno 15 caratteri dove si creano o si cambiano (ADR-0008,
  NIST SP 800-63B-4): registrazione, password impostata dal pannello, cambio
  della propria e comandi `reset-admin-pwd` e `reset-pwd`; prima ne bastavano
  4, e ai comandi nessuna. Il massimo resta 32 e si contano caratteri, non
  byte. Il login non cambia: le password corte gia' impostate continuano a
  funzionare e si possono cambiare. I due comandi escono con codice diverso
  da 0 quando rifiutano la password, quando l'utente non c'e' e quando
  l'aggiornamento non riesce: prima uscivano con 0 e uno script non se ne
  accorgeva.
- Token di sessione casuali (ADR-0008): senza `jwt.key` il token era
  md5(nome utente + ora del login), che si indovina conoscendo il nome utente
  e il momento del login; ora e' fatto di 16 byte da `crypto/rand`, sempre 32
  caratteri esadecimali. Se `crypto/rand` fallisce non si emette nessun
  token: con Go 1.26 si ferma il processo, e se l'errore arrivasse comunque
  fallirebbe il login. I token gia' emessi restano validi fino alla scadenza.
- Default sicuri nel codice (ADR-0008), con `conf/config.yaml` allineato:
  web client (`app.web-client` 0) e login `webauth` dal pannello
  (`app.web-sso` false) spenti, autoregistrazione e swagger spenti come
  prima, captcha dopo 3 login sbagliati e, dopo 10 in 10 minuti, ogni
  richiesta dallo stesso IP rifiutata per 30 minuti (`app.ban-threshold` 10,
  prima 0). `gin.trust-proxy` vuoto, il default, ora vuol dire nessun proxy
  fidato, non tutti: un client non sceglie piu' con `X-Forwarded-For` l'IP
  con cui captcha e ban lo contano. Rottura per chi non configurava niente:
  web client e `webauth` si riaccendono a mano, e dietro un reverse proxy va
  impostato `RUSTDESK_API_GIN_TRUST_PROXY`, altrimenti tutti i client
  contano come l'IP del proxy e 10 login sbagliati di chiunque bloccano
  tutti per 30 minuti.
- govulncheck v1.8.0 in CI: il job fallisce se il nostro codice chiama una
  vulnerabilita' nota. Ferma il merge solo quando `govulncheck` e' tra i
  controlli obbligatori del ruleset di `remotek`: si aggiunge dopo il merge.
- GO-2026-5970, `golang.org/x/text` v0.22.0 -> v0.39.0: ciclo infinito su
  input non valido (raggiunto tramite gorm).
- GO-2026-5004, `github.com/jackc/pgx/v5` v5.6.0 -> v5.9.2: SQL injection per
  confusione dei segnaposto con le stringhe dollar-quoted (driver PostgreSQL).
- GO-2026-4945, `github.com/go-jose/go-jose/v4` v4.0.2 -> v4.1.4: panic nella
  decifratura JWE. Il pacchetto serve alla verifica del token nel login OIDC;
  che il percorso JWE fosse davvero esposto non e' dimostrato (l'avviso non
  elenca i simboli), si corregge comunque.
- GO-2025-4188, `github.com/sirupsen/logrus` v1.8.1 -> v1.8.3: DoS in
  `Entry.writerScanner` (writer degli errori di gin).
- GO-2025-3595, `golang.org/x/net` v0.34.0 -> v0.56.0: neutralizzazione errata
  dell'input nel tokenizer HTML (raggiunto tramite il validatore). La
  correzione e' in v0.38.0; v0.56.0 e' il minimo richiesto da `x/text` v0.39.0.
- GO-2025-3553, `github.com/golang-jwt/jwt/v5` v5.2.1 -> v5.2.2: allocazione
  eccessiva di memoria nel parsing dell'header del token.
- Di conseguenza salgono, al minimo richiesto dai moduli sopra:
  `golang.org/x/crypto` v0.33.0 -> v0.53.0, `x/sys` v0.30.0 -> v0.46.0,
  `x/sync` v0.11.0 -> v0.21.0, `x/tools` v0.26.0 -> v0.47.0.
- Il file di log (`logger.path`, `./runtime/log.txt` in
  `conf/config.yaml`) nasce con permessi 0600 e non piu' 0644, e un file
  che c'era gia' viene portato a 0600 all'avvio: contiene nomi utente e
  indirizzi IP, e lo leggeva ogni utente della macchina. Se il file non si apre o i permessi non si
  cambiano, l'avvio si ferma con un messaggio che dice quale file e perche'.
- Le password salvate come md5(password + "rustdesk-api"), il formato delle
  versioni molto vecchie di rustdesk-api, non sono piu' accettate: prima il
  login le riconosceva e le riscriveva in bcrypt. Un hash che non e' bcrypt
  vale come password sbagliata, al login del client e del pannello e nel
  cambio della propria password. Remotek non ha mai avuto database cosi';
  chi ne importa uno reimposta quelle password con `reset-pwd`.
- Il logout del pannello (`POST /api/admin/logout`) invalida davvero il
  token: la rotta era registrata prima del controllo dell'autenticazione,
  quindi rispondeva successo senza cancellare nulla e il token restava
  valido fino alla scadenza anche dopo "Esci". Ora vuole l'`api-token`
  (senza, o con uno non valido, risponde `code` 403 come ogni rotta
  protetta) e cancella il token; se la cancellazione non riesce risponde
  "Operazione non riuscita." (`code` 101) con l'errore nel log, non piu'
  successo. Il logout del client (`/api/logout`) non cambia.
- Le pagine del login OAuth (successo ed errore) non caricano piu' niente da
  altri siti: quella di errore prendeva Font Awesome da un CDN terzo
  (`lf9-cdn-tos.bytecdntp.com`), che riceveva l'IP di chi la apriva. Le due
  icone sono SVG dentro la pagina, nei colori di prima; quella di successo
  prima non si vedeva, perche' la pagina non caricava il foglio delle icone.

### Corretto
- `RUSTDESK_API_ADMIN_TITLE` vale anche se il file di configurazione non ha
  `admin.title`: prima viper non conosceva la chiave e la ignorava.
- Se il file di configurazione non si legge o non si decodifica, l'avvio
  si ferma con un messaggio che nomina il file, per esempio
  `lettura della configurazione ./conf/config.yaml: ...` o
  `configurazione ./conf/config.yaml non valida: ...`, invece di
  `Fatal error config file: ...` con uno spazio e un a capo in fondo.
- Con la porta dell'API gia' occupata, o un altro errore all'apertura, il
  processo si ferma con codice 1 e scrive l'errore nel log
  (`server API fermato: ...`): prima usciva con codice 0 e l'errore andava
  solo su stderr, fuori dal log, cosi' systemd o Docker lo prendevano per
  uno stop normale e con `on-failure` non lo riavviavano. Lo stop con
  SIGTERM o SIGINT esce ancora con 0.
- Un file di lingua che non si carica ferma l'avvio con un messaggio che lo
  nomina: prima si saltava in silenzio e l'API rispondeva in inglese. Un
  file in `resources/i18n` dal nome di meno di 5 caratteri non manda piu'
  l'avvio in panic.
- 22 messaggi arrivavano all'utente come ID nudi, per esempio
  `UserDisabled` al login di un utente disattivato, `LoginBanned` e
  `NoCaptchaRequired` nel pannello e gli errori di LDAP e OIDC: ora hanno
  il testo in inglese e in italiano, e un test controlla che ogni ID usato
  nel codice sia in `en.toml`.
- Registrazione: il server rifiuta una conferma diversa dalla password. Prima
  salvava la password senza guardare la conferma; il pannello le confrontava
  gia' nel browser.
- Pannello, "aggiungi alla rubrica" dai dispositivi
  (`/api/admin/my/address_book/batchCreateFromPeers`): se una riga non si
  salva la risposta e' "Operazione non riuscita." (`code` 101) con l'errore
  nel log, non piu' successo. Le righe create prima dell'errore restano.
- Pannello, amministrazione della rubrica: la creazione per piu' utenti
  (`/api/admin/address_book/batchCreate`) e l'aggiunta dai dispositivi
  (`/api/admin/address_book/batchCreateFromPeers`) rispondono "Operazione
  non riuscita." (`code` 101) con l'errore nel log se una riga non si
  salva, non piu' successo. Le righe create prima dell'errore restano.

### Rimosso
- Workflow upstream `build.yml` e `build_test.yml` (build e pubblicazione su
  registry altrui): la CI del fork arriva con `remotek-ci.yml`.
- Caricamento di file su Aliyun OSS e in locale dal pannello
  (`/api/admin/file/oss_token`, `/notify`, `/upload`): le rotte erano gia'
  commentate in upstream e rispondevano 404, ora sparisce anche il codice.
  La sezione `oss` della configurazione e le variabili `RUSTDESK_API_OSS_*`
  non si leggono piu': non avevano effetto nemmeno prima.
- Client Redis e cache (`lib/cache`, su file o su Redis): si costruivano
  all'avvio ma nessuna parte dell'API li usava. Le sezioni `redis` e `cache`
  della configurazione e le variabili `RUSTDESK_API_REDIS_*` e
  `RUSTDESK_API_CACHE_*` non si leggono piu': non avevano effetto nemmeno
  prima. Redis non serve piu' ne' per l'API ne' per i test.
- `Dockerfile.dev`, `docker-compose-dev.yaml`, `docker-dev.sh` (build dal
  master del pannello con `npm install`) e `Dockerfile_full_s6` (binario gia'
  compilato dentro l'immagine `rustdesk-server-s6:latest`): li sostituisce il
  `Dockerfile` che costruisce tutto dal sorgente.

### Modificato
- Il prodotto si chiama Remotek dove lo vede chi lo usa: titolo del pannello
  (prima "RustDesk API Admin"), benvenuto del pannello (ora in italiano),
  titolo delle pagine del login OAuth/OIDC. `admin.title` vuoto, come in
  `conf/config.yaml`, vale `brand.name`.
- La lingua predefinita e' l'italiano, nel codice e in `conf/config.yaml`
  (prima `zh-CN` nel file e l'inglese senza file). Il client RustDesk non
  manda `Accept-Language`: riceve i messaggi in italiano, e
  `RUSTDESK_API_LANG=en` riporta all'inglese. I due testi che il client
  confronta alla lettera, `No authed oidc is found` e `SYSINFO_UPDATED`,
  non cambiano.
- Messaggi e validatore scelgono la lingua con la stessa regola: quella di
  `Accept-Language` se l'API la ha, altrimenti `lang`, altrimenti
  l'inglese. Prima il validatore voleva la stringa esatta, e `it-IT`, che il
  pannello manda con un browser italiano, finiva in inglese; i messaggi
  ripiegavano sull'inglese invece che su `lang`. Il validatore ha
  l'italiano, e i nomi dei campi escono nella lingua della risposta: prima
  erano in cinese in ogni lingua (`用户名 is a required field`) o erano il
  nome Go (`ConfirmPassword`, `NewPassword`).
- Si compila con Go 1.26 (toolchain go1.26.8) e `go.sum` e' versionato: le
  dipendenze di un build sono quelle del commit, non quelle del giorno.
- Nell'immagine Docker l'API gira come utente `remotek` (uid/gid 10001), non
  piu' come root. Una cartella del host montata su `/app/data` deve essere
  scrivibile da 10001 (`chown -R 10001:10001 <cartella>`), anche quella
  scritta finora dall'immagine di upstream, altrimenti l'API non parte.
  L'immagine non contiene piu' il web client (`resources/web`), spento di
  default, ne' `docs/`.

### Aggiunto
- Marchio in un posto solo: `brand.name` (`RUSTDESK_API_BRAND_NAME`,
  default "Remotek") e' il nome nel titolo del pannello, in `{{brand}}` del
  benvenuto e nelle pagine OAuth; `brand.dir` (`RUSTDESK_API_BRAND_DIR`,
  default `./resources/brand`) contiene `logo.svg` e `favicon.svg`, che
  l'API serve su `/brand/`: per cambiare logo si sostituiscono i file, anche
  montandoli nel container. Logo e favicon attuali sono provvisori.
- Nell'immagine Docker il pannello prende logo e favicon da `/brand/` e il
  titolo statico da `ARG BRAND_NAME` (default "Remotek"): il `Dockerfile`
  adatta sei righe del sorgente al commit fissato prima di `npm run build`,
  e la build si ferma se una non c'e' piu'.
- `Dockerfile` multi-stage che costruisce l'immagine dal sorgente: binario Go
  statico, pannello rustdesk-api-web di upstream compilato al commit fissato
  `3998c2a` (`ARG PANNELLO_COMMIT`), base Alpine; immagini di base fissate
  per digest, `HEALTHCHECK` su `/api/version`, label OCI, versione da
  `ARG VERSION`. Prima il `Dockerfile` copiava un binario compilato fuori.
- Messaggi dell'API in italiano (`resources/i18n/it.toml`): danno del tu e
  usano i termini della traduzione italiana del client RustDesk (Accedi,
  Nome utente, Password errata, Codice di verifica).
- `SECURITY.md`, `REMOTEK.md`, template di pull request; attribuzione delle
  modifiche in `LICENSE`.
- golangci-lint v2 in CI: bloccante sui pacchetti gia' bonificati, informativo
  sul resto (703 finding ereditati al primo giro, da azzerare nel Passo 3).
- CI `remotek-ci.yml`: ricerca di segreti, controllo di `go.mod`/`go.sum`,
  build, vet e test con `-race`, audit dei workflow.
- `test/contratto/`: risposte di riferimento di rustdesk-api v2.7 alle richieste
  del client 1.4.9, rieseguite dalla CI (push e PR verso `remotek`) sul
  router vero dell'API: tutti i 45 passi dello scenario (anonimi, 404 delle
  cinque richieste che l'API non implementa, utente di collaudo con login,
  rubrica e logout, heartbeat, sysinfo e audit del dispositivo, login con
  password sbagliata).
