import { reactive } from 'vue'

// Cio' che e' di chi installa (ragione sociale, informativa, nome sul
// pulsante del login, logo per il tema scuro) non sta nel codice: viene da
// /brand/brand.json, facoltativo, nella stessa cartella di logo.svg
// (brand.dir). Senza il file, o con un campo non valido, valori neutri.
// Formato nel README, sezione "Marchio del pannello".
export const marchio = reactive({
  ragioneSociale: '',
  informativa: '',
  nomeAccesso: '',
  logoScuro: '',
  iconaAccesso: '',
})

// Solo un nome di file dentro /brand/, niente percorsi ne' indirizzi esterni.
const file = (v) => typeof v === 'string' && /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}\.(svg|png|webp)$/.test(v) ? `/brand/${v}` : ''
const testo = (v, max) => typeof v === 'string' ? v.trim().slice(0, max) : ''
const https = (v) => {
  if (typeof v !== 'string') return ''
  try {
    return new URL(v).protocol === 'https:' ? v : ''
  } catch {
    return ''
  }
}

let caricamento = null
export const caricaMarchio = () => {
  caricamento ??= fetch('/brand/brand.json', { cache: 'no-cache' })
    .then(r => r.ok ? r.json() : {})
    .catch(_ => ({}))
    .then(b => {
      if (!b || typeof b !== 'object') return
      marchio.ragioneSociale = testo(b.ragione_sociale, 120)
      marchio.informativa = https(b.informativa)
      marchio.nomeAccesso = testo(b.nome_accesso, 60)
      marchio.logoScuro = file(b.logo_scuro)
      marchio.iconaAccesso = file(b.icona_accesso)
    })
  return caricamento
}

// Versione dell'API (/api/version), per il piede; vuota se non risponde.
export const versioneApi = () => fetch('/api/version')
  .then(r => r.json())
  .then(r => r.code === 0 && typeof r.data === 'string' ? r.data.trim() : '')
  .catch(_ => '')
