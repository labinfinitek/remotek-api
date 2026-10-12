import request from '@/utils/request'

// Chiude la sessione sul server. Timeout corto: se l'API non risponde,
// l'uscita non deve aspettare i 50 secondi delle altre richieste.
export function logout () {
  return request({
    url: '/logout',
    method: 'post',
    timeout: 5000,
  })
}

export function loginOptions () {
  return request({
    url: '/login-options',
    method: 'get',
  })
}

export function oidcAuth (data) {
  return request({
    url: '/oidc/auth',
    method: 'post',
    data,
  })
}

export function oidcQuery (params) {
  return request({
    url: '/oidc/auth-query',
    method: 'get',
    params,
  })
}

export function captcha () {
  return request({
    url: '/captcha',
    method: 'get',
  })
}
