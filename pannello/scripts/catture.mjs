// Catture del pannello con l'API simulata: login, Profilo e barra, nel tema
// chiaro e nel tema scuro, a 1600x900, in italiano; piu' la pagina del
// codice di associazione OIDC che vede il tecnico, le Associazioni in
// attesa dell'amministratore e la lista dei PC a 1280 e 1600 px. Il pannello e' il build
// di vite (dist/); /api/*, /brand/* e i file del pannello li serve lo script
// con page.route, e ogni richiesta verso un altro dominio si interrompe:
// niente esce dalla macchina. Playwright non e' una dipendenza del pannello:
// come si lancia sta nel README, sezione "Catture".
//
//   node scripts/catture.mjs [--dist dist] [--brand ../resources/brand] [--resources ../resources] [--out catture] [--nome dopo]
import { createRequire } from 'node:module'
import { mkdir, readFile } from 'node:fs/promises'
import { extname, join, resolve } from 'node:path'
import { parseArgs } from 'node:util'

const require = createRequire(import.meta.url)
const { chromium } = require('playwright')

const radice = join(import.meta.dirname, '..')
const { values: opz } = parseArgs({
  options: {
    dist: { type: 'string', default: join(radice, 'dist') },
    brand: { type: 'string', default: join(radice, '..', 'resources', 'brand') },
    // modelli e traduzioni delle pagine del callback OIDC
    resources: { type: 'string', default: join(radice, '..', 'resources') },
    out: { type: 'string', default: join(radice, 'catture') },
    nome: { type: 'string', default: 'pannello' },
  },
})

const BASE = 'http://pannello.prova'
const TOKEN = 'token-di-prova'
const tipi = {
  '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml',
  '.png': 'image/png', '.webp': 'image/webp', '.woff': 'font/woff', '.woff2': 'font/woff2', '.json': 'application/json',
}

// I dati di prova: un amministratore del gruppo "Assistenza" con un provider
// OIDC associato.
const utente = {
  username: 'mrossi', nickname: 'Mario Rossi', email: 'mario.rossi@esempio.it', avatar: '',
  token: TOKEN, route_names: ['*'], group_name: 'Assistenza',
}
const api = {
  '/api/version': 'prova',
  '/api/admin/login-options': { ops: ['aziendale'], register: false, need_captcha: false, disable_pwd: false, auto_oidc: false },
  '/api/admin/config/admin': { title: 'Remotek', hello: '' },
  '/api/admin/config/server': { id_server: '', key: '', api_server: '', relay_server: '' },
  '/api/admin/user/current': utente,
  '/api/admin/user/myOauth': [{ op: 'aziendale', status: 1 }],
  '/api/admin/oauth/richieste': [
    { codice: 'K7Q2-M9XD', op: 'aziendale', nome: 'Luca Bianchi', email: 'luca.bianchi@esempio.it', email_verificata: true, secondi: 240 },
    { codice: 'P4WZ-3HNC', op: 'aziendale', nome: 'Anna Verdi', email: 'anna.verdi@esempio.it', email_verificata: false, secondi: 1260 },
  ],
  '/api/admin/user/groupUsers': { groups: [], users: [{ id: 2, username: 'lbianchi', nickname: 'Luca Bianchi', group_id: 1 }] },
}
// La lista dei PC: alcuni in linea e altri no, con tutte le colonne.
const ora = Math.floor(Date.now() / 1000)
api['/api/admin/peer/list'] = {
  total: 3,
  list: [
    { row_id: 1, id: '999000111', cpu: 'Intel Core i5-1135G7', hostname: 'PC-UFFICIO', memory: '16GB', os: 'Windows 11', last_online_time: ora - 20, last_online_ip: '192.0.2.10', username: 'mrossi', group_id: 0, uuid: 'dXVpZC0x', version: '1.4.9', alias: '', created_at: '2026-10-01 09:00:00', updated_at: '2026-10-25 10:00:00' },
    { row_id: 2, id: '999000222', cpu: 'AMD Ryzen 5 5600U', hostname: 'PORTATILE-ANNA', memory: '8GB', os: 'Windows 10', last_online_time: ora - 7200, last_online_ip: '192.0.2.22', username: 'averdi', group_id: 0, uuid: 'dXVpZC0y', version: '1.4.9', alias: '', created_at: '2026-10-02 09:00:00', updated_at: '2026-10-24 18:00:00' },
    { row_id: 3, id: '999000333', cpu: 'Apple M2', hostname: 'MAC-GRAFICA', memory: '16GB', os: 'macOS 15', last_online_time: 0, last_online_ip: '', username: '', group_id: 0, uuid: 'dXVpZC0z', version: '1.4.8', alias: '', created_at: '2026-10-03 09:00:00', updated_at: '2026-10-03 09:00:00' },
  ],
}
const STATO = 'stato-del-login'
const CODICE = 'K7Q2-M9XD'

// La pagina del codice: il modello oauth_codice.html con i valori al posto
// delle azioni di Go (che qui sono solo nomi e stringhe) e /api/oidc/msg
// dalle traduzioni di resources/i18n/it.toml.
const traduzioni = (await readFile(join(opz.resources, 'i18n', 'it.toml'), 'utf8').catch(() => ''))
const traduci = (id) => traduzioni.match(new RegExp(`^\\[${id}\\]\\n(?:.*\\n)*?one = "(.*)"$`, 'm'))?.[1] ?? id
const modelloCodice = await readFile(join(opz.resources, 'templates', 'oauth_codice.html'), 'utf8').catch(() => null)
function paginaCodice () {
  return modelloCodice.replaceAll('{{lang}}', 'it').replaceAll('{{brand}}', 'Remotek')
    .replaceAll('{{.op}}', 'aziendale').replaceAll('{{.codice}}', CODICE)
}
function msg (url) {
  return ['title', 'msg'].map(k => `;${k} = ${JSON.stringify(traduci(url.searchParams.get(k)))};`).join('')
}
const sconosciute = new Set()

