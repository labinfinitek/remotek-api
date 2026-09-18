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

`scenario.json` elenca tutti i passi previsti, anche quelli non ancora
registrati (sezione Stato).

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

Registrati 8 passi: tre scenari anonimi che l'API implementa (`version`,
`login-options`, `non-autenticato`) e i 404 delle cinque richieste che il
client manda e l'API non implementa, che finiscono nel `NoRoute`
(`audit-conn-active-404`, nel gruppo `anonime`, e `audit-alarm-404`,
`devices-deploy-404`, `devices-cli-404`, `audit-nota-guid-404`, nel gruppo
`non-implementate`), rieseguiti
dalla CI (push e PR verso `remotek`) da `TestContract` in
`cmd/contratto_test.go`. Il test sta
in `package main` perche' li' c'e' `InitGlobal()`: e' provvisorio, finche'
il bootstrap non esce da `cmd/`. Gira nel processo di `go test`:
`InitGlobal()` vero, router vero (`http.NewEngine()`) servito da
`httptest.NewServer`, cartella corrente spostata in una cartella temporanea
con `data/`, `runtime/` e i collegamenti a `resources/` e `conf/`, quindi
sqlite su un file temporaneo.
La configurazione dell'istanza di riferimento e' fissata con le variabili
`RUSTDESK_API_*`, cosi' un cambio dei default nel codice non sposta i golden
(i default sicuri avranno test propri). Il modo di gin (`test`) e il livello
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
prerequisito verificato dei passi con utente, che riceveranno nome e
password da `Options.Vars`.

Gruppi attivi: `anonime` (4 passi) e `non-implementate` (4), 8 passi. Restano
da registrare 37 passi su 45: `utente` (29), `peer` (7), `login-errato` (1);
usano l'utente di collaudo i 30 di `utente` e `login-errato`. Aspettano solo
i golden, da registrare dall'istanza di riferimento nella MR successiva.
L'obiettivo e' che attivarli voglia dire aggiungere i golden e i gruppi a
`Groups`; se una differenza di stato tra l'istanza di riferimento e il
database nuovo del test lo impedisce, la precondizione si aggiunge al seme
via HTTP o, se non si puo', quel passo si confronta in `forma`.
`fonte_api` nei `meta.json` indica il codice della v2.7 da cui vengono i
golden: nel fork le righe si spostano (`NoRoute` oggi e' in
`http/http.go:35-37`, non 33-35) e i golden non si correggono per questo.

`estrai` in `meta.json` (variabile <- chiave) salva nel contesto il valore
della chiave nel corpo della risposta, solo se e' una stringa non vuota, e
sostituisce i segnaposto dei passi successivi (`token` <- `access_token`,
`guid` <- `guid`). Se manca vale come vuoto. Il registratore lo scrivera'
prima dei golden con utente: quelli registrati finora non estraggono nulla.

Scelte fissate in REGOLE 6.1 (ADR-0014): niente flag
`-update` (i golden li scrive solo il registratore finche' il riferimento e'
la v2.7), niente go-cmp (confronto byte a byte sulla forma canonica, nessuna
dipendenza nuova), sqlite su file in una cartella temporanea invece che in
memoria (il percorso del DB configurabile arriva dopo, sotto questi test).
