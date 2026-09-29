# Patch del pannello

Il pannello nell'immagine e' [rustdesk-api-web](https://github.com/lejianwen/rustdesk-api-web)
di lejianwen al commit fissato nel `Dockerfile` (`PANNELLO_COMMIT`),
**modificato** dalle patch di questa cartella (ADR-0020). rustdesk-api-web e'
MIT ("Copyright (c) 2016-2021 vue-manage-system"); le patch sono opera
derivata con la stessa licenza. Il `LICENSE` del pannello va nell'immagine
accanto ai suoi file, in `resources/admin/LICENSE`.

Come funziona:

- il `Dockerfile`, nello stadio `pannello-sorgente`, applica ogni
  `*.patch` di questa cartella con `git apply`, in ordine di nome, dopo
  l'adattamento del marchio (`adatta`); se una patch non si applica la build
  si ferma;
- una patch per argomento: il nome dice cosa fa, l'intestazione (prima del
  primo `diff --git`) perche';
- il marchio resta in `adatta`, non in una patch;
- [REMOTEK.md](../REMOTEK.md) elenca ogni patch col motivo;
- quando si alza `PANNELLO_COMMIT`, le patch che non si applicano piu' si
  rifanno nella stessa MR: la build non passa finche' non si applicano
  tutte;
- si passa al fork del pannello oltre 500 righe di patch in tutto, oppure
  per tradurre in italiano l'intero pannello (tutti i file di lingua e i
  testi fissi), anche sotto le 500 righe.

Per scrivere una patch: si clona rustdesk-api-web al commit fissato, si
applicano le patch che ci sono, si modifica, e `git diff` va in un file
nuovo `NNNN-argomento.patch` sotto l'intestazione.
