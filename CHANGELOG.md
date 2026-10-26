# Changelog

Formato: Keep a Changelog 1.1.0, in italiano. Versioni: SemVer 2.0.0 (`api-vX.Y.Z`).
Una riga per cambiamento visibile a chi usa o installa il prodotto; la sezione
"Sicurezza" e' obbligatoria per ogni correzione di sicurezza.

## [Non rilasciato]

## [0.12.0] - 2026-10-26
Immagine `ghcr.io/labinfinitek/remotek-api:0.12.0`, costruita con Go 1.26.10
su Alpine 3.24.3. Versione minore: un tecnico nuovo si associa al suo
account OIDC con un codice che l'amministratore conferma dal pannello
(ADR-0031), senza password e senza riavvii; una correzione di sicurezza
sulle regole di condivisione; rifiniture di OIDC e pannello. Nessuna
migrazione del database (`DatabaseVersion` resta 275), nessuna variabile
nuova, contratto col client invariato. Rotte nuove solo del pannello e
per i soli amministratori. Dalla 0.11.0 si passa direttamente alla 0.12.0.

### Aggiunto
- OIDC con il login con password spento: un account senza utente, con
  `auto_register` spento, riceve nella pagina del callback un codice di
  associazione ("K7Q2-M9XD", 30 minuti) da comunicare all'amministratore,
  che nel pannello (Sistema, Associazioni in attesa) lo associa a un utente
  esistente; prima la pagina di associazione voleva il login al pannello e
  il tecnico nuovo non entrava. Rotte `GET /api/admin/oauth/richieste` e
  `POST /api/admin/oauth/associa`, solo amministratori; richieste in
  memoria, al massimo 100, tolte a tempo dopo 30 minuti (con nome ed
  email); l'associazione scrive anche lo username del provider, come quella
  dalla pagina di associazione. Con il login con password acceso resta la
  pagina di associazione, che mostra anche il codice.

