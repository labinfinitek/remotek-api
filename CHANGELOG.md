# Changelog

Formato: Keep a Changelog 1.1.0, in italiano. Versioni: SemVer 2.0.0 (`api-vX.Y.Z`).
Una riga per cambiamento visibile a chi usa o installa il prodotto; la sezione
"Sicurezza" e' obbligatoria per ogni correzione di sicurezza.

## [Non rilasciato]
Base upstream: rustdesk-api v2.7.

### Sicurezza
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

### Rimosso
- Workflow upstream `build.yml` e `build_test.yml` (build e pubblicazione su
  registry altrui): la CI del fork arriva con `remotek-ci.yml`.

### Modificato
- Si compila con Go 1.26 (toolchain go1.26.8) e `go.sum` e' versionato: le
  dipendenze di un build sono quelle del commit, non quelle del giorno.

### Aggiunto
- `SECURITY.md`, `REMOTEK.md`, template di pull request; attribuzione delle
  modifiche in `LICENSE`.
- golangci-lint v2 in CI: bloccante sui pacchetti gia' bonificati, informativo
  sul resto (703 finding ereditati al primo giro, da azzerare nel Passo 3).
- CI `remotek-ci.yml`: ricerca di segreti, controllo di `go.mod`/`go.sum`,
  build, vet e test con `-race`, audit dei workflow.
- `test/contratto/`: risposte di riferimento di rustdesk-api v2.7 alle richieste
  del client 1.4.9, rieseguite dalla CI (push e PR verso `remotek`) sul
  router vero dell'API (per ora i quattro scenari anonimi e il 404 delle
  quattro richieste del client che l'API non implementa).
