# Changelog

Formato: Keep a Changelog 1.1.0, in italiano. Versioni: SemVer 2.0.0 (`api-vX.Y.Z`).
Una riga per cambiamento visibile a chi usa o installa il prodotto; la sezione
"Sicurezza" e' obbligatoria per ogni correzione di sicurezza.

## [Non rilasciato]

### Aggiunto
- Note di sessione del tecnico nel registro delle connessioni, che l'API
  prima perdeva. La nota che il client manda durante la sessione si salva
  sulla connessione gia' registrata con quell'ID e quel `session_id`, senza
  crearne di nuove; la nota di fine connessione (opzione del client
  `allow-ask-for-note`, spenta di fabbrica) passa da
  `GET /api/audit/conn/active` e `PUT /api/audit`, col login del tecnico e
  un guid casuale della connessione. Al massimo 2000 caratteri, oltre si
  tronca; il testo della nota non va mai nel log. La nota si legge nel
  JSON di `/api/admin/audit_conn/list` e nell'esportazione CSV del
  pannello: il pannello (rustdesk-api-web `3998c2a`) non ha una colonna per
  la nota.

### Sicurezza
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
  l'ultimo contatto fermo; si sblocca cancellandolo dal pannello, e il
  sysinfo successivo lo ricrea. L'audit salva il `session_id` esatto; prima,
  sopra 2^53, lo arrotondava.
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