### Sicurezza
- Regole di condivisione della rubrica: creazione e modifica, dal pannello
  di amministrazione e dalla sezione dell'utente, rifiutano come
  destinatario un utente disattivato o un account agente ("Non si condivide
  con un utente disattivato o con un account agente."), anche per chiamata
  diretta all'API; prima li toglieva solo la scelta del pannello. Una
  regola gia' salvata verso uno di loro resta e si puo' cancellare.

### Corretto
- OIDC con autoregistrazione: l'email del provider abbina anche un utente
  locale con l'email scritta con maiuscole; prima nasceva un secondo
  utente con la stessa email.
- Pannello: nella lista dei PC e in I miei PC "Ultimo collegamento" e'
  fissata accanto ad Azioni, che a 1280 px la copriva, intestazione e
  pallino di stato compresi.
- Pannello: la pagina di registrazione mostra il titolo sotto il simbolo,
  come il login.

## [0.11.0] - 2026-10-25
Immagine `ghcr.io/labinfinitek/remotek-api:0.11.0`, costruita con Go 1.26.10
su Alpine 3.24.3. Versione minore: la risposta del login del pannello ha il
campo nuovo `group_name`. Due correzioni di sicurezza (la lista dei
destinatari di una condivisione non espone piu' i dati degli utenti; i
tempi del login con password non dicono piu' chi e' un'eccezione o se un
utente esiste) e rifiniture di OIDC e pannello. Nessuna migrazione del
database (`DatabaseVersion` resta 275), nessuna variabile nuova, contratto
col client invariato. Ogni login con password fallito per uno username che
non esiste costa ora un bcrypt, come uno con la password sbagliata. Il
marchio di default diventa un simbolo: chi ha il suo in `brand.dir` non
vede differenze. Dalla 0.10.0 si passa direttamente alla 0.11.0.

### Sicurezza
- Pannello: `POST /api/admin/user/groupUsers`, aperto a ogni utente del
  pannello, dava tutti gli utenti con email, nota, `is_admin`, stato,
  `agente_di`, avatar e date. Ora risponde, uguale per amministratori e
  utenti, solo con quanto serve a scegliere il destinatario di una regola di
  condivisione: gruppi con id e nome, utenti con id, username, nickname e
  gruppo, senza utenti disattivati e account agente. Una regola gia' salvata
  verso un utente escluso resta nella lista, come `#<id> (escluso)`.
- Login con password spento: un nome di `app.pwd-login-eccezioni` senza
  utente locale, o con un hash che non e' bcrypt, fa anche lui il controllo
  bcrypt su un hash finto, come i nomi fuori dall'elenco: i tempi della
  risposta non dicono piu' quali nomi hanno un utente. Anche con il login
  con password acceso uno username che non esiste fa ora un bcrypt su un
  hash finto, cosi' i tempi non dicono se esiste. Il README dice il formato
  esatto delle righe di log del login per eccezione e che con LDAP acceso i
  tempi distinguono ancora le eccezioni.

### Modificato
- Marchio di default: `resources/brand/logo.svg` e `favicon.svg` sono un
  simbolo, una "R" bianca su un quadrato arrotondato blu `#1F6BFF`, e non
  piu' la parola "Remotek"; la barra del pannello mostra simbolo e titolo,
  senza il nome ripetuto. Chi ha il suo marchio in `brand.dir` non vede
  differenze.

### Corretto
- OIDC: la richiesta di associazione dal Profilo di un utente che ha gia' un
  account del provider risponde "Questo utente ha già un account associato a
  questo provider OIDC." e non piu' che l'account e' di un altro utente; in
  italiano il messaggio dell'account gia' associato dice "account OIDC" e
  non "account OAuth". L'error dei doppioni di `user_thirds` all'avvio dice
  quante righe sono, che l'indice non c'e', che l'avvio continua, cosa fare
  e dove leggere nel README.
- Provider OIDC: un segreto del client di soli spazi vale come vuoto (in
  modifica tiene quello salvato, alla creazione si rifiuta); nel pannello il
  campo del segreto ha l'asterisco dell'obbligatorio alla creazione; lo
  swagger non descrive piu' `client_secret` nelle risposte di lista e
  dettaglio.
- Pannello: il Profilo mostra il gruppo dell'utente. La risposta del login
  del pannello e di `/api/admin/user/current` ha il campo nuovo
  `group_name` (vuoto se il gruppo non c'e'); il login del client non
  cambia.

## [0.10.0] - 2026-10-25
Immagine `ghcr.io/labinfinitek/remotek-api:0.10.0`, costruita con Go 1.26.10
su Alpine 3.24.3. Versione minore: il pannello prende la veste di Infinitek
e del client Remotek (colori, caratteri ospitati, barre, login, tabelle,
pulsanti, Profilo) e legge il marchio di chi installa da un `brand.json`
facoltativo. Cambia solo il pannello: nessuna migrazione del database
(`DatabaseVersion` resta 275), nessuna variabile nuova, rotte e contratto
col client invariati. Le chiavi `admin.hello` e `admin.hello-file`
restano ma il pannello non le mostra piu'. Dalla 0.9.0 si passa
direttamente alla 0.10.0.

### Aggiunto
- Pannello: marchio di chi installa da `brand.json`, facoltativo, in
  `brand.dir` (servito su `/brand/brand.json`): ragione sociale, link
  all'informativa, nome e icona del pulsante OIDC del login, logo per il
  tema scuro. Senza il file, valori neutri. Formato nel README ("Marchio del
  pannello").

### Modificato
- Pannello nello stile di Infinitek e del client Remotek: colori in un
  posto solo (variabili di Element Plus in `style.scss`, valori del client),
  fondo chiaro con accento blu `#1F6BFF` e tema scuro navy; barra in alto
  chiara con logo e nome e una riga blu sotto; barra laterale navy con
  "Area personale" e "Amministrazione" come intestazioni; schede aperte con
  una riga blu sotto quella attiva; piede col nome, la ragione sociale di
  `brand.json` e la versione dell'API. Login su fondo chiaro: titolo
  "<nome> · Console", pulsante OIDC principale ("Accedi con ..."), modulo
  della password sotto e secondario (nascosto con `app.disable-pwd-login`,
  salvo `?password=1`), ragione sociale e informativa di `brand.json` in
  fondo. Stessi colori per le pagine di associazione OAuth e registrazione.
- Pannello: caratteri Inter (testo) e Space Grotesk (titoli e nome del
  prodotto), ospitati nel pannello dai pacchetti `@fontsource` (licenza OFL
  1.1, in `resources/admin/LICENSE-inter` e `LICENSE-space-grotesk`): il
  pannello non chiama siti di terzi.
- Pannello: il Profilo non mostra piu' la card di benvenuto col testo di
  `admin.hello-file` (o `admin.hello`). Le due chiavi restano in
  configurazione ma il pannello non le legge piu'.

### Corretto
- Pannello: via il pulsante "Connetti" dalle liste di PC, I miei PC,
  Rubrica e Voci di rubrica. Apriva un link `rustdesk://<id>`, che il
  client Remotek non gestisce: il clic non faceva niente. La colonna
  "Azioni" si stringe di conseguenza.
- Pannello: rosso solo per eliminare. "Aggiungi" e "Cerca" sono blu pieni,
  "Esporta" e "Importa" neutri. Nella colonna "Azioni" restano visibili al
  massimo due azioni, le altre vanno nel menu "Altro"; "Elimina" sta sempre
  nel menu, in rosso, e chiede conferma (in Utenti "Reimposta la password"
  usciva dalla colonna). Il pulsante blu premuto nel tema scuro ora si
  legge (bianco su `#4C89FF` faceva 3.32:1).
- Pannello: tabelle leggibili. Testata su fondo tenue, celle su una riga
  con i puntini e il testo intero nel tooltip (note, UUID), date intere,
  nessuna parola spezzata a meta' ("Nome visualizz-ato"); le etichette dei
  filtri stanno su una riga.
- Pannello: il Profilo riassume utente, email e account OIDC collegato al
  posto della card tolta; "Cambia password" non e' piu' rosso e sparisce
  con `app.disable-pwd-login`.

## [0.9.0] - 2026-10-23
Immagine `ghcr.io/labinfinitek/remotek-api:0.9.0`, costruita con Go 1.26.10
su Alpine 3.24.3. Versione minore: prepara la MFA dei tecnici via OIDC
(ADR-0029). Eccezioni al login con password spento, accesso di emergenza
nel pannello, legame OIDC uno a uno, segreto del client che non torna al
pannello. Nessuna migrazione del database (`DatabaseVersion` resta 275):
all'avvio si crea un indice unico nuovo su `user_thirds`; con doppioni,
un error col loro numero e l'avvio continua senza indice. Configurazione:
`app.pwd-login-eccezioni` e' nuova e vuota, e conta solo con
`app.disable-pwd-login` acceso. Contratto col client invariato; solo
l'auth-query OIDC del client risponde 400 a un utente disattivato, che
prima riceveva il token. Dalla 0.8.0 si passa direttamente alla 0.9.0.

### Sicurezza
- Eccezioni al login con password spento (ADR-0029): con
  `app.disable-pwd-login` acceso `POST /api/login` accetta ancora la
  password degli account agente AI e, come `POST /api/admin/login`, degli
  username in `app.pwd-login-eccezioni` (nuova, vuota per default; gli
  agenti restano fuori dal pannello). Agli altri, a un utente inesistente e a
  un'eccezione con la password sbagliata la risposta di prima, senza dire chi
  e' un'eccezione; ogni rifiuto ora conta per captcha e ban. Un login per
  eccezione scrive una riga INFO, un warn se l'utente e' amministratore. Il
  pannello mostra il modulo della password su `/_admin/#/login?password=1`
  (accesso di emergenza se l'OIDC non risponde).
- Legame OIDC sicuro (ADR-0029): indice unico su `user_thirds(open_id, op)`,
  creato all'avvio (con doppioni un error col loro numero e l'avvio continua
  senza indice; nessuna riga si cancella). L'associazione dal pannello e dal
  callback rifiuta, nella stessa transazione della scrittura, un `sub` gia'
  associato a qualcuno e un utente che ha gia' un account di quel provider.
  `/api/oidc/auth-query` e `/api/admin/oidc/auth-query` rifiutano un utente
  disattivato ("Utente disattivato.") prima di creare il token. Con
  `auto_register` l'email abbina un utente locale solo se il provider la
  dichiara verificata, e mai un agente AI o un utente disattivato: in quei
  casi il login si ferma invece di entrare come quell'utente.
- Il segreto del client OIDC non torna piu' al pannello: lista e dettaglio
  del provider (`/api/admin/oauth/list`, `/detail/:id`) lo sostituiscono con
  `has_client_secret`; nell'aggiornamento un segreto vuoto tiene quello
  salvato. Il pannello lascia vuoto il campo in modifica.

### Corretto
- Gli scope OIDC salvati con spazi o virgole in piu' (`openid, profile ,`)
  arrivavano al provider con gli spazi e con scope vuoti: ora si puliscono.
- Il pannello teneva il codice del login OIDC per 60 secondi: con login e
  TOTP su Keycloak al ritorno era gia' scaduto e il login non si chiudeva.
  Ora vale 5 minuti, quanto sul server.

## [0.8.0] - 2026-10-22
Immagine `ghcr.io/labinfinitek/remotek-api:0.8.0`, costruita con Go 1.26.10
su Alpine 3.24.3. Versione minore: il lasciapassare dell'agente AI
(ADR-0028, parte dell'API), gli account agente limitati al client a riga
di comando, il pannello con la sigla della lingua e il logo leggibile, e
**un cambio incompatibile**: via i comandi al server (pagina e rotte
`/api/admin/rustdesk/*`). Migrazioni del database 273-275, solo aggiunte
(tabella `lasciapassare`, colonne nuove di `audit_conns`); la tabella
`server_cmds` resta. Configurazione: `agente.lasciapassare-chiave` e'
nuova e vuota per default, cioe' il lasciapassare e' spento;
`admin.id-server-port` e `admin.relay-server-port` non servono piu' e si
ignorano. Contratto col client RustDesk invariato. Dalla 0.6.0 o dalla
0.7.x si passa direttamente alla 0.8.0.

### Aggiunto
- Lasciapassare dell'agente (ADR-0028): con `agente.lasciapassare-chiave`
  (file col seme Ed25519, permessi 600) `POST /api/agente/lasciapassare`
  rilascia a un agente AI attivo, con il tecnico attivo, un lasciapassare
  firmato per un PC nella rubrica del tecnico (una sua rubrica o una
  condivisa con lui o con il suo gruppo), valido 60 secondi, che il PC
  verifica con la chiave pubblica cablata; un altro PC e' rifiutato con 400
  e un warn con agente, tecnico e PC. Registro nella tabella `lasciapassare`
  (migrazione 273) col solo nonce, riga INFO e evento
  `agente.lasciapassare`; comando `lasciapassare chiave-nuova <file>`.
  Senza chiave la rotta lo rifiuta; un file che manca, leggibile da altri o
  con una chiave non valida ferma l'avvio (README, Lasciapassare
  dell'agente).
- Lasciapassare dell'agente: `POST /api/audit/conn` accetta il campo
  facoltativo `lasciapassare` col nonce; se e' di un lasciapassare per
  quello stesso PC non ancora usato, lo lega alla connessione e segna la
  riga del registro (migrazione 274) con agente e tecnico verificati. Il
  campo facoltativo `amministratore`, che manda il PC, dice se la shell ha
  i permessi di amministratore, con o senza agente. Nel pannello le
  etichette, indipendenti, "Agente verificato" e "Amministratore";
  `connessione.aperta` e `connessione.chiusa` portano
  `sessione.amministratore` e, se verificate, `agente`. Un nonce
  sconosciuto, di un altro PC o gia' usato da' un warn, un errore del
  database un error, e la riga si salva come prima; senza i campi non
  cambia niente. Il lasciapassare si lega alla riga del registro delle
  connessioni (`audit_conn_id`, migrazione 275), una volta sola, e non
  solo al `conn_id`, che il client fa ripartire a ogni riavvio del PC
  (README, Lasciapassare dell'agente).

### Rimosso
- **Cambio incompatibile per chi usava le rotte `/api/admin/rustdesk/*`.**
  Via i comandi al server: la pagina "Comandi al server" del pannello e le
  rotte `sendCmd`, `cmdList`, `cmdCreate`, `cmdDelete` e `cmdUpdate`, che
  ora rispondono come una rotta che non esiste. Nel server di Remotek l'API
  sta nella rete Docker e non raggiunge `hbbs` e `hbbr` sul loopback, quindi
  la pagina diceva sempre "Non disponibile"; relay, blocchi e limiti li
  gestisce chi installa il server. La tabella `server_cmds` di un database
  esistente resta, e nessuno la legge piu'. Le chiavi
  `admin.id-server-port` e `admin.relay-server-port` (e le variabili
  `RUSTDESK_API_ADMIN_ID_SERVER_PORT` e `RUSTDESK_API_ADMIN_RELAY_SERVER_PORT`)
  non servono piu': una configurazione che le ha parte lo stesso e le ignora.

### Corretto
- Pannello: il selettore della lingua mostra la sigla della lingua attiva
  ("IT", "EN") al posto del glifo "文A", con l'etichetta accessibile
  "Lingua: IT" / "Language: EN".
- Pannello: il logo della barra e del login ha l'altezza fissa (30 e 80 px)
  e la larghezza del suo formato, fino a 160 e 320 px; un logo largo non
  si rimpicciolisce piu' in un quadrato. Un logo quadrato resta com'era.

### Sicurezza
- Agenti AI: un account agente usa solo le rotte del client a riga di
  comando (`/api/user/info`, `/api/currentUser`, `/api/logout`,
  `/api/agente`, `/api/agente/lasciapassare`); le altre rotte con login gli
  rispondono 403, il login del pannello non gli da' un token e il pannello
  rifiuta anche un token gia' rilasciato. Le rubriche di un agente non
  contano per il lasciapassare. Prima un account agente entrava nel
  pannello e usava le rotte del client come un utente qualunque del
  gruppo del suo tecnico (README, Agenti AI).
- Immagine di base Alpine 3.24.3: zlib 1.3.2-r1 (CVE-2026-85091, MEDIUM) e
  musl 1.2.6-r3 (CVE-2026-107891), che trivy segnalava sulla 3.24.2. Il
  binario dell'API e' statico e non le usa; le usano gli strumenti di
  sistema dell'immagine (busybox, il `wget` dell'HEALTHCHECK).

## [0.7.2] - 2026-10-22
Immagine `ghcr.io/labinfinitek/remotek-api:0.7.2`, costruita con Go 1.26.10.
Rilascio di sicurezza e di correzione: Go 1.26.10 per GO-2026-6809
(`crypto/x509`), piu' piccole correzioni dell'integrazione e del pannello.
Nessun cambio incompatibile: nessuna migrazione del database, nessuna
variabile nuova, rotte e contratto col client invariati. Dalla 0.6.0 si
puo' passare direttamente alla 0.7.2.

### Corretto
- Integrazione: un trasferimento di file con un type diverso da 0 (dal PC) e
  1 (al PC) non accoda piu' `file.trasferiti` come `dal_pc`, ma scrive un
  warn con il type e l'ID del PC; la riga del registro si salva come prima.
- Integrazione: se la coda degli eventi nel database non risponde, la riga
  ERROR "coda degli eventi" esce una volta quando il guasto comincia (o
  cambia messaggio) invece che ogni 5 secondi, e una riga INFO dice quando
  la coda torna a rispondere.
- Pannello: "Esci" con il token gia' scaduto o senza rete non mostra piu'
  per un attimo l'avviso rosso ("Accedi prima di continuare.", "Network
  Error") prima della pagina di accesso; l'uscita resta la stessa.
- Pannello: l'avviso dell'esportazione della trascrizione non riuscita
  comincia con la maiuscola ("Risposta HTTP 502 dal server"); nella
  finestra, dopo i due punti, resta minuscolo.
- Pannello: le colonne "Azioni" della rubrica (anche in "Area
  personale") e dei PC di "Area personale" hanno la larghezza dei
  pulsanti rimasti, senza lo spazio vuoto dei pulsanti del web client.

### Sicurezza
- Go 1.26.10: corregge GO-2026-6809 (CVE-2026-111943) della libreria
  standard, in `crypto/x509`: Go 1.26.9 accettava ancora certificati
  intermedi compromessi. govulncheck lo trova raggiungibile. Il server
  apre connessioni TLS in uscita (provider OIDC, n8n), e la verifica dei
  loro certificati passa da `crypto/x509`. Immagine di build
  `golang:1.26.10-trixie`.

## [0.7.1] - 2026-10-12
Immagine `ghcr.io/labinfinitek/remotek-api:0.7.1`. Correzioni del solo
pannello, nessun cambio incompatibile: nessuna migrazione del database,
nessuna variabile nuova, rotte e contratto col client invariati. Escono le
pagine e i pulsanti del web client, che non funzionavano dalla 0.1.0;
"Esci" chiude la sessione anche sul server (REM-2026-007). Dalla 0.6.0 si
puo' passare direttamente alla 0.7.1.

### Rimosso
- Pannello: le pagine "Condivisioni" (in "Area personale" e
  nell'amministrazione) e i pulsanti "Web client" e "Condividi col web
  client" della lista dei PC e della rubrica, che chiamavano rotte tolte
  dall'API gia' nella 0.1.0 (556d6dc) e davano errore o pagine che non ci
  sono piu'. Con loro escono dal pannello le dipendenze `protobufjs` e
  `long` e i componenti di upstream che nessuna rotta raggiungeva.

### Corretto
- Pannello: la pagina dichiara la lingua del pannello (`lang` di `<html>`),
  all'avvio e a ogni cambio dal menu; prima era sempre inglese e il browser
  poteva proporre di tradurre il pannello in italiano.
- Pannello: l'esportazione della trascrizione, quando la risposta non e'
  JSON (per esempio la pagina HTML di un 502 del proxy), mostra "Non e'
  stato possibile esportare la trascrizione: risposta HTTP 502 dal server"
  invece dell'errore di JSON.parse; con un errore dell'API mostra il suo
  messaggio, come prima.

### Sicurezza
- Pannello: "Esci" chiama `POST /api/admin/logout`, che invalida il
  token sul server, e poi lo toglie dal browser; prima lo toglieva solo
  dal browser e il token restava valido fino alla scadenza. Se la chiamata
  non riesce (rete, API che non risponde entro 5 secondi, token gia'
  scaduto) il token si toglie lo stesso e si torna al login. L'uscita dopo
  il cambio della password non lo chiama: il cambio cancella gia' tutti i
  token dell'utente.
- Pannello: il token di sessione non si copia piu' in
  `wc-option:local:access_token` del `localStorage`, la chiave del web
  client che nessuno legge piu', e "Esci" cancella anche le copie lasciate
  dai login di prima: un token valido fino alla scadenza non resta piu' nel
  browser dopo l'uscita.

## [0.7.0] - 2026-10-12
Immagine `ghcr.io/labinfinitek/remotek-api:0.7.0`. Nessun cambio
incompatibile per chi usa l'immagine: nessuna migrazione del database,
nessuna variabile nuova, rotte e contratto col client invariati. Cambia
solo il pannello di amministrazione: il suo sorgente sta nel repo
(ADR-0027), le sue dipendenze npm non hanno piu' avvisi HIGH ne' CRITICAL
e il build dell'immagine si ferma se ne compare uno, ed e' in italiano
(l'inglese si sceglie dal menu). Chi costruisce l'immagine da se' perde
l'argomento `PANNELLO_COMMIT`.

### Modificato
- Pannello in italiano, con l'inglese come seconda lingua: sono le sole due,
  come nell'API, anche per i componenti di element-plus. La lingua iniziale
  e' l'italiano, a meno che dal menu non si sia scelto l'inglese; la lingua
  del browser non conta piu'. I testi che erano fissi in cinese o in inglese
  passano dalle traduzioni.
- Pannello: si costruisce dal sorgente in `pannello/` (ADR-0027), con le
  modifiche di prima ora come commit; il pannello nell'immagine e' identico
  a prima e il build non scarica piu' niente da GitHub.

### Rimosso
- Pannello: le lingue cinese semplificato e tradizionale, coreano, russo,
  francese e spagnolo.
- Argomento di build `PANNELLO_COMMIT`: il commit di rustdesk-api-web non si
  sceglie piu' al build, il sorgente sta in `pannello/`.

### Sicurezza
- Pannello: dipendenze npm senza avvisi HIGH e CRITICAL (`npm audit`, prima
  19 HIGH e 2 CRITICAL). Aggiornati: axios 1.8.2 -> 1.20.0 (con form-data
  4.0.6 e follow-redirects 1.16.1), vue 3.5.13 -> 3.5.43, js-cookie 3.0.5 ->
  3.0.8, lodash e lodash-es 4.17.21 -> 4.18.1, vite 6.3.4 -> 6.4.4 (rollup
  4.64.5, postcss 8.5.29, nanoid 3.3.20, source-map-js 1.2.2, picomatch
  4.0.7), sass 1.78.0 -> 1.105.1 (senza piu' braces e browserslist),
  protobufjs 7.4.0 -> 7.6.6, ora dipendenza diretta con `long`. Tolti
  sass-loader (con webpack, ajv, fast-uri, serialize-javascript), ts-proto e
  qs, che il build non usa. Avvisi chiusi: form-data GHSA-fjxv-7rqg-78g4,
  GHSA-hmw2-7cc7-3qxx; protobufjs GHSA-xq3m-2v4x-88gg, GHSA-66ff-xgx4-vchm,
  GHSA-685m-2w69-288q, GHSA-75px-5xx7-5xc7, GHSA-jvwf-75h9-cwgg,
  GHSA-wcpc-wj8m-hjx6; axios GHSA-35jp-ww65-95wh, GHSA-3g43-6gmg-66jw,
  GHSA-43fc-jf86-j433, GHSA-4hjh-wcwx-xvwj, GHSA-6chq-wfr3-2hj9,
  GHSA-777c-7fjr-54vf, GHSA-hfxv-24rg-xrqf, GHSA-j5f8-grm9-p9fc,
  GHSA-p92q-9vqr-4j8v, GHSA-pf86-5x62-jrwf, GHSA-pmwg-cvhr-8vh7,
  GHSA-q8qp-cvcw-x6jj; vue (@vue/server-renderer) GHSA-g2v6-rqmx-r4w6;
  js-cookie GHSA-qjx8-664m-686j; lodash e lodash-es GHSA-r5fr-rjxr-66jc;
  vite GHSA-fx2h-pf6j-xcff, GHSA-p9ff-h696-f583; rollup GHSA-mw96-cpmx-2vgc;
  postcss GHSA-6g55-p6wh-862q, GHSA-r28c-9q8g-f849; nanoid
  GHSA-28wg-ghj8-5hjv, GHSA-2v37-7h3g-55p8, GHSA-xwg4-73v4-xw9w;
  source-map-js GHSA-68fv-2mgg-jv7q; picomatch GHSA-c2c7-rcm5-vvqj;
  immutable GHSA-v56q-mh7h-f735, GHSA-wf6x-7x77-mvgw, GHSA-xvcm-6775-5m9r;
  braces GHSA-vfj7-8cjw-p6xm; browserslist GHSA-73wf-gq98-2v4g,
  GHSA-c83g-rgw3-j3cx; fast-uri GHSA-4c8g-83qw-93j6, GHSA-7p8r-x3mc-p8w7,
  GHSA-f65p-4m7j-42xc, GHSA-jqff-g426-hqxp, GHSA-q3j6-qgpj-74h6,
  GHSA-qw65-cvwx-89v3, GHSA-v2hh-gcrm-f6hx, GHSA-v39h-62p7-jpjc;
  serialize-javascript GHSA-5c6j-r48x-rmvq. Resta element-plus 2.8.2 con
  GHSA-5m5x-9j46-h678 (MODERATE).
- Pannello: `npm audit --audit-level=high` nel build dell'immagine, dopo
  `npm ci`: un avviso HIGH o CRITICAL ferma il build (REGOLE sez. 9). Il
  lockfile risolve tutto sul registry ufficiale e non piu' su
  registry.npmmirror.com, e il build si ferma se un `resolved` punta
  altrove; la versione di node sta in `pannello/.nvmrc` e il build si ferma
  se l'immagine ne ha un'altra.

## [0.6.0] - 2026-10-11
Immagine `ghcr.io/labinfinitek/remotek-api:0.6.0`. Nessun cambio
incompatibile. Due migrazioni del database, fatte all'avvio: 271 (tabella
`eventi`, la coda degli eventi) e 272 (tabella `token_integrazione`).
Entra l'integrazione con n8n e Zammad (ADR-0026): eventi firmati verso un
solo URL, spenti finche' `integrazione.url` e' vuoto, e token di
integrazione in sola lettura. Senza configurazione l'API si comporta come
la 0.5.0. Entrano anche i seguiti della firma e delle copie: una riga INFO
`chiave del PC registrata` quando un PC passa alla firma.

### Aggiunto
- Integrazione (eventi): con `integrazione.url` (`https://`) e
  `integrazione.segreto` (almeno 32 caratteri) l'API manda un evento
  firmato (HMAC-SHA256, header `X-Remotek-Evento-Firma`) per apertura,
  chiusura e nota delle connessioni, da una coda nel database (migrazione
  271, tabella `eventi`) con tentativi per 24 ore e una riga ERROR
  `evento non consegnato` dopo. Senza URL non cambia niente; URL non
  `https://`, con credenziali o segreto corto fermano l'avvio. Mai IP,
  uuid, chiavi, token o trascrizioni (README, Integrazione).
- Integrazione (eventi): anche `file.trasferiti` (direzione e numero, senza
  nomi ne' percorsi), `terminale.chiuso` (esito della verifica e hash
  finale, senza dati), `pc.nuovo` e `pc.possibile_copia` (numero di cambi,
  senza indirizzi).
- Token di integrazione in sola lettura: `./apimain token-integrazione
  crea|elenco|revoca` (nel database solo lo SHA-256, scadenza un anno,
  migrazione 272) e rotte `GET /api/integrazione/pc`, `/connessioni` e
  `/connessioni/{id}/verifica`, che accettano solo quel token (README,
  Integrazione).

### Corretto
- Copie dello stesso PC: una copia rimasta zitta piu' di 10 minuti non ha
  piu' un secondo warn `possibile copia del PC` prima dell'ora dal primo.
- Firma del dispositivo: un sysinfo firmato con un uuid diverso da quello
  salvato non fa verificare la firma ne' la mette nella memoria delle firme
  viste; per un PC creato dal pannello senza uuid il warn dice "PC senza
  uuid, in attesa del primo sysinfo". Una chiave registrata scrive nel log
  una riga info `chiave del PC registrata` con l'ID del PC.

## [0.5.0] - 2026-10-10
Immagine `ghcr.io/labinfinitek/remotek-api:0.5.0`, costruita con Go 1.26.9.
Nessun cambio incompatibile. Quattro migrazioni del database, fatte
all'avvio: 267 (tabella `audit_terminals`), 268 (`users.agente_di`), 269
(`audit_terminals.totale`), 270 (`peers.chiave_pubblica`). Entrano la
trascrizione delle sessioni terminale con l'esportazione, gli account degli
agenti AI, il limite di 1 MiB al corpo delle richieste, la firma del
dispositivo, non obbligatoria (`app.firma-obbligatoria` = `false`: i
client che non firmano funzionano come prima), e la segnalazione delle
copie dello stesso PC nel log e nel pannello. Serve al client
`remotek-1.4.9-3`, che firma e manda le trascrizioni. Dopo l'aggiornamento
vanno accesi gli allarmi sui log della firma (ADR-0023) e delle copie
(ADR-0024).

### Aggiunto
- Segnalazione delle copie di un PC (VM clonata o immagine disco senza
  sysprep, che porta ID, uuid e chiave): con almeno 10 cambi di indirizzo
  negli ultimi 10 minuti fra heartbeat a meno di 60 secondi l'uno
  dall'altro, il PC ha `possibile_copia` vero in
  `GET /api/admin/peer/list`, il pannello mostra "Possible copy" accanto
  all'ID con cosa fare, e il log ha un warn `possibile copia del PC` al
  massimo una volta all'ora. Nessuna richiesta bloccata, rotte del client
  invariate. Regola e limiti nel README, Copie dello stesso PC.
- Opzione `app.firma-obbligatoria` (`RUSTDESK_API_APP_FIRMA_OBBLIGATORIA`,
  default `false`): se vera, anche un PC senza chiave registrata deve
  firmare le richieste senza login, e il suo sysinfo firmato con `pk` e
  uuid giusto registra la chiave. Si accende quando tutti i PC hanno un
  client che firma. README, Firma del dispositivo.
- Esportazione della trascrizione di una sessione terminale, solo per gli
  amministratori: `GET /api/admin/audit_conn/terminal/export` con
  `audit_conn_id` scarica `trascrizione-<id>.json` con la riga della
  connessione, i blocchi in ordine e la verifica calcolata sugli stessi
  blocchi, letti a pagine: il database non resta occupato durante il
  download. Formato e ricalcolo della catena nel README.
- Account degli agenti AI: campo `agente_di` dell'utente (il tecnico che ne
  risponde), comando `agente-ai <username> <tecnico>` che crea l'agente e
  stampa una volta la sua password, regole nel pannello (il tecnico e' una
  persona, un agente non e' amministratore, disattivare il tecnico
  disattiva i suoi agenti, un tecnico con agenti non si cancella) e
  `GET /api/agente` per il client a riga di comando; la sincronizzazione
  LDAP non rende amministratore un agente. Migrazione del database:
  versione 268, colonna `users.agente_di`.
- Trascrizione del terminale nel registro delle connessioni del pannello: la
  colonna Type mostra "Terminal" per le sessioni terminale, e il pulsante
  "Transcript" apre in sola lettura i blocchi in ordine con l'esito della
  verifica (integra o no, hash finale); "No transcript" se non ci sono
  blocchi. Il pulsante "Export" della finestra scarica il file della
  trascrizione (`trascrizione-<id>.json`); un errore resta scritto nella
  finestra. Un carattere UTF-8 troncato in coda all'ultimo blocco di un
  verso si vede come U+FFFD invece di sparire.
- Trascrizione delle sessioni terminale: `POST /api/audit/terminal` salva i
  blocchi mandati dal PC controllato, concatenati da SHA-256, e solo per
  gli amministratori `GET /api/admin/audit_conn/terminal/list` li elenca e
  `GET /api/admin/audit_conn/terminal/verify` ricalcola la catena, con
  l'id della connessione. Ogni blocco si lega alla connessione piu'
  recente del PC con quel `conn_id`. Formato e controlli nel README. Cancellare una connessione terminale dal pannello
  cancella anche la sua trascrizione. Migrazione del database: versione
  267, tabella `audit_terminals`.

### Corretto
- Trascrizione delle sessioni terminale: il controllo dei 20 MiB legge il
  totale salvato nell'ultimo blocco invece di risommare tutta la sessione
  a ogni blocco; un blocco vuoto si accetta solo con `fine` e una sessione
  ha al massimo 100000 blocchi. Migrazione del database: versione 269,
  colonna `audit_terminals.totale`.
- Registro delle connessioni: chiusura, autenticazione e tipo di una
  connessione vanno sulla riga piu' recente del PC con quel `conn_id`.
  Il client fa ripartire `conn_id` a ogni avvio del servizio, e prima
  finivano sulla riga piu' vecchia con lo stesso numero, lasciando aperta
  quella nuova.

### Sicurezza
- Go 1.26.9 e `golang.org/x/net` v0.60.0: govulncheck trova raggiungibili
  dal server dell'API dodici avvisi della libreria standard di Go 1.26.8
  (`net/http` e HTTP/2, `crypto/tls`, `net/textproto`, `html/template`; da
  GO-2026-6599 a GO-2026-6617), cinque dei quali anche in `x/net` v0.57.0.
  Immagine di build `golang:1.26.9-trixie`.
- Corpo delle richieste al massimo 1 MiB su ogni rotta, anche quelle senza
  login (prima nessun limite: un corpo grande si leggeva tutto in memoria).
  Con `Content-Length` oltre il limite risposta 413 senza leggere il
  corpo (nel pannello nella forma delle sue risposte); senza, la lettura
  si ferma a 1 MiB e il bind fallisce come per un corpo sbagliato. Un
  warn nel log con metodo, rotta e `request_id`.
- Firma del dispositivo (ADR-0023): sysinfo, heartbeat e audit di un PC
  si riconoscevano solo da ID e uuid, che il client manda anche a `hbbs` in
  chiaro. Ora il PC puo' firmarli con la sua chiave Ed25519 (intestazione
  `X-Remotek-Firma`, campo `pk` nel sysinfo): il primo sysinfo firmato
  registra la chiave, e da li' per quel PC valgono solo richieste firmate
  con quella chiave, recenti (300 secondi) e non ripetute. I client che non
  firmano, come il 1.4.9, restano con le regole dell'uuid. Una chiave non
  la cambia nessun sysinfo: si riapre cancellando il PC dal pannello. Una
  firma con una chiave diversa e l'uuid giusto scrive un error nel log.
  Formato e regole nel README. Migrazione del database: versione 270,
  colonna `peers.chiave_pubblica`.

## [0.4.0] - 2026-09-29
Immagine `ghcr.io/labinfinitek/remotek-api:0.4.0`. Un cambio incompatibile,
sotto Modificato: il log esce solo in JSON su stdout, `logger.path` e'
ignorata e `logger.level` accetta solo `debug`, `info`, `warn`, `error`. Il
pannello nell'immagine e' modificato da patch (`pannello/`): nota di
sessione visibile, comandi del server e esportazioni CSV corretti. Ogni
risposta ha l'header `X-Request-Id`. Nessuna migrazione del database.

### Aggiunto
- Header `X-Request-Id` in ogni risposta, con un id casuale scelto dall'API
  (quello mandato dal chiamante non si usa), e lo stesso id come
  `request_id` nelle righe di log che cominciano con metodo e rotta (errori
  mandati al client, avvisi dei gestori, panic) e nella riga di debug per
  richiesta; le altre righe, come i `Login Fail`, quelle dei servizi e
  quelle SQL, non lo hanno. A debug, una riga per richiesta con
  `request_id`, metodo, rotta, stato e durata; quella di prima, con URL e
  IP, non c'e' piu'.
- Colonna "Remark" nel registro delle connessioni del pannello, con la nota
  di sessione del tecnico; il testo lungo si legge nel tooltip. Il pannello
  nell'immagine e' ora rustdesk-api-web modificato dalle patch in
  `pannello/`, applicate dal `Dockerfile`; il suo `LICENSE` e' nell'immagine
  in `resources/admin/LICENSE`.

### Corretto
- `reset-admin-pwd`, `reset-pwd` o un comando sconosciuto con gli argomenti
  sbagliati escono con codice 1 e il messaggio su stderr: prima, dopo il
  messaggio, il processo andava in panic e usciva con 2.
- Comandi del server nel pannello: "Aggiungi" dopo "Modifica" crea un
  comando nuovo invece di riscrivere quello appena modificato. L'API, in
  `POST /api/admin/rustdesk/cmdCreate`, ignora l'id del corpo: prima un id
  esistente faceva fallire il create e uno libero dava la riga con l'id
  scelto.
- Comandi del server nel pannello: `cmdCreate` e `cmdUpdate` rifiutano un
  target diverso da hbbs (21115) o hbbr (21117), anche vuoto, con la stessa
  risposta di `sendCmd` ("Parametri non validi."). Prima il comando si
  salvava e poi non si poteva mandare.

### Modificato
- Log in JSON su stdout con `log/slog`, una riga per evento con `time`,
  `level` (`DEBUG`, `INFO`, `WARN`, `ERROR`), `msg` e, con
  `logger.report-caller`, `source`; prima testo `[AAAA-MM-GG hh:mm:ss]
  [WARN] ...`. Le righe SQL lente o in errore di gorm escono a warn, senza
  codici colore; il panic di una richiesta esce a error con metodo, rotta e
  stack, senza gli header della richiesta.
- **Rottura:** niente piu' file di log: `logger.path` (prima
  `./runtime/log.txt`, fuori dal volume dei dati e fuori dalla rotazione, con
  nomi utente e IP) e' ignorata, e se impostata all'avvio un warn lo dice.
  `logger.level` accetta solo `debug`, `info`, `warn`, `error` (vuoto =
  `info`): `trace`, `fatal`, `panic` o un valore non valido fermano l'avvio,
  dove prima un valore non valido valeva `debug`.

### Sicurezza
- Esportazioni CSV del pannello (connessioni, file, utenti, dispositivi, i
  miei dispositivi, login) senza formule: un `=`, `+`, `-`, `@`, tab o
  ritorno a capo (`\r`) all'inizio di una cella, o dopo un `;`, una `,` o
  un a capo (`\n` o `\r`) del valore, anche dopo virgolette o spazi (Excel
  in italiano separa le celle ai `;` e le righe agli a capo, anche dentro
  le virgolette), prende davanti un apostrofo, e non diventa una formula
  quando il file si apre in un foglio di calcolo. I numeri puri e il
  segnaposto `-` restano come sono. Note di sessione, nomi dei PC, alias e
  note degli utenti sono testo libero. Un valore vuoto (null) non ferma
  piu' l'esportazione, e un elenco vuoto da' un file vuoto invece di un
  errore.
- Login OIDC: un `id_token` senza `sub` non fa entrare. Prima, se il `sub`
  mancava anche nella userinfo, il confronto passava e l'associazione al
  provider nasceva con l'account vuoto, lo stesso per ogni utente di quel
  provider. Il login si ferma come per un `sub` diverso da quello della
  userinfo: la pagina dice OauthFailed e il motivo va nel log.

## [0.3.0] - 2026-09-29
Immagine `ghcr.io/labinfinitek/remotek-api:0.3.0`. Due cambi incompatibili,
sotto Sicurezza: `ldap.tls-verify` vale `true` anche senza configurazione, e
chi usa `ldaps://` con una CA interna deve indicarla con `ldap.tls-ca-file`;
il login OIDC richiede un `id_token` valido, e un provider che non lo manda
non fa piu' entrare.
Le rotte del client senza login accettano dati solo dal PC registrato con
quell'ID (REM-2026-002): un PC che cambia uuid si sblocca cancellandolo dal
pannello mentre e' acceso (Sicurezza). Al primo avvio il database passa alla
versione 266 (nota di sessione nel registro delle connessioni) e `peers.id`
riceve l'indice unico, se non ci sono ID doppi (Corretto).

### Aggiunto
- Note di sessione del tecnico nel registro delle connessioni, che l'API
  prima perdeva. La nota che il client manda durante la sessione si salva
  sulla connessione gia' registrata con quell'ID e quel `session_id`, senza
  crearne di nuove; la nota di fine connessione (opzione del client
  `allow-ask-for-note`, spenta di fabbrica) passa da
  `GET /api/audit/conn/active` e `PUT /api/audit`, col login del tecnico e
  un guid casuale della connessione. Con `session_id` 0 o assente, quello
  delle connessioni non ancora autorizzate, la nota non si salva e il guid
  non si da'. Al massimo 2000 caratteri, oltre si tronca; il testo della
  nota non va mai nel log. La nota si legge nel JSON di
  `/api/admin/audit_conn/list` e nell'esportazione CSV del pannello: il
  pannello (rustdesk-api-web `3998c2a`) non ha una colonna per la nota.

### Sicurezza
- Login OIDC: l'`id_token` nella risposta del token e' obbligatorio (OIDC
  Core 3.1.3.3). Senza, il login si ferma come per un `id_token` non valido
  (la pagina dice `OauthFailed`), con un warn nel log che dice di controllare
  che gli scope comprendano `openid`; il nonce dell'`id_token` si confronta
  sempre, e il `sub` della userinfo deve essere quello dell'`id_token` (OIDC
  Core 5.3.2). `openid` si chiede sempre al provider, anche se gli scope
  salvati non lo hanno. Prima, senza `id_token`, firma e nonce non si
  verificavano e l'account si associava col solo `sub` della userinfo.
  **Rottura:** un provider che non manda l'`id_token` non fa piu' entrare
  nessuno; va configurato perche' lo mandi.
- Pannello, "le mie rubriche" (`/api/admin/my/address_book_collection/update`):
  la rubrica resta di chi la modifica, e lo `user_id` nel corpo non conta.
  Prima un utente del pannello non amministratore poteva passare una sua
  rubrica, con le voci dentro, a un altro utente, che se la trovava tra le
  sue rubriche nel client.
- Pannello, "i miei dispositivi" e "i miei login"
  (`/api/admin/my/peer/list`, `/api/admin/my/login_log/list`): l'uuid dei
  dispositivi resta vuoto. Dal legame ID-uuid (REM-2026-002) l'uuid autorizza
  a scrivere scheda, heartbeat e audit di un PC, e un utente del pannello non
  amministratore lo leggeva dei PC legati a lui. Le liste di amministrazione
  (`/api/admin/peer/list`, `/api/admin/login_log/list`) lo mostrano ancora.
- Pannello, comandi del server (`/api/admin/rustdesk/sendCmd`, `cmdList`,
  `cmdCreate`, `cmdDelete`): solo per gli amministratori (REM-2026-004). A
  un utente del pannello non amministratore rispondono come le altre rotte
  di amministrazione, "Non hai i permessi per questa operazione.", e non
  mandano niente a hbbs e hbbr ne' cambiano i comandi salvati. Prima
  qualunque utente del pannello poteva mandare a hbbs e hbbr i comandi di
  gestione (server relay, blocco degli IP, `always-use-relay`, blacklist,
  limiti di banda) e creare o cancellare i comandi salvati.
- Rotte del client senza login (`/api/sysinfo`, `/api/heartbeat`,
  `/api/audit/conn`, `/api/audit/file`): accettano dati solo dal dispositivo
  registrato con quell'ID (REM-2026-002). Il primo sysinfo di un ID lega
  l'ID all'uuid del PC, un PC creato dal pannello si lega al primo uuid che
  arriva; poi un sysinfo con un altro uuid non cambia la scheda e risponde
  400 "Il dispositivo non corrisponde a quello registrato.", heartbeat e
  audit con un altro uuid o da un ID sconosciuto non scrivono niente e
  rispondono come prima; nel log un warn con rotta e ID del PC. Un sysinfo
  senza uuid non crea ne' aggiorna niente. Prima chi conosceva l'ID di un PC
  ne sovrascriveva la scheda e scriveva nel registro delle connessioni e dei
  file. **Per chi gestisce i PC**: un PC che cambia uuid (Windows
  reinstallato, macchina sostituita) non si aggiorna piu' e resta con
  l'ultimo contatto fermo; si sblocca cancellandolo dal pannello mentre e'
  acceso (per esempio quando nel log arriva il warn col suo ID, che
  l'heartbeat manda ogni 15 secondi). Con il PC acceso e il servizio Remotek
  attivo il client riprova da solo il sysinfo rifiutato circa ogni 2 minuti
  e il primo tentativo dopo la cancellazione lo ricrea col nuovo uuid; fino
  ad allora (al massimo circa 2 minuti) l'ID lo prende il primo che manda un
  sysinfo con quell'ID. Con il PC spento o il servizio fermo nessuno
  riprova, e l'ID resta libero finche' il PC non torna in linea; riavviare
  subito il servizio Remotek sul PC (o il PC) chiude prima la finestra. Un
  PC cancellato mentre era legato al suo uuid attuale va invece riavviato
  subito: il client rimanda il sysinfo solo all'avvio del servizio o quando
  cambiano l'utente di Windows, l'ID o l'indirizzo dell'API, e fino al
  riavvio l'ID resta libero senza limite di tempo. L'audit salva il
  `session_id` esatto; prima, sopra 2^53, lo arrotondava.
- LDAP: `ldap.tls-verify` vale `true` anche senza configurazione, e con
  `ldaps://` il certificato del server si verifica; prima valeva `false` e
  un server qualsiasi poteva ricevere le password del bind e degli utenti.
  **Rottura**: chi usa `ldaps://` con un certificato che il sistema non
  riconosce (CA interna, autofirmato) deve indicarne la CA con
  `ldap.tls-ca-file` (`RUSTDESK_API_LDAP_TLS_CA_FILE`), o scegliere
  `ldap.tls-verify: false`; altrimenti il login LDAP non riesce e si prova
  l'utente locale. Con LDAP acceso, all'avvio una riga di warn segnala
  `ldap://` (password in chiaro sulla rete) e `ldap.tls-verify: false` con
  `ldaps://`. Nel `conf/config.yaml` d'esempio `bind-password` e' vuota.

### Corretto
- `/api/login-options` senza provider OIDC e con `app.web-sso` spento
  risponde `["common-oidc/[]"]` invece di `["common-oidc/null"]`. Per il
  client 1.4.9 non cambia niente (nessun pulsante in entrambi i casi): prima
  la lista vuota ci arrivava solo passando dal ramo d'errore.
- Un ID di PC compare una volta sola nei dispositivi. All'avvio l'API crea
  l'indice unico `idx_peers_id_unico` su `peers.id`; il pannello non crea
  ne' salva un dispositivo senza ID o con l'ID di un altro ("Un altro PC ha
  già questo ID."). Prima un ID poteva stare su piu' righe, e il legame
  ID-uuid valeva per la prima. Chi ha ID doppi nel database trova a ogni
  avvio nel log la riga error `peers ha N ID di PC su piu' righe: l'indice
  unico idx_peers_id_unico non si crea ...`: l'API parte lo stesso, senza
  l'indice; dal pannello, in Dispositivi, si cercano gli ID doppi (anche
  vuoti) e si cancellano le righe in piu', e al riavvio l'indice si crea.
  Nessuna riga si cancella da sola.
- Pannello: la modifica di un dispositivo, di un utente, di un provider
  OAuth o di un comando del server salva vuoti i campi del modulo che
  arrivano vuoti. Prima il pannello rispondeva successo ma quei campi
  restavano com'erano. Dal pannello di oggi (rustdesk-api-web `3998c2a`)
  si svuotano alias, nome del PC, utente del PC, CPU, memoria, OS e
  versione del dispositivo; email, nickname e nota dell'utente; scope
  del provider; alias, opzione e spiegazione del comando. Gruppo del
  dispositivo, avatar dell'utente e issuer del provider quel pannello
  non li manda vuoti: il gruppo e' una scelta senza voce vuota, l'avatar
  non e' nel modulo dell'utente, l'issuer e' obbligatorio. Non cambiano:
  l'uuid del dispositivo mandato vuoto resta quello di prima, come la
  password dell'utente e il ruolo se `is_admin` non c'e'; utente
  assegnato, ultimo contatto e IP del dispositivo non sono nel modulo e
  non si toccano. Il pannello (rustdesk-api-web `3998c2a`) manda l'alias
  del dispositivo solo se l'utente tocca quel campo: se nel corpo manca,
  l'alias resta quello di prima. Gruppi, gruppi di dispositivi,
  collezioni e regole della rubrica non avevano il problema: il
  validatore non accetta vuoti i loro campi.
- File d'esempio `conf/config.yaml`: `rustdesk.id-server`,
  `rustdesk.relay-server` e `rustdesk.api-server` sono vuoti, da
  configurare, invece di `192.168.1.66:21116`, `192.168.1.66:21117` e
  `http://127.0.0.1:21114` di upstream; chi usa il file cosi' com'e' vede
  nel pannello i campi vuoti invece di un indirizzo sbagliato. All'avvio un
  warn per `id-server` vuoto (un client senza server ID usa i server
  pubblici di RustDesk) e uno per `api-server` vuoto (login OIDC e
  `webauth` senza host); si impostano con `RUSTDESK_API_RUSTDESK_*`.
- Pannello, comandi del server: la modifica di un comando salvato riesce.
  Il pannello la manda a `/api/admin/rustdesk/cmdUpdate`, che non era
  registrata e rispondeva 404; ora c'e', solo per gli amministratori come
  le altre rotte dei comandi.
- Rubriche condivise del client (`/api/ab/shared/profiles`): una rubrica
  condivisa il cui proprietario non c'e' piu' nel database (dati rimasti
  orfani prima della cancellazione a cascata degli utenti) si salta, con
  una riga di warn nel log, e le altre restano nella risposta. Prima la
  rotta andava in panic e rispondeva 500, e il client 1.4.9 toglieva
  dall'elenco tutte le rubriche condivise dell'utente.
- Cancellazione in blocco dei dispositivi dal pannello
  (`/api/admin/peer/batchDelete`): se gli uuid dei dispositivi non si
  leggono dal database non cancella niente e risponde "Operazione non
  riuscita.", con l'errore nel log; dispositivi e token di sessione dei loro
  uuid si cancellano in una transazione. Prima cancellava i dispositivi e
  rispondeva successo, e se i token non si cancellavano i dispositivi
  restavano cancellati: i token dei dispositivi cancellati restavano validi.
- Login OIDC con autoregistrazione: se l'utente del provider ha l'email di
  un utente locale e l'associazione al provider non si salva, la pagina
  dice "Registrazione con OAuth non riuscita." e il login non si lega
  all'utente; prima riusciva senza associazione. Un errore del database
  nel leggere l'associazione, il suo utente o l'utente con la stessa email
  ferma il login con "Autorizzazione OAuth non riuscita." e va nel log:
  prima valeva "non trovato", e nasceva un utente doppio o la pagina diceva
  successo senza login.
- Login del client (`/api/login`, `/api/oidc/auth-query`) e del pannello
  (login e registrazione): se il token di sessione o la riga del registro
  degli accessi non si salvano, la risposta e' d'errore ("Operazione non
  riuscita.", "Accesso non riuscito." nel login OIDC) e l'errore va nel
  log; token e registro si salvano in una transazione. Prima la risposta
  dava un token che il database non aveva, e il client usciva alla prima
  richiesta (401), o lasciava il login fuori dal registro. Se il
  dispositivo del login non si lega all'utente il login vale lo stesso e
  l'errore va nel log.
- Logout del pannello (`/api/admin/logout`) col token di un dispositivo: se
  il token non si legge o il dispositivo non si scollega dall'utente,
  risponde "Operazione non riuscita." e l'errore va nel log; prima
  rispondeva successo e il dispositivo restava dell'utente.
- Il rinnovo della scadenza del token di sessione, che le rotte del client
  e del pannello fanno quando al token manca meno di un terzo di
  `app.token-expire`, se non si salva va nel log a livello warn, con
  metodo e rotta; la richiesta va avanti come prima. Prima l'errore si
  perdeva.
- LDAP: un bind rifiutato (dell'account di servizio o di un utente) e un
  `userAccountControl` non numerico vanno nel log, col motivo (warn e
  error; per `userAccountControl` anche il nome dell'utente), non piu' su
  stdout senza motivo; anche l'errore di chiusura della connessione dopo il
  bind rifiutato va nel log.
- Login OIDC: le richieste del callback al provider (scambio del codice,
  chiavi dell'id_token, userinfo) hanno 30 secondi in tutto; un provider
  che non risponde fa dire alla pagina "Autorizzazione OAuth non
  riuscita.", con l'errore nel log, invece di tenerla ferma senza limite:
  senza proxy il client HTTP non aveva timeout.
- Comandi al server rustdesk dal pannello: connessione, invio e risposta
  hanno 5 secondi per indirizzo (IPv6, poi IPv4); un server che accetta la
  connessione e non risponde fa rispondere "Operazione non riuscita."
  invece di tenere ferma la richiesta senza limite.
- Audit del client (`/api/audit/conn`, `/api/audit/file`): se il database
  non salva la connessione o il trasferimento di file, l'errore va nel log
  a livello error, con metodo e rotta; la risposta resta quella di sempre,
  che il client ignora. Prima l'errore si perdeva.
- Heartbeat del client (`/api/heartbeat`): se il database non aggiorna
  l'ultimo contatto del dispositivo, l'errore va nel log a livello warn,
  con metodo e rotta; la risposta resta `{}`. Prima si perdeva.
- Logout del client (`/api/logout`): se il token o il dispositivo non si
  aggiornano nel database, risponde 400 "Operazione non riuscita." e
  l'errore va nel log; prima rispondeva 200 `null` col token ancora
  valido. Il client 1.4.9 non legge la risposta del logout ed esce
  comunque; nel caso normale la risposta resta `200 null`.
- Rubrica del client: se il database non legge voci, tag o rubriche,
  `GET /api/ab`, `/api/ab/peers`, `/api/ab/tags/{guid}` e
  `/api/ab/shared/profiles` rispondono 400 "Errore di sistema." e l'errore
  va nel log. Prima rispondevano 200 con le liste vuote: il client 1.4.9
  mostrava la rubrica vuota senza avvisi e la salvava nella cache, un
  client legacy la svuotava e poi la rimandava vuota con `POST /api/ab`,
  che la cancellava dal database. Nel pannello gli elenchi di voci, tag e
  rubriche rispondono "Errore di sistema." invece di un elenco vuoto, e il
  cambio dei tag di piu' voci invece di "Elemento non trovato.".
- Rubrica condivisa del client: se il database non legge le regole dei
  permessi, le rotte rispondono 400 "Errore di sistema." con l'errore nel
  log, e non concedono niente; prima rispondevano "Non hai i permessi per
  questa operazione." e l'errore si perdeva.
- Rubrica legacy del client (`POST /api/ab`): cambia solo la rubrica
  personale, la sola che `GET /api/ab` manda; prima cancellava anche le
  voci e i tag delle altre rubriche dell'utente, che il client legacy non
  vede.
- Tag della rubrica del client: se il database non legge il tag cercato
  per nome, aggiunta, rinomina, colore e cancellazione rispondono 400
  "Errore di sistema." con l'errore nel log, e non cambiano niente. Prima
  la lettura fallita valeva "il tag non c'e'": l'aggiunta creava un tag
  doppio, la rinomina rinominava su un nome gia' usato, le altre
  rispondevano "Elemento non trovato.".
- Voci della rubrica: se il database non legge la voce, cancellazione e
  modifica dal client rispondono 400 "Errore di sistema." invece di
  "Elemento non trovato.", con l'errore nel log. Nel pannello la
  creazione di una voce, anche in blocco o dai dispositivi, risponde
  "Errore di sistema." e non crea niente; prima creava un doppione.
- Rubriche diverse dalla personale: se il database non legge la rubrica,
  le rotte del client su di essa rispondono 400 "Errore di sistema."
  invece di "Parametri non validi.", e nel pannello dettaglio, modifica e
  cancellazione della rubrica e le voci create dai dispositivi rispondono
  "Errore di sistema." invece di "Elemento non trovato.", con l'errore nel
  log.
- Regole di condivisione delle rubriche nel pannello: se il database non le
  legge, le rotte rispondono "Errore di sistema." con l'errore nel log.
  Prima gli elenchi (amministrazione e sezione dell'utente) erano vuoti,
  dettaglio, modifica e cancellazione di una regola rispondevano "Elemento
  non trovato.", e creazione e modifica prendevano la lettura fallita per
  "nessuna regola uguale" e salvavano la regola doppia.
- Pannello, voci, tag e regole in una rubrica diversa dalla personale: se il
  database non legge la rubrica scelta, creazione e modifica rispondono
  "Errore di sistema." con l'errore nel log, e non salvano niente come
  prima; prima rispondevano "Parametri non validi.", come a chi sceglie la
  rubrica di un altro, e l'errore si perdeva.
- Pannello, voci e tag della rubrica: se il database non legge la voce o il
  tag, modifica e cancellazione (amministrazione e sezione dell'utente), e
  il dettaglio di un tag, rispondono "Errore di sistema." con l'errore nel
  log, invece di "Elemento non trovato.".
- Rotte autenticate del client e del pannello: se il database non legge il
  token di sessione o il suo utente, il client riceve 500 "Errore di
  sistema." e il pannello "Errore di sistema." (code 101), con l'errore nel
  log. Prima il client riceveva 401 e il pannello code 403, che per tutti e
  due sono un logout: un errore del database faceva uscire il tecnico. Un
  token che non c'e', scaduto o di un utente disabilitato ha le risposte di
  prima. Anche la configurazione del pannello (`/api/admin/config/admin`)
  con un token risponde "Errore di sistema." invece del solo titolo.
- Login del client (`/api/login`) e del pannello: se il database non legge
  l'utente, la risposta e' "Errore di sistema." con l'errore nel log, e il
  tentativo non conta per captcha e ban. Prima valeva una password
  sbagliata: col database in difficolta' dieci tentativi
  (`app.ban-threshold`) bloccavano per 30 minuti l'IP di tecnici con la
  password giusta. Lo stesso con LDAP acceso, quando il database non legge
  l'utente locale di chi la directory ha autenticato.
- Utenti e dispositivi del gruppo nel client (`/api/users`, `/api/peers`) e
  rubriche condivise (`/api/ab/shared/profiles`): se il database non legge
  gli utenti rispondono 400 "Errore di sistema." con l'errore nel log. Prima
  i primi due rispondevano 200 con l'elenco vuoto, e le rubriche condivise
  500 senza corpo (un panic). Nel pannello l'elenco degli utenti risponde
  "Errore di sistema." invece di un elenco vuoto.
- Utente letto per id: se il database non lo legge, le rotte del client
  sulla rubrica di un altro utente rispondono 400 "Errore di sistema."
  invece di "Parametri non validi.", `/api/oidc/auth-query` risponde 400
  "Errore di sistema." invece di dare al client il token di un utente
  vuoto, che lo faceva uscire alla prima richiesta, e il callback OIDC
  dice "Autorizzazione OAuth non riuscita." invece di associare lo stesso
  l'account o, con l'autoregistrazione spenta, di rimandare ad associarlo
  di nuovo. Nel pannello dettaglio, cancellazione, cambio della password e
  modifica di un utente, e le regole di condivisione verso un utente,
  rispondono "Errore di sistema." invece di "Elemento non trovato." (la
  modifica andava in panic). L'errore va nel log. Un utente che non c'e'
  ha le risposte di prima, tranne `/api/oidc/auth-query`, che risponde
  "Utente non trovato.", e il callback, che dice "Elemento non trovato.":
  prima davano il token di un utente vuoto e associavano l'account a un
  utente che non c'e'.
- Login OIDC: se il database non legge l'associazione dell'account al
  provider, la pagina del callback dice "Autorizzazione OAuth non
  riuscita." con l'errore nel log. Prima l'associazione chiesta dal
  pannello si salvava lo stesso, anche per un account gia' associato a un
  altro utente, e il login con l'autoregistrazione spenta rimandava ad
  associare l'account.
- `/api/sysinfo`: se il database non legge l'ultimo login del dispositivo,
  l'errore va nel log a livello warn, con metodo e rotta; il dispositivo si
  salva senza utente e la risposta resta `SYSINFO_UPDATED`, come prima.
  Prima l'errore si perdeva.
- Pannello, cancellazione e modifica di un amministratore: se il database
  non conta gli amministratori, rispondono "Errore di sistema." con
  l'errore nel log, e non cancellano ne' declassano nessuno. Prima la
  lettura fallita valeva zero amministratori: la risposta era "Operazione
  non riuscita.", come per l'ultimo amministratore, e l'errore si perdeva.
- Registrazione con OIDC e creazione di un utente dal pannello: se il
  database non dice se il nome utente e' preso, la registrazione si ferma
  e la pagina del callback dice "Autorizzazione OAuth non riuscita.", e la
  creazione risponde "Errore di sistema.", con l'errore nel log; nessun
  utente nasce. Prima la lettura fallita valeva "nome libero".
- Pannello, token di sessione (`/api/admin/user_token`): se il database non
  li legge, l'elenco e la cancellazione rispondono "Errore di sistema." con
  l'errore nel log. Prima l'elenco era vuoto e la cancellazione diceva
  "Elemento non trovato.".
- Pannello, associazioni dell'utente ai provider OAuth: se il database non
  le legge, l'elenco dei provider dell'utente, l'associazione e lo
  scollegamento rispondono "Errore di sistema." con l'errore nel log. Prima
  l'elenco mostrava i provider come non associati, l'associazione partiva
  anche se c'era gia', e lo scollegamento diceva "Elemento non trovato.".
- Provider OAuth: se il database non li legge, `/api/oidc/auth` risponde
  400 "Errore di sistema." invece di "Configurazione non trovata.", il
  callback OIDC dice "Autorizzazione OAuth non riuscita." invece di andare
  in panic (500) quando non legge il provider per l'autoregistrazione, e
  nel pannello elenco, dettaglio, creazione e cancellazione dei provider, e
  l'elenco dei provider dell'utente, rispondono "Errore di sistema.";
  l'errore va nel log. Prima l'elenco era vuoto, dettaglio e cancellazione
  dicevano "Elemento non trovato.", e la creazione salvava il provider
  senza sapere se ce n'era gia' uno con lo stesso op.
- Login OIDC di un account senza utente, con un provider salvato senza
  `auto_register` (NULL nel database): l'autoregistrazione vale spenta e
  il callback rimanda ad associare l'account dal pannello; prima andava in
  panic (500). Il pannello, quando crea o aggiorna un provider senza
  `auto_register`, la salva spenta, come gia' `pkce_enable`.
- Informazioni di sistema del client (`/api/sysinfo`): se il database non
  legge il dispositivo risponde 400 "Errore di sistema." e non crea
  niente, con l'errore nel log; il client riprova piu' tardi. Prima lo
  prendeva per nuovo e ne creava un altro con lo stesso id, un doppione.
  L'heartbeat (`/api/heartbeat`) risponde come sempre, ma l'errore va nel
  log; prima l'ultimo contatto non si aggiornava senza traccia.
  L'aggiunta di una voce in rubrica (`/api/ab/peer/add`) risponde 400
  "Errore di sistema." e non crea la voce, come `POST /api/ab`; prima
  nasceva senza piattaforma, utente e nome del computer.
- Utenti, dispositivi e gruppi di dispositivi nel client (`/api/users`,
  `/api/peers`, `/api/device-group/accessible`): se il database non legge
  il gruppo dell'utente, i gruppi di dispositivi o i dispositivi del
  gruppo, rispondono 400 "Errore di sistema." con l'errore nel log, mai
  401. Prima il gruppo non letto valeva "non condiviso", e chi stava in un
  gruppo condiviso vedeva solo se stesso; `/api/peers` perdeva i nomi dei
  gruppi di dispositivi o dava un elenco vuoto, e il client salvava questi
  elenchi nella cache. Nel pannello dettaglio e cancellazione di un
  gruppo, le regole di condivisione verso un gruppo e l'elenco dei gruppi
  di dispositivi rispondono "Errore di sistema." invece di "Elemento non
  trovato." o di un elenco vuoto; un gruppo che non c'e' ha la risposta di
  prima.
- Pannello, dispositivi e gruppi: se il database non li legge, dettaglio,
  cancellazione ed elenco dei dispositivi, l'elenco dei propri
  dispositivi, gli ID dei dispositivi (`/api/admin/peer/simpleData`),
  l'aggiunta in rubrica dai dispositivi, l'elenco dei gruppi, gruppi e
  utenti (`/api/admin/user/groupUsers`) e dettaglio e cancellazione dei
  gruppi di dispositivi rispondono "Errore di sistema.", con l'errore nel
  log. Prima dicevano "Elemento non trovato." o davano un elenco vuoto.
  Una riga che non c'e' ha la risposta di prima.
- Audit delle connessioni (`/api/audit/conn`): se il database non legge la
  connessione da chiudere o da annotare, l'errore va nel log. Prima
  valeva "connessione mai vista" e l'orario di chiusura o la nota si
  perdevano senza traccia. La risposta al client non cambia.
- Pannello, audit, registro dei login e comandi del server: se il
  database non li legge, gli elenchi (connessioni e file dell'audit,
  registro dei login, propri login, comandi del server) e le
  cancellazioni rispondono "Errore di sistema.", con l'errore nel log.
  Prima gli elenchi erano vuoti (quello dei comandi con i soli comandi
  di sistema) e le cancellazioni dicevano "Elemento non trovato.". Una
  riga che non c'e' ha la risposta di prima.

## [0.2.0] - 2026-09-27
Immagine `ghcr.io/labinfinitek/remotek-api:0.2.0`. Due cambi incompatibili,
sotto Rimosso: via il login GitHub, Google e Linux.do (resta OIDC generico)
e via le chiavi `gorm.max-idle-conns` e `gorm.max-open-conns`. Il database
passa in WAL al primo avvio, senza perdere righe (Corretto).

### Sicurezza
- Il login `webauth` (il client apre il pannello e un utente del pannello
  conferma) esiste solo con `app.web-sso` acceso: spento, `/api/oidc/auth`
  del client e l'associazione dal pannello rispondono come a un provider
  che non esiste (400 "Configurazione non trovata."), non piu' solo senza
  la voce in `/api/login-options`. La conferma dal pannello
  (`/api/admin/oauth/confirm`) accetta solo un login `webauth` non ancora
  confermato: prima confermava qualsiasi codice, e un utente del pannello
  che apriva `/_admin/#/oauth/<code>` di un login OIDC avviato da un altro
  dispositivo gli dava il proprio token senza passare dal provider.
  `/api/admin/oauth/bindConfirm` accetta solo un login che il provider ha
  gia' autenticato. `/api/admin/oauth/info`, `confirm` e `bindConfirm` non
  mandano piu' al browser verifier PKCE, nonce e dati dell'utente del
  provider, solo i campi che le pagine del pannello mostrano.
- Anche `/api/oidc/*` non manda piu' al client ne' al browser il testo di
  un errore interno (REGOLE 8), ultimo gruppo di rotte rimasto: un corpo
  JSON rotto a `/api/oidc/auth` riceve "Parametri non validi." senza il
  testo del parser; la pagina del login OAuth, se il provider risponde con
  un errore o l'autoregistrazione non riesce, non riceve piu' il testo
  dell'errore (che nel secondo caso mostrava grezzo) ma l'ID del messaggio.
  L'errore va nel log a livello warn, con metodo e rotta.

### Corretto
- La pagina del login OAuth mostra le frasi tradotte anche quando hanno un
  apostrofo: `/api/oidc/msg` le scriveva tra apici nello script, e con 8
  messaggi italiani ("L'elemento esiste già.", "L'accesso con password è
  disattivato." e altri) lo script non partiva e la pagina mostrava l'ID.
- La pagina del login OAuth senza `state` dice "Il provider OAuth non ha
  restituito lo stato del login: ripeti il login." invece di "Il campo
  <no value> è vuoto.".
- Le pagine del login OAuth dichiarano la lingua configurata (`lang`,
  vuota vale l'inglese) nell'attributo `lang` e la usano se il browser non
  dice la sua, invece di `zh-CN`.
- Le opzioni di login del client e del pannello, se l'elenco dei provider
  non si legge dal database, rispondono "Errore di sistema." e l'errore va
  nel log, invece di un elenco vuoto.
- Rubrica del client (`POST /api/ab`): se una voce o un tag non si salva,
  non cambia niente e la risposta e' 400 "Operazione non riuscita.", con
  l'errore nel log; prima era 200 `null` anche con i tag salvati a meta'
  (quelli vecchi cancellati, i nuovi no). Lo stesso per la cancellazione
  di una collezione dal pannello, che poteva cancellarne regole e voci e
  lasciare la collezione. La voce nuova senza piattaforma, utente o nome
  del computer legge il dispositivo dentro la transazione: con una
  connessione sola al database (ADR-0007) la richiesta si sarebbe fermata
  per sempre, e con lei l'API.
- Login OIDC con autoregistrazione: se l'associazione al provider non si
  salva, l'utente nuovo non resta e la pagina dice che la registrazione non
  e' riuscita; prima restava un utente senza associazione e la pagina
  diceva successo. Il nome utente libero si cerca prima della transazione,
  nel database e, con LDAP acceso, in LDAP: con una connessione sola al
  database la registrazione si sarebbe fermata per sempre.
- Pannello, cancellazione di un utente: se la conferma della transazione
  non riesce la risposta e' "Operazione non riuscita.", non piu' successo
  con l'utente ancora nel database, e un errore imprevisto a meta' annulla
  la transazione invece di lasciarla aperta.
- Un database creato da rustdesk-api prima del 14 ottobre 2024 ha in
  `peers` una chiave esterna verso `users` (`fk_peers_user`) che l'API non
  crea piu': con le chiavi esterne controllate (ADR-0007) rifiuterebbe i
  dispositivi senza utente, e `/api/sysinfo` di un dispositivo nuovo
  risponderebbe "Operazione non riuscita.". All'avvio l'API la toglie, una
  volta sola, tenendo dispositivi e indici, e lo scrive nel log.
- Il database SQLite passa in WAL, con `synchronous` NORMAL, e l'API lo usa
  con una connessione sola (ADR-0007): prima era in `delete`, dove NORMAL a
  una caduta di corrente puo' corrompere il database, e fino a 100
  connessioni scrivevano insieme, aspettandosi a vicenda fino all'errore
  `database is locked`. Il database in `delete` di un'installazione
  esistente passa a WAL al primo avvio, senza perdere righe. All'avvio,
  dopo la scrittura di prova, l'API si ferma con codice 1 se il database non
  e' in WAL o se SQLite lo trova danneggiato (`PRAGMA integrity_check`, o
  gia' all'apertura), e nel secondo caso dice di ripristinare l'ultimo
  backup. Allo stop normale e alla fine di `reset-admin-pwd` e `reset-pwd`
  il database si chiude e in `data/` resta il solo `rustdeskapi.db`; mentre
  l'API gira ci sono anche `-wal` e `-shm`, e il backup si fa con
  `VACUUM INTO`, non copiando il solo `.db` (README).

### Rimosso
- **Cambio incompatibile per chi usava il login GitHub, Google o Linux.do.**
  Resta il provider OIDC generico, con LDAP (decisione A3). Un provider di
  quei tipi gia' nel database resta, e il pannello lo mostra e lo cancella,
  ma il login lo ignora: non compare in `/api/login-options` ne' tra le
  opzioni del pannello (non conta per `auto_oidc`), chi lo sceglie riceve
  "Configurazione non trovata." come per un provider che non esiste, e
  all'avvio un warn nel log lo nomina col suo op. Crearne o modificarne uno
  dal pannello, il cui menu offre ancora GitHub, Google e LinuxDo, risponde
  "Questo tipo di provider non è più supportato: usa OIDC.". Chi aveva
  Google cancella il vecchio provider (l'op e' unico) e lo rifa' di tipo
  OIDC con IdP `google` e Issuer `https://accounts.google.com`: con lo stesso
  op gli utenti gia' associati restano associati. Chi aveva GitHub o
  Linux.do, che non sono OIDC, cancella il provider: gli utenti nati da quel
  login (autoregistrazione) non hanno una password, e l'amministratore
  gliela imposta dal pannello o con `reset-pwd`. Con `app.disable-pwd-login`
  acceso e solo GitHub o Linux.do, prima di aggiornare va configurato un
  provider OIDC: senza, nel pannello si rientra solo riaccendendo la
  password.
- **Cambio incompatibile per chi impostava `gorm.max-idle-conns` o
  `gorm.max-open-conns`** (`RUSTDESK_API_GORM_MAX_IDLE_CONNS`,
  `RUSTDESK_API_GORM_MAX_OPEN_CONNS`): le due chiavi non ci sono piu',
  perche' con uno scrittore solo (ADR-0007) l'API usa una connessione al
  database, che non si configura. Un valore rimasto nel file o
  nell'ambiente si ignora, senza errore.

## [0.1.0] - 2026-09-26
Base upstream: rustdesk-api v2.7. Primo rilascio: immagine
`ghcr.io/labinfinitek/remotek-api:0.1.0`.

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
- GHSA-2c4m-59x9-fr2g, `github.com/gin-gonic/gin` v1.9.0 -> v1.9.1: nome
  del file non ripulito nell'header `Content-Disposition` di
  `Context.FileAttachment`, che il nostro codice non chiama. Salgono con gin,
  al minimo che chiede: `bytedance/sonic` v1.9.1, `goccy/go-json` v0.10.2,
  `klauspost/cpuid/v2` v2.2.4, `mattn/go-isatty` v0.0.19,
  `pelletier/go-toml/v2` v2.0.8, `ugorji/go/codec` v1.2.11, `x/arch` v0.3.0.
- GHSA-6v2p-p543-phr9, `golang.org/x/oauth2` v0.23.0 -> v0.27.0: consumo di
  memoria eccessivo nel parsing di un token malformato in
  `golang.org/x/oauth2/jws`, pacchetto che il binario non include (il login
  OIDC usa `oauth2`, `endpoints` e `github`, dove cambiano solo commenti).
  Nessun altro modulo sale.
- GHSA-9phm-fm57-rhg8, GHSA-44p7-9xx4-hf2g, GHSA-q675-qj96-32m9,
  `golang.org/x/image` v0.13.0 -> v0.41.0: panic e consumo eccessivo di
  memoria decodificando immagini TIFF o con palette malformate (corretti in
  v0.18.0, v0.38.0 e v0.41.0). Il modulo arriva con il captcha del pannello
  (`mojocn/base64Captcha`), che usa solo `font` e `math/fixed`, identici
  nelle due versioni. Nessun altro modulo sale.
- GHSA-pjcq-xvwq-hhpj, `github.com/Azure/go-ntlmssp`
  v0.0.0-20221128193559-754e69321358 -> v0.1.1: panic su una challenge NTLM
  malformata. Il modulo arriva con `go-ldap/ldap/v3`, che resta v3.4.10; il
  login LDAP usa il bind semplice, non quello NTLM. Nessun altro modulo sale.
- CVE-2026-56854, CVE-2026-56855, CVE-2026-78662, `golang.org/x/crypto`
  v0.53.0 -> v0.56.0, e CVE-2026-46602, CVE-2026-46603, CVE-2026-33813,
  CVE-2026-46601, CVE-2026-46604, `golang.org/x/image` v0.41.0 -> v0.45.0:
  negazione del servizio in `x/crypto/ssh` e nei decodificatori TIFF, WebP
  e VP8L, che il codice non raggiunge (govulncheck; il nostro codice usa
  solo `x/crypto/bcrypt`, il captcha `font` e `math/fixed` di `x/image`).
  Si alzano perche' trivy, che guarda i moduli e non le
  chiamate, ferma il rilascio dell'immagine su tre di loro (HIGH). Salgono
  con loro, al minimo che chiedono: `x/text` v0.41.0, `x/net` v0.57.0,
  `x/sys` v0.47.0, `x/tools` v0.48.0, `x/mod` v0.38.0, `x/sync` v0.22.0.
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
- Le pagine del login OAuth non scrivono piu' come HTML il messaggio del
  server: lo script delle traduzioni si creava con `document.writeln`
  mettendo il messaggio nell'indirizzo cosi' com'era, e un messaggio con
  `"` e `>`, come `"></script><img src=x onerror=alert(1)>`, eseguiva
  JavaScript nell'origine dell'API, la stessa del pannello. Ora lo script si
  crea con `createElement` e lingua, messaggio e titolo passano da
  `encodeURIComponent`; il messaggio si vede come testo. Titolo e messaggio
  si mostrano quando le traduzioni sono arrivate, o subito se non arrivano;
  la pagina di successo non va piu' in errore cercando un paragrafo che non
  ha.
- I file del marchio su `/brand/` escono con una `Content-Security-Policy`
  senza script e in sandbox (`default-src 'none'`; stili dentro il file,
  immagini e font da `/brand/` o `data:`) e con
  `X-Content-Type-Options: nosniff`: uno SVG aperto direttamente e' un
  documento nell'origine del pannello, e uno SVG con uno script lo
  eseguiva. Nel pannello logo e favicon si vedono come prima; uno SVG che
  carica font, immagini o fogli di stile da altri siti ora li perde.

### Corretto
- Se il database si apre ma non si scrive, l'avvio si ferma con codice 1:
  SQLite apre in sola lettura, senza errore, un `data/rustdeskapi.db` che
  l'utente del processo non puo' scrivere, e senza la cartella scrivibile
  non crea il journal, quindi l'API partiva e sbagliava alla prima
  scrittura (login, heartbeat, rubrica). Succede passando dall'immagine di
  upstream, che girava come root, alla nostra, che gira come 10001. Il
  messaggio nomina il file col percorso assoluto, dice se a non scriversi
  e' il file o la cartella e da' il rimedio: `chown -R 10001:10001` della
  cartella dati. La prova crea una tabella e annulla la transazione: nel
  database non resta niente.
- Se il database non si apre, per esempio con `data/` montata dall'host e
  non scrivibile dall'utente 10001 dell'immagine Docker, l'avvio si ferma
  con codice 1 e un messaggio che nomina il file col percorso assoluto e
  dice di controllare che la cartella esista e sia scrivibile dall'utente
  del processo: prima l'errore andava solo su stdout e il processo finiva
  in panic con `unable to open database file: no such file or directory`.
  Se `data/` manca, l'avvio la crea (permessi 0700) invece di fallire.
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
- **Cambio incompatibile per chi usava il web client.** Via il web client
  di RustDesk (`resources/web`, spento di default) con `/webclient`,
  `/webclient2`, `/webclient-config/index.js`, `/api/shared-peer`,
  `/api/server-config` e `/api/server-config-v2`, la condivisione col web
  client dal pannello (`/api/admin/address_book/shareByWebClient`) e i
  registri delle condivisioni (`/api/admin/share_record/*`,
  `/api/admin/my/share_record/*`): ora rispondono 404 come una rotta che non
  esiste. Il client RustDesk nativo non ne chiamava nessuna.
  `app.web-client` (`RUSTDESK_API_APP_WEB_CLIENT`),
  `rustdesk.webclient-magic-queryonline` e `rustdesk.ws-host` non fanno piu'
  niente; il pannello riceve sempre `web_client` 0. Una tabella
  `share_records` gia' nel database resta, innocua: l'API non la usa piu'.
- **Cambio incompatibile per chi ha `lang` diverso da `it` o `en`.** L'API
  parla solo italiano e inglese: via i file di lingua `es`, `fr`, `ko`,
  `ru`, `zh_CN` e `zh_TW` e le traduzioni del validatore in quelle lingue.
  `lang` (`RUSTDESK_API_LANG`) vale solo `it` (il default) o `en`, vuoto
  vale l'inglese; con un altro valore, per esempio `zh-CN` di un file di
  upstream, l'API non parte: esce con codice 1 e un messaggio che elenca i
  valori ammessi. Prima ripiegava sull'inglese in silenzio. Una richiesta
  con `Accept-Language` in un'altra lingua riceve la lingua configurata.
- **Cambio incompatibile per chi usa MySQL o PostgreSQL.** L'API usa solo
  SQLite (`data/rustdeskapi.db`): via i driver MySQL e PostgreSQL, le
  sezioni `mysql` e `postgresql` della configurazione e le variabili
  `RUSTDESK_API_MYSQL_*` e `RUSTDESK_API_POSTGRESQL_*`. `gorm.type` vale
  solo `sqlite` (vuoto vale lo stesso); con un altro valore, per esempio
  `mysql`, l'API non parte: esce con codice 1 e lo scrive nel log, prima di
  creare un database SQLite nuovo e vuoto. Prima un valore sconosciuto
  ripiegava su SQLite in silenzio. Chi ha i dati su MySQL o PostgreSQL resta
  sulla versione precedente oppure li porta su SQLite prima di aggiornare;
  con SQLite non cambia niente.
- Dal binario escono i driver `gorm.io/driver/mysql` e
  `gorm.io/driver/postgres` con i moduli che servivano solo a loro
  (`go-sql-driver/mysql`, `jackc/pgx/v5`, `jackc/pgpassfile`,
  `jackc/pgservicefile`, `jackc/puddle/v2`, `golang.org/x/sync`): 8 moduli
  in meno, nessuno in piu'.
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
- `build.sh` e `build.bat` (con `go env -w` cambiavano per sempre
  l'ambiente Go di chi li lanciava, `GOPROXY` compreso, che puntava a
  `goproxy.cn`), `debian/` (pacchetto `rustdesk-api-server`) e `systemd/`
  (`rustdesk-api.service`): nessuno li usava e nessun test li provava.
  Si compila con `go build -o apimain ./cmd`, l'immagine con `docker build`.

### Modificato
- `docker-compose.yaml` e' un esempio che funziona col `Dockerfile` del
  repo: costruisce l'immagine come `ghcr.io/labinfinitek/remotek-api:dev`,
  con `TZ=Europe/Rome`, segnaposti `<...>` nelle variabili
  `RUSTDESK_API_RUSTDESK_*` e i dati in `./remotek-data`, che deve essere
  di 10001:10001. Prima usava l'immagine di upstream
  `lejianwen/rustdesk-api`, `TZ=Asia/Shanghai`, indirizzi e chiave
  d'esempio (`123456789`) e il `container_name` `rustdesk-api`.
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
- Rilascio con `remotek-release.yml` sul tag `api-vX.Y.Z`: immagine
  costruita dal `Dockerfile`, scansionata con trivy (una vulnerabilita' alta
  o critica con correzione disponibile ferma il rilascio), pubblicata come
  `ghcr.io/labinfinitek/remotek-api:X.Y.Z` e `:X.Y` con attestazione di
  provenienza (`gh attestation verify oci://... --owner labinfinitek`);
  GitHub Release con le note di questo file, SBOM CycloneDX dell'immagine e
  inventario delle licenze.
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
  build, vet e test con `-race`, audit dei workflow, e il `Dockerfile`:
  hadolint, build dell'immagine senza push e avvio del container.
- `test/contratto/`: risposte di riferimento di rustdesk-api v2.7 alle richieste
  del client 1.4.9, rieseguite dalla CI (push e PR verso `remotek`) sul
  router vero dell'API: tutti i 52 passi dello scenario (anonimi, 404 delle
  cinque richieste che l'API non implementa, utente di collaudo con login,
  rubrica e logout, heartbeat, sysinfo e audit del dispositivo, login con
  password sbagliata, avvio e attesa del login OIDC, `sysinfo_ver`, risposta
  a un IP bannato).
