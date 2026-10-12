export function get_suffix (filename) {
  var pos = filename.lastIndexOf('.')
  var suffix = ''
  if (pos !== -1) {
    suffix = filename.substring(pos)
  }
  return suffix
}

export function random_string (len) {
  len = len || 32
  var chars = 'ABCDEFGHJKMNPQRSTWXYZabcdefhijkmnprstwxyz2345678'
  var maxPos = chars.length
  var pwd = ''
  for (let i = 0; i < len; i++) {
    pwd += chars.charAt(Math.floor(Math.random() * maxPos))
  }
  return pwd
}

export function random_filename (filename) {
  var suffix = get_suffix(filename)
  var time = new Date()
  var time2 = new Date('2020/01/01')
  return Math.ceil((time.getTime() - time2.getTime()) / 1000) + '_' + random_string(10) + suffix
}

export function utf8_to_b64 (str) {
  return window.btoa(unescape(encodeURIComponent(str)))
}

export function b64_to_utf8 (str) {
  return decodeURIComponent(escape(window.atob(str)))
}

// Una cella che comincia con = + - @, tab o \r diventa una formula quando il
// CSV si apre in un foglio di calcolo (CSV injection, OWASP): prende davanti
// un apostrofo. Lo stesso dopo ogni ; , \r o \n del valore, anche seguiti da
// virgolette o spazi, perche' Excel in italiano separa le celle ai ; e le
// righe agli a capo anche dentro le virgolette. L'a capo messo davanti al
// valore fa valere la stessa regola all'inizio. Un numero puro e il
// segnaposto "-" restano come sono: non sono formule. null e undefined sono
// una cella vuota.
function csvCell (value) {
  let s = value === null || value === undefined ? '' : String(value)
  if (!/^(-|-?\d+(\.\d+)?)$/.test(s)) {
    s = ('\n' + s).replace(/([;,\r\n][" ]*)(?=[=+\-@\t\r])/g, "$1'").slice(1)
  }
  return `"${s.replaceAll('"', '""')}"`
}

export function jsonToCsv (data) {
  let csv = ''
  if (data && data.length > 0) {
    const keys = Object.keys(data[0])
    csv += keys.join(',') + '\n'
    data.forEach(row => {
      csv += keys.map(key => csvCell(row[key])).join(',') + '\n'
    })
  }
  return new Blob([csv], { type: 'text/csv' })
}

export function downBlob (blob, filename) {
  const url = window.URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  setTimeout(() => {
    window.URL.revokeObjectURL(url)
    document.body.removeChild(a)
  })
}

export function sizeFormat (size) {
  if (size < 1024) {
    return size + 'B'
  } else if (size < 1024 * 1024) {
    return (size / 1024).toFixed(2) + 'KB'
  } else if (size < 1024 * 1024 * 1024) {
    return (size / 1024 / 1024).toFixed(2) + 'MB'
  } else {
    return (size / 1024 / 1024 / 1024).toFixed(2) + 'GB'
  }
}
