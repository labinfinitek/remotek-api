<template>
  <div>
    <el-card shadow="hover">
      <template #header>{{ T('Userinfo') }}</template>
      <el-form class="info-form" ref="form" label-width="150px" label-suffix=":">
        <el-form-item :label="T('User')">
          <div>{{ userStore.nickname || userStore.username }}<span v-if="userStore.nickname" class="secondario"> · {{ userStore.username }}</span></div>
        </el-form-item>
        <el-form-item :label="T('Email')">
          <div>{{ userStore.email || '—' }}</div>
        </el-form-item>
        <el-form-item :label="T('LinkedAccount')">
          <div>{{ oidcData.filter(o => o.status === 1).map(o => o.op).join(', ') || T('NoBind') }}</div>
        </el-form-item>
        <el-form-item v-if="!disablePwd" :label="T('Password')" prop="password">
          <el-button @click="showChangePwd">{{ T('ChangePassword') }}</el-button>
        </el-form-item>
        <el-form-item label="OIDC">
          <el-table :data="oidcData" border fit>
            <el-table-column :label="T('IdP')" prop="op" align="center"></el-table-column>
            <el-table-column :label="T('Status')" prop="status" align="center">
              <template #default="{ row }">
                <el-tag v-if="row.status === 1" type="success">{{ T('HasBind') }}</el-tag>
                <el-tag v-else type="danger">{{ T('NoBind') }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="T('Actions')" align="center" width="200">
              <template #default="{ row }">
                <el-button v-if="row.status === 1" link type="primary" @click="toUnBind(row)">{{ T('UnBind') }}</el-button>
                <el-button v-else link type="primary" @click="toBind(row)">{{ T('ToBind') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-form-item>
      </el-form>
    </el-card>
    <changePwdDialog v-model:visible="changePwdVisible"></changePwdDialog>
  </div>
</template>

<script setup>
  import changePwdDialog from '@/components/changePwdDialog.vue'
  import { ref } from 'vue'
  import { useUserStore } from '@/store/user'
  import { bind, unbind } from '@/api/oauth'
  import { myOauth } from '@/api/user'
  import { loginOptions } from '@/api/login'
  import { ElMessageBox } from 'element-plus'
  import { T } from '@/utils/i18n'

  const userStore = useUserStore()
  const changePwdVisible = ref(false)
  const showChangePwd = () => {
    changePwdVisible.value = true
  }
  // Con app.disable-pwd-login la password non serve: il pulsante non compare.
  const disablePwd = ref(true)
  loginOptions().then(res => { disablePwd.value = !!res.data.disable_pwd }).catch(_ => false)
  const oidcData = ref([])
  const getMyOauth = async () => {
    const res = await myOauth().catch(_ => false)
    if (res) {
      oidcData.value = res.data
    }

  }
  getMyOauth()
  const toBind = async (row) => {
    const res = await bind({ op: row.op }).catch(_ => false)
    if (res) {
      const { code, url } = res.data
      window.open(url)
    }
  }
  const toUnBind = async (row) => {
    const cf = await ElMessageBox.confirm(T('Confirm?', { param: T('UnBind') }), {
      confirmButtonText: T('Confirm'),
      cancelButtonText: T('Cancel'),
      type: 'warning',
    }).catch(_ => false)
    if (!cf) {
      return false
    }
    const res = await unbind({ op: row.op }).catch(_ => false)
    if (res) {
      getMyOauth()
    }

  }

</script>

<style scoped lang="scss">
.info-form {
  width: 600px;
  margin: 0 auto;
}

.secondario {
  color: var(--el-text-color-secondary);
}
</style>
