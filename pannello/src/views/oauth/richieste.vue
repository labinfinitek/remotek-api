<template>
  <div>
    <el-card class="list-query" shadow="hover">
      <el-form inline>
        <el-form-item>
          <el-button type="primary" @click="getList">{{ T('Filter') }}</el-button>
        </el-form-item>
        <el-form-item>
          <span class="nota">{{ T('OauthPendingNote') }}</span>
        </el-form-item>
      </el-form>
    </el-card>
    <el-card class="list-body" shadow="hover">
      <el-table :data="lista" v-loading="loading" border :empty-text="T('OauthPendingEmpty')">
        <el-table-column :label="T('OauthPendingCode')" align="center" width="150">
          <template #default="{row}"><span class="codice">{{ row.codice }}</span></template>
        </el-table-column>
        <el-table-column prop="op" :label="T('IdP')" align="center" width="130"/>
        <el-table-column prop="nome" :label="T('Name')" align="center" min-width="150"/>
        <el-table-column :label="T('Email')" align="center" min-width="220">
          <template #default="{row}">
            {{ row.email }}
            <el-tag v-if="row.email" size="small" :type="row.email_verificata ? 'success' : 'info'">
              {{ row.email_verificata ? T('OauthPendingVerified') : T('OauthPendingNotVerified') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="T('OauthPendingAge')" align="center" width="130">
          <template #default="{row}">{{ T('OauthPendingMinutes', { n: String(Math.floor(row.secondi / 60)) }) }}</template>
        </el-table-column>
        <el-table-column :label="T('Actions')" align="center" width="140">
          <template #default="{row}">
            <el-button link type="primary" @click="apri(row)">{{ T('OauthPendingBindTo') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
    <el-dialog v-model="visibile" :title="T('OauthPendingBindTo')" width="500">
      <el-form label-width="100px">
        <el-form-item :label="T('OauthPendingCode')">
          <span class="codice">{{ scelta.codice }}</span>
        </el-form-item>
        <el-form-item :label="T('User')">
          <el-select v-model="utente" filterable style="width: 100%">
            <el-option v-for="u in utenti" :key="u.id" :label="etichetta(u)" :value="u.id"/>
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="visibile = false">{{ T('Cancel') }}</el-button>
        <el-button type="primary" :disabled="!utente" @click="associa">{{ T('Submit') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
  import { onMounted, onActivated, ref } from 'vue'
  import { richieste, associaRichiesta } from '@/api/oauth'
  import { groupUsers } from '@/api/user'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import { T } from '@/utils/i18n'

  const lista = ref([])
  const loading = ref(false)
  const getList = async () => {
    loading.value = true
    const res = await richieste().catch(_ => false)
    loading.value = false
    if (res) {
      lista.value = res.data
    }
  }
  onMounted(getList)
  onActivated(getList)

  // groupUsers da' solo utenti attivi e non agenti: gli altri l'API li
  // rifiuta comunque.
  const utenti = ref([])
  const etichetta = (u) => u.nickname ? `${u.nickname} (${u.username})` : u.username
  const visibile = ref(false)
  const scelta = ref({})
  const utente = ref(null)
  const apri = async (row) => {
    scelta.value = row
    utente.value = null
    visibile.value = true
    const res = await groupUsers().catch(_ => false)
    if (res) {
      utenti.value = res.data.users
    }
  }
  const associa = async () => {
    const u = utenti.value.find(x => x.id === utente.value)
    const cf = await ElMessageBox.confirm(T('OauthPendingConfirm', { codice: scelta.value.codice, utente: etichetta(u) }), {
      confirmButtonText: T('Confirm'),
      cancelButtonText: T('Cancel'),
      type: 'warning',
    }).catch(_ => false)
    if (!cf) {
      return
    }
    const res = await associaRichiesta({ codice: scelta.value.codice, user_id: utente.value }).catch(_ => false)
    if (res) {
      ElMessage.success(T('OperationSuccess'))
      visibile.value = false
      getList()
    }
  }
</script>

<style scoped lang="scss">
.codice {
  font-family: ui-monospace, "SF Mono", Consolas, monospace;
  font-weight: bold;
  letter-spacing: 0.08em;
}

.nota {
  color: var(--el-text-color-secondary);
}
</style>
