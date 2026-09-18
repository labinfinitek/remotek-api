# Changelog

Formato: Keep a Changelog 1.1.0, in italiano. Versioni: SemVer 2.0.0 (`api-vX.Y.Z`).
Una riga per cambiamento visibile a chi usa o installa il prodotto; la sezione
"Sicurezza" e' obbligatoria per ogni correzione di sicurezza.

## [Non rilasciato]
Base upstream: rustdesk-api v2.7.

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
  router vero dell'API (per ora i quattro scenari anonimi).
