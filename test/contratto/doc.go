// Package contratto riesegue le richieste del client RustDesk 1.4.9 contro
// l'API e confronta le risposte con i golden registrati dall'istanza di
// riferimento v2.7 (REGOLE 6.1).
//
// E' una libreria in file normali, non di test, perche' la importa il test
// che avvia il router vero da un altro pacchetto. Normalizzazione e forma
// canonica hanno la stessa semantica del registratore Python che scrive i
// golden: questo pacchetto li legge e li confronta, non li riscrive mai.
package contratto
