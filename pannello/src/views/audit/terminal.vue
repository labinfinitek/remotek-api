<template>
  <el-dialog v-model="visible" :title="T('Transcript') + ' #' + (conn && conn.id)" width="900" destroy-on-close>
    <div v-loading="loading">
      <el-alert v-if="failed" type="error" :title="T('VerifyFailed')" :closable="false"/>
      <el-alert v-if="exportError" type="error" :title="T('ExportFailed') + ' ' + exportError" :closable="false"/>
      <template v-if="verify">
        <el-empty v-if="!verify.blocchi" :description="T('NoTranscript')"/>
        <template v-else>
          <el-alert v-if="read < verify.blocchi" type="error" :closable="false"
                    :title="T('IncompleteTranscript', { read: String(read), expected: String(verify.blocchi) })"/>
          <template v-else>
            <p>
              <el-tag v-if="verify.integra" type="success">{{ T('Intact') }}</el-tag>
              <el-tag v-else type="danger">{{ T('Altered') }}: {{ T('FirstBadBlock') }} {{ verify.primo_errato }}</el-tag>
              <el-tag v-if="verify.fine" type="info">{{ T('SessionEnded') }}</el-tag>
              <el-tag v-else type="warning">{{ T('SessionNotEnded') }}</el-tag>
              {{ T('Blocks') }}: {{ verify.blocchi }}
            </p>
            <p>{{ T('FinalHash') }}: <code class="hash">{{ verify.hash }}</code></p>
          </template>
          <pre class="transcript"><span v-for="p in parts" :key="p.seq" :class="p.dir === 'in' ? 'input' : ''" :title="p.dir === 'in' ? T('Input') : ''">{{ p.text }}</span></pre>
        </template>
      </template>
    </div>
    <template #footer>
      <el-button :loading="exporting" :disabled="!conn" @click="download">{{ T('Export') }}</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
  import { ref } from 'vue'
  import { terminalExport, terminalList, terminalVerify } from '@/api/audit'
  import { T } from '@/utils/i18n'

  const visible = ref(false)
  const loading = ref(false)
  const failed = ref(false)
  const conn = ref(null)
  const verify = ref(null)
  const parts = ref([])
  const read = ref(0)
  const exporting = ref(false)
  const exportError = ref('')
  let last = 0

  // Blocchi in ordine di seq: l'output com'e' (sequenze di controllo
  // comprese, come testo), l'input segnato. Un decoder per verso, perche' un
  // carattere UTF-8 puo' stare a cavallo di due blocchi. Solo l'ultima
  // apertura scrive nella finestra: una precedente ancora in corso, dopo
  // ogni await, vede di non esserlo piu' ed esce. Si mostrano al massimo i
  // blocchi contati dalla verifica; se ne arrivano meno, la trascrizione e'
  // incompleta e non si mostra come integra.
  const open = async (row) => {
    const mine = ++last
    conn.value = row
    verify.value = null
    parts.value = []
    read.value = 0
    failed.value = false
    exportError.value = ''
    exporting.value = false
    visible.value = true
    loading.value = true
    const decoders = { in: new TextDecoder(), out: new TextDecoder() }
    const query = { audit_conn_id: row.id, page: 1, page_size: 50 }
    const v = await terminalVerify(query).catch(_ => false)
    if (mine !== last) {
      return
    }
    const got = []
    const lastOf = {}
    while (v && got.length < v.data.blocchi) {
      const res = await terminalList(query).catch(_ => false)
      if (mine !== last) {
        return
      }
      if (!res || !res.data.list.length) {
        break
      }
      for (const b of res.data.list.slice(0, v.data.blocchi - got.length)) {
        const bytes = Uint8Array.from(atob(b.data), c => c.charCodeAt(0))
        const k = decoders[b.dir] ? b.dir : 'out'
        lastOf[k] = got.length
        got.push({ seq: b.seq, dir: b.dir, text: decoders[k].decode(bytes, { stream: true }) })
      }
      query.page++
    }
    // A fine lettura si svuota ogni decoder: un carattere troncato in coda
    // all'ultimo blocco di un verso esce come U+FFFD invece di sparire.
    for (const k in lastOf) {
      got[lastOf[k]].text += decoders[k].decode()
    }
    failed.value = !v
    parts.value = got
    read.value = got.length
    verify.value = v ? v.data : null
    loading.value = false
  }
  // Scarica il file della trascrizione (GET .../terminal/export) col token
  // del pannello; un errore resta scritto nella finestra, se e' ancora
  // quella della stessa apertura.
  const download = async () => {
    const mine = last
    const id = conn.value.id
    exporting.value = true
    exportError.value = ''
    const res = await terminalExport({ audit_conn_id: id }).catch(e => ({ error: (e && e.message) || 'error' }))
    if (mine !== last) {
      return
    }
    exporting.value = false
    if (res.error) {
      exportError.value = res.error
      return
    }
    const url = URL.createObjectURL(new Blob([res.data], { type: 'application/json' }))
    const a = document.createElement('a')
    a.href = url
    a.download = 'trascrizione-' + id + '.json'
    a.click()
    setTimeout(() => URL.revokeObjectURL(url), 1000)
  }
  defineExpose({ open })
</script>

<style scoped lang="scss">
  .hash { word-break: break-all; }
  .transcript { max-height: 60vh; overflow: auto; white-space: pre-wrap; word-break: break-all; background: #1e1e1e; color: #ddd; padding: 8px; }
  .input { background: #3a3d7a; color: #fff; }
</style>
