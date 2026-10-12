<template>
  <el-card class="simple-card" shadow="hover" v-loading="form.loading">
    <template #header>
      <div class="card-header">
        <span>{{ T('CmdUsage') }}</span>
      </div>
    </template>
    <el-form :disabled="!canSend">
      <el-form-item>
        <el-table :data="form.list" size="small">
          <el-table-column prop="0" label="IP"></el-table-column>
          <el-table-column prop="1" :label="T('UsageTime')"></el-table-column>
          <el-table-column prop="2" :label="T('UsageTotal')"></el-table-column>
          <el-table-column prop="3" :label="T('UsageHighest')"></el-table-column>
          <el-table-column prop="4" :label="T('UsageAvg')"></el-table-column>
          <el-table-column prop="5" :label="T('UsageSpeed')"></el-table-column>
        </el-table>
      </el-form-item>
      <el-form-item>
        <el-button @click="getList">{{ T('Refresh') }}</el-button>
      </el-form-item>
    </el-form>
  </el-card>
</template>
<script setup>

  import { T } from '@/utils/i18n'
  import { reactive, watch } from 'vue'
  import { sendCmd } from '@/api/rustdesk'
  import { RELAY_TARGET } from '@/views/rustdesk/options'

  const props = defineProps({
    canSend: Boolean,
  })

  const form = reactive({
    get_cmd: 'u',
    list: [],
    target: RELAY_TARGET,
    loading: false,
  })
  const getList = async () => {
    form.loading = true
    const res = await sendCmd({ cmd: form.get_cmd, target: RELAY_TARGET }).catch(_ => false)
    form.loading = false
    if (res) {
      form.list = res.data.split('\n').filter(i => i).map(i => i.split(" "))
    }
  }
  watch(() => props.canSend, (v) => {
    if (v) {
      getList()
    }
  })


</script>
<style scoped lang="scss">
.simple-card{
  width: 500px;
}
</style>
