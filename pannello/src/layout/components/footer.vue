<template>
  <div class="piede">
    <span>{{ appStore.setting.title }}<template v-if="marchio.ragioneSociale"> · {{ marchio.ragioneSociale }}</template></span>
    <span v-if="versione">API {{ versione }}</span>
  </div>
</template>

<script setup>
  import { onMounted, ref } from 'vue'
  import { marchio, caricaMarchio, versioneApi } from '@/utils/marchio'
  import { useAppStore } from '@/store/app'

  // Il nome e' brand.name (titolo da /api/admin/config/admin, Remotek di default).
  const appStore = useAppStore()

  const versione = ref('')
  onMounted(async () => {
    caricaMarchio()
    versione.value = await versioneApi()
  })
</script>

<style scoped lang="scss">
.piede {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  padding: 10px 20px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  border-top: 1px solid var(--el-border-color-light);
}
</style>
