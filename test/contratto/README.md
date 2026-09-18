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
registrati (servono un utente di collaudo sull'istanza di riferimento).

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

## Stato

Registrati i quattro scenari anonimi (`version`, `login-options`,
`non-autenticato`, `audit-conn-active-404`). Il test Go che li riesegue contro
l'API in memoria arriva con la MR dei prerequisiti (DSN configurabile,
`NewEngine()`); fino ad allora sono dati, non un controllo.
