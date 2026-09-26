# Test del contratto verso il client

Il client RustDesk/Remotek parla con questa API con richieste fisse, scritte
nei suoi sorgenti (`src/hbbs_http/`, `flutter/lib/common/hbbs/`). Questa
cartella conserva **cosa rispondeva rustdesk-api v2.7** a quelle richieste:
ogni modifica all'API deve continuare a rispondere nella stessa forma, oppure
cambiare il golden con una motivazione scritta nella MR.

## Cosa c'e'

`testdata/client-<versione del client>/<scenario>/`:

| File | Contenuto |
|---|---|
| `richiesta.json` | metodo, percorso, query, intestazioni e corpo, copiati dai sorgenti del client |
| `risposta.golden.json` | stato, content-type e corpo registrati dall'istanza di riferimento v2.7 |
| `meta.json` | da dove viene la richiesta nel client (`fonte_client`), quale codice dell'API risponde (`fonte_api`), quali campi sono stati sostituiti da segnaposto (`normalizzati`) e come si confronta (`confronto`: `esatto` o `forma`) |

`scenario.json` elenca i passi della tabella del registratore nell'ordine in
cui il test li riesegue; un passo ancora senza golden, se ce ne sara' uno, si
elenca nella sezione Stato.

## Come si registrano

Non a mano. Li scrive `strumenti/registra-contratto.py` del repo interno,
contro l'istanza v2.7 di riferimento; id, timestamp e token diventano
segnaposto, quindi qui dentro **non ci sono segreti ne' dati personali** (la
CI lo controlla con gitleaks). I golden restano il riferimento anche dopo
`api-v0.1.0`: si rigenerano solo quando cambia la versione del client
supportata, in una cartella nuova.

## Libreria Go

Il pacchetto `contratto` rilegge i golden con la stessa semantica del
registratore: `Decode` (numeri come `json.Number`, dati dopo il valore =
errore), `Normalize` (segnaposto) e `Serialize` (forma canonica);
`TestSerializeMatchesRecorder` pretende gli stessi byte su ogni file di
`testdata/`. I golden li scrive solo il registratore: niente flag `-update`,
perche' riscriverli dal nostro output svuoterebbe il test. Limiti noti, tutti
assenti dai payload di oggi: i numeri non interi e l'intero `-0` Python li
riscrive (`1e-06`, `0`), Go ne conserva il letterale; `NaN` e `Infinity`
`json.loads` li accetta, `Decode` no; `\d` in Python accetta anche cifre non
ASCII; i byte non UTF-8.

`Run` riesegue verso un URL base i passi dei gruppi scelti, nell'ordine di
`scenario.json` e uno per sottotest, e confronta ogni risposta col golden.
Le regole sono fail-closed, perche' un verde deve aver provato qualcosa:
sono errori un gruppo che lo scenario non ha, nessun passo da eseguire, un
passo di un gruppo attivo senza golden (un gruppo non ancora registrato
non si attiva), un confronto diverso da `esatto` o `forma`, un segnaposto
senza valore nel contesto, una risposta compressa o oltre 4 MiB.
`TestRunStepReportsDifference` e' la prova permanente che il motore
diventa rosso quando la risposta cambia.

## Stato

Registrati tutti i 52 passi di `scenario.json`, rieseguiti dalla CI (push e
PR verso `remotek`) da `TestContract` in `cmd/contratto_test.go`: tre
scenari anonimi che l'API implementa (`version`, `login-options`,
`non-autenticato`); i 404 delle cinque richieste che il client manda e l'API
non implementa, che finiscono nel `NoRoute` (`audit-conn-active-404`, nel
gruppo `anonime`, e `audit-alarm-404`, `devices-deploy-404`,
`devices-cli-404`, `audit-nota-guid-404`, nel gruppo `non-implementate`); i
29 passi dell'utente di collaudo (`utente`: login, utente corrente, rubrica
personale con tag e peer, rubrica legacy, utenti e dispositivi del gruppo,
logout); i 7 del dispositivo, senza autenticazione (`peer`: heartbeat,
sysinfo, audit delle connessioni e dei file); il login con la password
sbagliata (`login-errato`); i 5 senza utente che non scrivono nel database
(`senza-utente`: l'avvio del login OIDC `webauth` e le due risposte di
`/api/oidc/auth-query` che il client distingue, "in attesa" e "scaduto", un
`op` sconosciuto, `/api/sysinfo_ver`); i 2 del ban (`ban`: login con la
password giusta e heartbeat da un IP bannato). Il test sta
in `package main` perche' li' c'e' `InitGlobal()`: e' provvisorio, finche'
il bootstrap non esce da `cmd/`. Gira nel processo di `go test`:
`InitGlobal()` vero, router vero (`http.NewEngine()`) servito da
`httptest.NewServer`, cartella corrente spostata in una cartella temporanea
con `data/`, `runtime/` e i collegamenti a `resources/` e `conf/`, quindi
sqlite su un file temporaneo.
La configurazione dell'istanza di riferimento e' fissata con le variabili
`RUSTDESK_API_*`, cosi' un cambio dei default nel codice non sposta i golden
(i default sicuri hanno test propri). Il modo di gin (`test`) e il livello
del log (`warn`) sono scelti per la CI e non toccano le risposte.
L'autoregistrazione invece e' accesa, `RUSTDESK_API_APP_REGISTER=true` (con
`RUSTDESK_API_APP_REGISTER_STATUS=1`), solo nel processo del test:
l'istanza di riferimento e la produzione la tengono spenta, e nessun
endpoint del client la legge.

