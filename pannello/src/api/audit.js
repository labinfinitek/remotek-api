import request from '@/utils/request'

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
// comincia sempre con {"connessione":. Niente timeout: il file arriva a
// pezzi e puo' metterci.
export function terminalExport (params) {
  return request({
    url: '/audit_conn/terminal/export',
    params,
    timeout: 0,
    responseType: 'text',
    transformResponse: [text => text.startsWith('{"connessione":') ? { code: 0, data: text } : JSON.parse(text)],
  })
}
