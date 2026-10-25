// Catture del pannello con l'API simulata: login, Profilo e barra, nel tema
// chiaro e nel tema scuro, a 1600x900, in italiano. Il pannello e' il build
// di vite (dist/); /api/*, /brand/* e i file del pannello li serve lo script
// con page.route, e ogni richiesta verso un altro dominio si interrompe:
// niente esce dalla macchina. Playwright non e' una dipendenza del pannello:
// come si lancia sta nel README, sezione "Catture".
//
//   node scripts/catture.mjs [--dist dist] [--brand ../resources/brand] [--out catture] [--nome dopo]
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
}
const sconosciute = new Set()

const ok = (data) => ({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, message: 'success', data }) })

async function servi (route) {
  const url = new URL(route.request().url())
  if (url.origin !== BASE) return route.abort()
  const p = url.pathname
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
  await contesto.close()
}
await browser.close()
if (sconosciute.size) console.log('rotte simulate con una lista vuota:', [...sconosciute].sort().join(' '))
console.log(`catture in ${resolve(opz.out)}`)