Serve al seme. Prima degli scenari il sottotest `seme` crea l'utente di
collaudo `collaudo-contratto` (non admin, email vuota) con
`POST /api/admin/user/register`: via HTTP e non dai servizi interni, perche'
deve sopravvivere al loro refactor (ADR-0014), e con una password casuale a
ogni esecuzione che non si stampa mai. Poi rifa il giro di `--verifica` del
registratore col corpo che manda il client: login (200, token non vuoto,
nome giusto, email vuota, non admin, `info` oggetto), `currentUser` (200),
logout (200), `currentUser` di nuovo (401). Se il seme fallisce gli scenari
non partono. Il seme non e' un test del contratto (non ha golden): e' il
prerequisito verificato dei passi con utente, che ricevono nome e password
da `Options.Vars`.

Gruppi attivi: tutti e sette, nell'ordine dello scenario: `anonime` (4
passi), `non-implementate` (4), `utente` (29), `peer` (7), `login-errato`
(1), `senza-utente` (5), `ban` (2). Nessun passo resta da registrare.
`ban` gira per ultimo e in una chiamata a parte di `Run`: prima il
sottotest `preparazione-ban` manda login sbagliati via HTTP finche' la
risposta non e' piu' 400, come il passo di servizio `ban-preparazione` del
registratore (che nello scenario non c'e'); da li' ogni richiesta del test
riceve la risposta del ban. Il ban della v2.7 e' un middleware su tutto il
router (`http/middleware/limiter.go`): risponde 200 con
`{"code":423,...}` a ogni richiesta dell'IP, heartbeat compresi, per 30
minuti. I golden vengono da un utente di
collaudo dell'istanza di riferimento che, quando e' stato registrato il
gruppo `utente`, aveva le stesse caratteristiche di quello del seme (non
admin, gruppo predefinito, email vuota, nessuna rubrica condivisa, nessun
peer); `peer` e `login-errato` sono stati registrati dopo, una volta sola;
`senza-utente` e `ban` il 2026-09-26, senza login (`ban` con i login
sbagliati della preparazione).
Se un giorno una differenza di stato tra l'istanza e il database nuovo del
test rompesse un passo, la precondizione si aggiunge al seme via HTTP o, se
non si puo', quel passo si confronta in `forma` nella tabella del
registratore e si registra di nuovo; mai un golden corretto a mano.

Attenzione prima di registrare di nuovo il gruppo `utente`: l'istanza di
riferimento non e' piu' nello stato di allora. Il giro di `peer` ha lasciato
il peer finto `999000111`, che ha lo stesso id e lo stesso uuid del login di
`utente`, legato all'utente di collaudo: `sysinfo` lo crea con l'utente
dell'ultimo login di quel dispositivo e ogni login lo ricollega (v2.7,
`http/controller/api/peer.go:35-38`, `service/user.go:109-110`). Nel database
nuovo del test quel peer nasce solo al passo `sysinfo`, dopo tutto il gruppo
`utente`. Registrare di nuovo `utente` con il peer ancora sull'istanza
cambia tre golden: `peers` (un elemento invece di nessuno),
`ab-peers-con-peer` e `ab-legacy-get` (nome host, piattaforma e utente
copiati dal peer, `http/controller/api/ab.go:598-604`). Il registratore
stampa `cambiato` ma esce con 0 problemi, perche' conta solo gli stati HTTP
diversi dall'atteso, e `TestContract` diventa rosso per un residuo
dell'istanza, non per una regressione. La precondizione di
`peers/meta.json`, che ammette il peer finto, e' quindi sbagliata: si
corregge nella tabella del registratore, da cui viene. `forma` non basta,
perche' la forma conserva la lunghezza degli elenchi. I rimedi sono due, da
scegliere prima di registrare:

