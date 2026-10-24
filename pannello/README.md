# Pannello di amministrazione

Il pannello che l'API serve su `/_admin/` si costruisce da questa cartella
(ADR-0027).

## Origine e licenza

Il sorgente viene da [rustdesk-api-web](https://github.com/lejianwen/rustdesk-api-web)
di lejianwen, commit `3998c2a9213fcd047252776d0f0db33e6717026c` del
2025-08-31, l'ultimo di upstream. La documentazione di upstream e' il suo
[README a quel commit](https://github.com/lejianwen/rustdesk-api-web/blob/3998c2a9213fcd047252776d0f0db33e6717026c/README.md);
l'unico file di upstream che qui non c'e' e' proprio quel README.

rustdesk-api-web e' MIT ("Copyright (c) 2016-2021 vue-manage-system"):
[LICENSE](LICENSE) resta intatto. Le nostre modifiche sono opera derivata
con la stessa licenza. Il `LICENSE` va nell'immagine accanto ai file del
pannello, in `resources/admin/LICENSE`.

I caratteri Inter e Space Grotesk vengono dai pacchetti npm `@fontsource/inter`
e `@fontsource/space-grotesk` e finiscono nel build come file del pannello:
nessuna richiesta a Google Fonts o ad altri siti. Sono SIL Open Font License
1.1; le due licenze vanno nell'immagine in `resources/admin/LICENSE-inter` e
`resources/admin/LICENSE-space-grotesk`.

## Le nostre modifiche

Il primo commit su questa cartella importa upstream senza toccarlo; tutto
quello che viene dopo e' nostro:

```bash
git log -- pannello/
# tutte le modifiche insieme, dal commit dell'import a oggi
import=$(git log --diff-filter=A --format=%h -1 -- pannello/package.json)
git diff "$import" -- pannello/
```

[REMOTEK.md](../REMOTEK.md) elenca ogni modifica col motivo.

## Build in locale

Con la versione di node di [.nvmrc](.nvmrc), quella dell'immagine, e gli
stessi comandi dello stadio `pannello` del Dockerfile:

```bash
cd pannello
{ test "$(node --version)" = "v$(cat .nvmrc)" || { echo "serve node v$(cat .nvmrc)" >&2; false; }; } \
  && ! grep '"resolved":' package-lock.json | grep -vF '"resolved": "https://registry.npmjs.org/' \
  && npm ci --registry=https://registry.npmjs.org/ --ignore-scripts \
  && npm audit --audit-level=high --registry=https://registry.npmjs.org/ \
  && node scripts/verifica-lingue.mjs \
  && npm run build    # produce dist/
```

La catena si ferma, come l'immagine, se node non e' quello di `.nvmrc` (e lo
dice), se il lockfile ha un `resolved` fuori dal registry ufficiale (lo
stampa), se l'audit trova un avviso HIGH o CRITICAL o se le lingue non sono
allineate (sotto, Lingue).

`node_modules/` e `dist/` non entrano nel repo ne' nel contesto di
`docker build`; ne' i file `*.local` (`.env.*.local`, che `vite build`
leggerebbe) e `*.log`.

## Lingue

Il pannello parla italiano e inglese, come l'API: `src/utils/i18n/it.json`
ed `en.json`, con le locale `it` ed `en` di element-plus. La lingua
iniziale e' quella scelta dal menu e salvata nel browser (`localStorage`,
chiave `lang`) se e' `it` o `en`, altrimenti `it`; la lingua del browser non
conta. La lingua scelta va all'API in `Accept-Language`.

Ogni testo che l'utente vede passa da `T('Chiave')`, anche etichette,
segnaposto e messaggi. Una chiave nuova si aggiunge a tutti e due i file,
nella stessa posizione, con la stessa struttura (`One`, e `Other` per il
plurale) e gli stessi segnaposto `{nome}`. I nomi propri e i termini tecnici
senza un equivalente usato (ID, IP, OIDC, token, hash) restano in inglese.

`scripts/verifica-lingue.mjs`, prima di `npm run build` nel Dockerfile,
ferma il build se i due file differiscono in chiavi, struttura o segnaposto,
o se manca una chiave usata con un letterale (`T('...')`, `T("...")`, titoli
delle rotte). Le chiavi composte a runtime (`T(row.label)`) non le vede.

## Dipendenze

Il lockfile risolve tutto sul registry ufficiale
(`https://registry.npmjs.org/`) e si cambia solo con npm. Nel build
dell'immagine `npm audit --audit-level=high` e' bloccante (REGOLE sez. 9):
un avviso HIGH o CRITICAL nuovo ferma il build, e si chiude aggiornando il
pacchetto. Non ci sono `overrides`.

## Un cambio di upstream

Upstream e' fermo; se pubblica qualcosa che serve, lo si porta a mano: la
differenza fra `3998c2a` e il commit nuovo di rustdesk-api-web si applica a
questa cartella in una MR sua, che dice da quale commit viene. Non si
reimporta upstream sopra le nostre modifiche.

## BRAND_NAME

Nel sorgente il titolo statico (`index.html` e `src/store/app.js`) e'
Remotek; `docker build --build-arg BRAND_NAME=<nome>` lo cambia, e la build
si ferma se una delle due righe non c'e' piu'. E' il titolo che il pannello
mostra prima di leggere la configurazione: quello vero viene da
`/api/admin/config/admin`. Logo e favicon non dipendono da `BRAND_NAME`:
vengono da `/brand/`, che l'API serve da `brand.dir`.
