const TokenKey = 'access_token'
const OidcCode = 'oidc_code'
const OidcCodeExpiry = 'oidc_code_expiry';
// Il codice del login OIDC vale quanto la voce sul server (5 minuti, la
// cache di OidcAuth): con Keycloak login e TOTP superano spesso i 60
// secondi di prima, e al ritorno il codice era gia' scaduto nel browser.
const DURATA_CODICE_OIDC_MS = 5 * 60 * 1000

export function getToken () {
  return localStorage.getItem(TokenKey)
}

export function setToken (token) {
  return localStorage.setItem(TokenKey, token)
}

// Toglie anche la copia per il web client che i login di prima lasciavano
// nel browser: il web client non c'e' piu' e nessuno la legge.
export function removeToken () {
  localStorage.removeItem('wc-option:local:access_token')
  return localStorage.removeItem(TokenKey)
}

// 设置 code，并存储当前时间戳（单位：毫秒）
export function setCode(code) {
  const now = Date.now(); // 当前时间戳（毫秒）
  const expiry = now + DURATA_CODICE_OIDC_MS;

  localStorage.setItem(OidcCode, code); // 存储 code
  localStorage.setItem(OidcCodeExpiry, expiry); // 存储过期时间戳
}

// 获取 code，如果已过期则删除并返回 null
export function getCode() {
  const expiry = localStorage.getItem(OidcCodeExpiry); // 获取过期时间戳
  const now = Date.now(); // 当前时间戳

  if (expiry && now > parseInt(expiry)) {
    // 如果已过期，删除 code 和过期时间
    removeCode();
    return null;
  }
  return localStorage.getItem(OidcCode); // 返回 code（如果未过期）
}

// 删除 code 和过期时间
export function removeCode() {
  localStorage.removeItem(OidcCode);
  localStorage.removeItem(OidcCodeExpiry);
}
