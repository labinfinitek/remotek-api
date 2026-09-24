# Changelog

Formato: Keep a Changelog 1.1.0, in italiano. Versioni: SemVer 2.0.0 (`api-vX.Y.Z`).
Una riga per cambiamento visibile a chi usa o installa il prodotto; la sezione
"Sicurezza" e' obbligatoria per ogni correzione di sicurezza.

## [Non rilasciato]
Base upstream: rustdesk-api v2.7.

### Sicurezza
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

### Corretto
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

### Rimosso
- Workflow upstream `build.yml` e `build_test.yml` (build e pubblicazione su
  registry altrui): la CI del fork arriva con `remotek-ci.yml`.

### Modificato
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

### Aggiunto
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
