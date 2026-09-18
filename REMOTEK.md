# Cosa cambia in questo fork rispetto a upstream

Base upstream: **rustdesk-api v2.7** (lejianwen, tag `v2.7`, ultimo commit
2025-09-29). Upstream e' fermo: questo fork e' il progetto (ADR-0005), quindi
l'elenco crescera'. `git log v2.7..remotek` mostra la stessa lista come commit.

| File | Modifica | Motivo | ADR |
|---|---|---|---|
| `.github/workflows/build.yml`, `build_test.yml` | rimossi | partivano su tag `v*.*.*`/`test*` e pubblicavano su registry altrui con azioni non piu' eseguite | 0006 |
| `LICENSE` | riga "Modifiche (c) 2026 Infinitek" sotto l'attribuzione originale | MIT: attribuzione a lejianwen conservata | 0005 |
| `SECURITY.md`, `REMOTEK.md`, `CHANGELOG.md`, `.github/pull_request_template.md` | documenti del fork | sicurezza, tracciabilita' | REGOLE 13 |
| `go.mod`, `.gitignore`, `go.sum` | Go 1.26 / toolchain go1.26.8; `go.sum` versionato (generato dal job `gosum` della CI) | build riproducibile su una release di Go supportata | REGOLE 9 |
| `generate_api.go`, `generate_run.go` -> `tools/generate.go` | direttive `go:generate` spostate fuori dal pacchetto radice, con build tag | la radice era un `package main` senza `main`: `go build/vet/test ./...` fallivano | REGOLE 10.2 |
| `lib/cache/*_test.go`, `lib/lock/local_test.go` | test Redis solo con `REDIS_ADDR`; formato costante nei `Fatalf`; niente letture non sincronizzate | i test devono girare in CI con `-race -shuffle=on` | REGOLE 6.1 |
| `.github/workflows/remotek-ci.yml`, `.gitleaks.toml` | CI del fork: gitleaks, go.sum, build/vet/test, zizmor | controlli a ogni push e PR verso `remotek`, gratis sul repo pubblico | 0012, REGOLE 7 |

Contratto verso il client RustDesk: invariato (ogni endpoint chiamato dal
client avra' un test del contratto prima di essere toccato).

Licenza: MIT, vedi `LICENSE`. Segnalazioni di sicurezza: `SECURITY.md`.

---

# What this fork changes (English)
Upstream base: **rustdesk-api v2.7** (unmaintained upstream; this fork is the
project). Changes are listed above with their reason; the client-facing API
contract is unchanged. Licence: MIT, see `LICENSE`; security: `SECURITY.md`.
