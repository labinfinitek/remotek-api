// Controlla le lingue del pannello prima del build (ADR-0027): it.json ed
// en.json hanno le stesse chiavi, la stessa struttura (One, e Other dove
// c'e') e gli stessi segnaposto {nome}; ogni chiave che il codice usa con
// un letterale, T('...') o T("..."), e ogni titolo delle rotte esiste in
// tutti e due. Solo librerie di node, nessuna dipendenza.
//
//   node scripts/verifica-lingue.mjs [cartella del pannello]
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'

const radice = process.argv[2] ?? join(import.meta.dirname, '..')
const cartellaLingue = join(radice, 'src/utils/i18n')
const lingue = ['it', 'en']
const forme = ['One', 'Other']
const errori = []

// Un JSON rotto ferma lo script con l'errore di JSON.parse.
const leggi = (lingua) => JSON.parse(readFileSync(join(cartellaLingue, `${lingua}.json`), 'utf8'))
const segnaposti = (testo) => (testo.match(/\{\w+\}/g) ?? []).sort().join(' ')

const dizionari = Object.fromEntries(lingue.map(l => [l, leggi(l)]))
const [a, b] = lingue

// Stesse chiavi e stessa struttura.
for (const lingua of lingue) {
  for (const [chiave, voce] of Object.entries(dizionari[lingua])) {
    const ok = voce && typeof voce === 'object' && !Array.isArray(voce)
      && typeof voce.One === 'string' && voce.One !== ''
      && Object.keys(voce).every(f => forme.includes(f) && typeof voce[f] === 'string' && voce[f] !== '')
    if (!ok) errori.push(`${lingua}.json: "${chiave}" non e' {"One": "..."} con "Other" facoltativo`)
  }
}
for (const [x, y] of [[a, b], [b, a]]) {
  for (const chiave of Object.keys(dizionari[x])) {
    if (!Object.hasOwn(dizionari[y], chiave)) errori.push(`"${chiave}" c'e' in ${x}.json e manca in ${y}.json`)
  }
}

// Stesse forme e stessi segnaposto, chiave per chiave.
for (const chiave of Object.keys(dizionari[a])) {
  const va = dizionari[a][chiave], vb = dizionari[b][chiave]
  if (!va || !vb || typeof va !== 'object' || typeof vb !== 'object') continue
  const fa = Object.keys(va).sort().join(','), fb = Object.keys(vb).sort().join(',')
  if (fa !== fb) errori.push(`"${chiave}": forme diverse (${a}: ${fa}; ${b}: ${fb})`)
  else for (const forma of Object.keys(va)) {
    if (typeof va[forma] !== 'string' || typeof vb[forma] !== 'string') continue
    const sa = segnaposti(va[forma]), sb = segnaposti(vb[forma])
    if (sa !== sb) errori.push(`"${chiave}".${forma}: segnaposto diversi (${a}: [${sa}]; ${b}: [${sb}])`)
  }
}

// Chiavi usate nel codice: T('...') e T("...") in src/, piu' i titoli delle
// rotte, che il menu passa a T(). I commenti non contano.
const sorgenti = []
const visita = (cartella) => {
  for (const nome of readdirSync(cartella)) {
    const p = join(cartella, nome)
    if (statSync(p).isDirectory()) visita(p)
    else if (/\.(vue|js|mjs|ts)$/.test(nome)) sorgenti.push(p)
  }
}
visita(join(radice, 'src'))
const senzaCommenti = (testo) => testo
  .replace(/<!--[\s\S]*?-->/g, m => m.replace(/[^\n]/g, ''))
  .replace(/\/\*[\s\S]*?\*\//g, m => m.replace(/[^\n]/g, ''))
  .replace(/^\s*\/\/.*$/gm, '')
const usate = new Map()
const usa = (chiave, dove) => { if (!usate.has(chiave)) usate.set(chiave, dove) }
for (const file of sorgenti) {
  const testo = senzaCommenti(readFileSync(file, 'utf8'))
  const dove = (indice) => `${relative(radice, file)}:${testo.slice(0, indice).split('\n').length}`
  for (const m of testo.matchAll(/\bT\(\s*(['"])((?:(?!\1)[^\\\n])*)\1/g)) usa(m[2], dove(m.index))
  if (relative(join(radice, 'src'), file) === join('router', 'index.js')) {
    for (const m of testo.matchAll(/\btitle:\s*(['"])((?:(?!\1)[^\\\n])*)\1/g)) usa(m[2], dove(m.index))
  }
}
for (const [chiave, dove] of usate) {
  for (const lingua of lingue) {
    if (!Object.hasOwn(dizionari[lingua], chiave)) errori.push(`${dove}: la chiave "${chiave}" manca in ${lingua}.json`)
  }
}

if (errori.length) {
  console.error(`verifica-lingue: ${errori.length} errori`)
  for (const e of errori) console.error(`  ${e}`)
  process.exit(1)
}
console.log(`verifica-lingue: ok, ${Object.keys(dizionari[a]).length} chiavi in ${lingue.map(l => `${l}.json`).join(' e ')}, ${usate.size} usate nel codice da ${sorgenti.length} file`)
