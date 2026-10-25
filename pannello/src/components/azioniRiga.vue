<template>
  <div class="azioni-riga">
    <slot/>
    <el-dropdown trigger="click">
      <el-button link type="primary">{{ T('OtherActions') }}<el-icon class="el-icon--right"><ArrowDown/></el-icon></el-button>
      <template #dropdown>
        <el-dropdown-menu>
          <slot name="altro"/>
          <el-dropdown-item class="azione-elimina" :divided="!!$slots.altro" @click="emit('elimina')">{{ testoElimina || T('Delete') }}</el-dropdown-item>
        </el-dropdown-menu>
      </template>
    </el-dropdown>
  </div>
</template>

<script setup>
  import { ArrowDown } from '@element-plus/icons'
  import { T } from '@/utils/i18n'

  // Colonna "Azioni" delle liste: al massimo due azioni visibili (nello slot),
  // le altre nel menu "Altro" (slot altro). "Elimina" sta sempre nel menu, in
  // rosso; la conferma la chiede chi ascolta l'evento.
  defineProps({ testoElimina: String })
  const emit = defineEmits(['elimina'])
</script>

<style scoped lang="scss">
.azioni-riga {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
  white-space: nowrap;

  .el-button + .el-button {
    margin-left: 0;
  }
}
</style>
