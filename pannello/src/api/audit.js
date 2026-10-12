import request from '@/utils/request'
import { T } from '@/utils/i18n'

export function list (params) {
  return request({
    url: '/audit_conn/list',
    params,
  })
}

export function remove (data) {
  return request({
    url: '/audit_conn/delete',
    method: 'post',
    data,
  })
}

export function batchDelete (data) {
  return request({
    url: '/audit_conn/batchDelete',
    method: 'post',
    data,
  })
}

export function fileList (params) {
  return request({
    url: '/audit_file/list',
    params,
  })
}

export function fileRemove (data) {
  return request({
    url: '/audit_file/delete',
    method: 'post',
    data,
  })
}

export function fileBatchDelete (data) {
  return request({
    url: '/audit_file/batchDelete',
    method: 'post',
    data,
  })
}

export function terminalList (params) {
  return request({
    url: '/audit_conn/terminal/list',
    params,
  })
}

export function terminalVerify (params) {
  return request({
    url: '/audit_conn/terminal/verify',
    params,
  })
}

// Il file della trascrizione e' JSON ma non ha code: lo si avvolge in
// {code: 0, data: testo} perche' passi l'intercettore come le altre
// risposte; un errore dell'API ({code, message}) resta com'e'. Il file
// comincia sempre con {"connessione":. Un corpo che non e' JSON non viene
// dall'API (per esempio la pagina HTML di un 502 del proxy): l'errore dice
// lo stato HTTP invece di quello di JSON.parse. Niente timeout: il file
// arriva a pezzi e puo' metterci.
export function terminalExport (params) {
  return request({
    url: '/audit_conn/terminal/export',
    params,
    timeout: 0,
    responseType: 'text',
    transformResponse: [(text, headers, status) => {
      if (text.startsWith('{"connessione":')) {
        return { code: 0, data: text }
      }
      try {
        return JSON.parse(text)
      } catch (_) {
        throw new Error(T('HttpStatusResponse', { status }))
      }
    }],
  })
}
