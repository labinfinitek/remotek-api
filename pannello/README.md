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
test "$(node --version)" = "v$(cat .nvmrc)" || echo "serve node $(cat .nvmrc)"
npm ci --registry=https://registry.npmjs.org/ --ignore-scripts
npm audit --audit-level=high --registry=https://registry.npmjs.org/
npm run build    # produce dist/
```

`node_modules/` e `dist/` non entrano nel repo ne' nel contesto di
`docker build`; ne' i file `*.local` (`.env.*.local`, che `vite build`
leggerebbe) e `*.log`.

## Dipendenze

Il lockfile risolve tutto sul registry ufficiale
(`https://registry.npmjs.org/`) e si cambia solo con npm. Nel build
dell'immagine `npm audit --audit-level=high` e' bloccante (REGOLE sez. 9):
un avviso HIGH o CRITICAL nuovo ferma il build, e si chiude aggiornando il
pacchetto. Non ci sono `overrides`.

`protobufjs` e `long` sono dipendenze dirette: le importano
`src/utils/webclient/message.ts` e `rendezvous.ts`, generati a suo tempo con
ts-proto, che non serve piu' e non c'e'. Se quei file vanno rigenerati, si
installa ts-proto solo per farlo.

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