const ok = (data) => ({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, message: 'success', data }) })

async function servi (route) {
  const url = new URL(route.request().url())
  if (url.origin !== BASE) return route.abort()
  const p = url.pathname
  if (p === '/api/oidc/callback') return route.fulfill({ status: 200, contentType: 'text/html', body: paginaCodice() })
  if (p === '/api/oidc/msg') return route.fulfill({ status: 200, contentType: 'application/javascript', body: msg(url) })
  // senza login la pagina di associazione non legge il login OIDC, come l'API
  if (p === '/api/admin/oauth/info' && !route.request().headers()['api-token']) {
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 403, message: 'Accedi prima', data: null }) })
  }
  if (p === '/api/admin/oauth/info') return route.fulfill(ok({ id: '999000111', device_name: 'PC-COLLAUDO', op: 'aziendale', codice: CODICE }))
  if (p.startsWith('/api/')) {
    if (Object.hasOwn(api, p)) return route.fulfill(ok(api[p]))
    sconosciute.add(p)
    return route.fulfill(ok({ list: [], total: 0 }))
  }
  let file
  if (p.startsWith('/brand/')) file = join(opz.brand, p.slice('/brand/'.length))
  else if (p.startsWith('/_admin/')) file = join(opz.dist, p.slice('/_admin/'.length) || 'index.html')
  else return route.fulfill({ status: 404 })
  try {
    return route.fulfill({ status: 200, contentType: tipi[extname(file)] ?? 'application/octet-stream', body: await readFile(file) })
  } catch {
    return route.fulfill({ status: 404 })
  }
}

await mkdir(opz.out, { recursive: true })
const browser = await chromium.launch()
for (const tema of ['light', 'dark']) {
  const nomeTema = tema === 'light' ? 'chiaro' : 'scuro'
  const contesto = await browser.newContext({ viewport: { width: 1600, height: 900 }, colorScheme: tema, locale: 'it-IT' })
  await contesto.route('**/*', servi)
  const page = await contesto.newPage()

  // Login: senza token, col modulo della password.
  await page.goto(`${BASE}/_admin/#/login?password=1`)
  await page.waitForLoadState('networkidle')
  await page.screenshot({ path: resolve(opz.out, `${opz.nome}-login-${nomeTema}.png`) })

  // Profilo, con la barra in alto: col token nel browser.
  await page.evaluate((t) => localStorage.setItem('access_token', t), TOKEN)
  await page.goto(`${BASE}/_admin/#/`)
  await page.reload()
  await page.locator('.header-logo').waitFor()
  await page.waitForLoadState('networkidle')
  await page.screenshot({ path: resolve(opz.out, `${opz.nome}-profilo-${nomeTema}.png`) })
  await page.screenshot({ path: resolve(opz.out, `${opz.nome}-barra-${nomeTema}.png`), clip: { x: 0, y: 0, width: 1600, height: 64 } })

  // Associazioni in attesa e la finestra "Associa a", da amministratore.
  await page.goto(`${BASE}/_admin/#/oauth/richieste`)
  await page.reload()
  await page.waitForLoadState('networkidle')
  await page.screenshot({ path: resolve(opz.out, `${opz.nome}-richieste-${nomeTema}.png`) })
  const associa = page.getByRole('button', { name: 'Associa a' }).first()
  if (await associa.count()) {
    await associa.click()
    await page.locator('.el-dialog').waitFor()
    await page.waitForTimeout(300)
    await page.screenshot({ path: resolve(opz.out, `${opz.nome}-associa-${nomeTema}.png`) })
  }

  // Il tecnico senza associazione dopo il callback OIDC: la pagina del
  // codice, o senza il modello la pagina di associazione del pannello, dove
  // il callback mandava prima, aperta senza login.
  await page.evaluate(() => localStorage.removeItem('access_token'))
  if (modelloCodice) await page.goto(`${BASE}/api/oidc/callback?state=${STATO}&code=x`)
  else await page.goto(`${BASE}/_admin/#/oauth/bind/${STATO}`)
  await page.reload()
  await page.waitForLoadState('networkidle')
  await page.screenshot({ path: resolve(opz.out, `${opz.nome}-codice-${nomeTema}.png`) })
  await contesto.close()

  // La lista dei PC a 1280 e a 1600 px, per la colonna Azioni fissata.
  for (const larghezza of [1280, 1600]) {
    const c = await browser.newContext({ viewport: { width: larghezza, height: 900 }, colorScheme: tema, locale: 'it-IT' })
    await c.route('**/*', servi)
    const p = await c.newPage()
    await p.goto(`${BASE}/_admin/#/login`)
    await p.evaluate((t) => localStorage.setItem('access_token', t), TOKEN)
    await p.goto(`${BASE}/_admin/#/user/peer`)
    await p.reload()
    await p.locator('.el-table__row').first().waitFor()
    await p.waitForLoadState('networkidle')
    await p.screenshot({ path: resolve(opz.out, `${opz.nome}-pc-${larghezza}-${nomeTema}.png`) })
    await c.close()
  }
}
await browser.close()
if (sconosciute.size) console.log('rotte simulate con una lista vuota:', [...sconosciute].sort().join(' '))
console.log(`catture in ${resolve(opz.out)}`)
