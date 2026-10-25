// Catture del pannello con l'API simulata: login, Profilo e barra, nel tema
// chiaro e nel tema scuro, a 1600x900, in italiano; piu' la pagina del
// codice di associazione OIDC che vede il tecnico e le Associazioni in
// attesa dell'amministratore. Il pannello e' il build
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
}
await browser.close()
if (sconosciute.size) console.log('rotte simulate con una lista vuota:', [...sconosciute].sort().join(' '))
console.log(`catture in ${resolve(opz.out)}`)