1. il seme, come dice la regola sopra: dopo il login il sottotest `seme`
   manda `POST /api/sysinfo` col corpo del passo `sysinfo` (rotta senza
   autenticazione, `http/router/api.go:60`, prima di `RustAuth` a :76) e
   pretende 200 `SYSINFO_UPDATED`; il peer nasce legato all'utente del seme,
   come sull'istanza, e nella stessa MR si registra di nuovo `utente`
   sull'istanza cosi' com'e'. Il rimedio si regge proprio su quel residuo:
   se la riga `999000111` non c'e' piu' (rimedio 2, o l'azzeramento
   dell'istanza prima del go-live), prima di `utente` si rifa' il giro
   `--gruppi peer --includi-scritture-peer`, che la ricrea con `sysinfo`, e
   il login di `utente` la ricollega. I tre golden diventano riproducibili e
   `peers` controlla finalmente un elemento, che e' deterministico (campi
   del corpo di `sysinfo` e nome dell'utente,
   `http/response/api/peer.go:63-73`). I passi `heartbeat` e `sysinfo`
   dello scenario passano allora dai rami del peer noto
   (`http/controller/api/index.go:57-61`,
   `http/controller/api/peer.go:43-54`), quelli di ogni giorno in
   produzione; la creazione la prova solo il seme, senza golden;
2. chiedere a setup-lab di cancellare dall'istanza la riga `peers`
   `999000111`. Cancellarla, non scollegarla: il login la ricollega
   cercandola per uuid, di chiunque sia (`service/peer.go:36-41`), e
   `ab-peer-add` copia i campi del peer anche senza utente. I golden di
   oggi restano validi.

`fonte_api` nei `meta.json` indica il codice della v2.7 da cui vengono i
golden: nel fork le righe si spostano (`NoRoute` oggi e' in
`http/http.go:35-37`, non 33-35) e i golden non si correggono per questo.

Non coperto da questi test: i messaggi in italiano (l'istanza di riferimento
risponde in inglese e il test fissa `RUSTDESK_API_LANG=en`), la scadenza e
il rinnovo del token, il login OIDC completato (serve il pannello o un
provider vero: coperti solo l'avvio e l'attesa) e LDAP, il pannello admin, l'avvio
vero (`main`, cobra, endless) e i
default sicuri, che hanno test propri. Coperti solo in parte, finche' la
tabella del registratore non avra' i passi che mancano: `/api/peers`
risponde sempre con l'elenco vuoto, perche' `peers` viene prima di
`sysinfo`, quindi la forma di un elemento (`GroupPeerPayload`, letto dal
client in `group_model.dart`) non la controlla nessun golden; `heartbeat`
si prova solo con il peer sconosciuto (`{}` senza scrivere,
`http/controller/api/index.go:52-56`) e `sysinfo` solo quando crea il
peer, mentre il client manda `sysinfo` prima dell'heartbeat
(`src/hbbs_http/sync.rs:125-244`) e in produzione il peer c'e' quasi
sempre (il rimedio 1 sopra darebbe un elemento a `peers` e porterebbe i
due passi sul ramo del peer noto); di `ab-peer-delete`, `ab-tag-delete`,
`ab-legacy-post` e `sysinfo` si confronta la risposta ma non l'effetto,
perche' nessun passo successivo rilegge rubrica o peer.

`estrai` in `meta.json` (variabile <- chiave) salva nel contesto il valore
della chiave nel corpo della risposta, solo se e' una stringa non vuota, e
sostituisce i segnaposto dei passi successivi: lo hanno `login` (`token` <-
`access_token`), `ab-personal` (`guid` <- `guid`) e `oidc-auth-webauth`
(`codice` <- `code`, segnaposto `__CODICE__` nella query di
`oidc-auth-query-in-attesa`). Se manca vale come vuoto.

`__MASCHERATO__` nel corpo di un golden e' un valore che il registratore
non scrive perche' casuale o legato all'istanza (il `code` e l'`url` di
`oidc-auth-webauth`, che contiene l'indirizzo dell'API): succede solo nei
passi a confronto `forma`, dove conta il tipo, e `normalizzati` in
`meta.json` lo elenca.

Scelte fissate in REGOLE 6.1 (ADR-0014): niente flag
`-update` (i golden li scrive solo il registratore finche' il riferimento e'
la v2.7), niente go-cmp (confronto byte a byte sulla forma canonica, nessuna
dipendenza nuova), sqlite su file in una cartella temporanea invece che in
memoria (il percorso del DB configurabile arriva dopo, sotto questi test).
