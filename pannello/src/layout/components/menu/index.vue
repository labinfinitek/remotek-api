<template>
  <el-menu
          class="menus"
          :collapse="isCollapse"
          :default-active="activeIndex"
          router
  >
    <menu-item v-for="(route,index) in routes" :key="route.name" :route="route"></menu-item>
  </el-menu>
</template>

<script>
  import { defineComponent, ref, onMounted, watch, computed } from 'vue'
  import { useRouteStore } from '@/store/router'
  import MenuItem from '@/layout/components/menu/item.vue'
  import { useRoute } from 'vue-router'
  import { useAppStore } from '@/store/app'

  export default defineComponent({
    name: 'Menu',
    created () {
    },
    components: { MenuItem },
    setup () {
      const routes = ref([])
      const route = useRoute()
      const app = useAppStore()
      const isCollapse = computed(() => app.setting.sideIsCollapse)
      const activeIndex = computed(() => route.name)

      routes.value = useRouteStore().routes
      return {
        routes,
        activeIndex,
        isCollapse,
      }
    },

  })
</script>

<style lang="scss" scoped>
  // Navy del sito in tutti e due i temi: voci #D7E0EE, icone #6FA8FF, voce
  // attiva su blu tenue con una barretta blu a sinistra.
  .menus {
    --el-menu-bg-color: var(--rk-navy);
    --el-menu-text-color: var(--rk-laterale-voce);
    --el-menu-active-color: var(--rk-laterale-attiva-testo);
    --el-menu-hover-bg-color: var(--rk-laterale-sopra-fondo);
    --el-menu-item-height: 44px;
    min-height: 100vh;
    border-right: none;

    :deep(.el-icon) {
      color: var(--rk-laterale-icona);
    }

    :deep(.el-menu-item.is-active) {
      background-color: var(--rk-laterale-attiva-fondo);
      box-shadow: inset 3px 0 0 var(--el-color-primary);
    }

    // "Area personale" e "Amministrazione": intestazioni piccole, non voci.
    :deep(.el-menu-item-group__title) {
      padding-top: 16px;
      font-size: 11px;
      font-weight: 600;
      letter-spacing: 0.08em;
      text-transform: uppercase;
      color: var(--rk-laterale-intestazione);
    }

    // Barra chiusa: solo le icone, centrate. Element nasconde il testo solo
    // delle voci figlie dirette del menu, non di quelle dentro un gruppo.
    &.el-menu--collapse {
      :deep(.el-menu-item-group__title) {
        display: none;
      }

      :deep(.el-menu-item) {
        justify-content: center;
        padding: 0 !important;

        > span {
          display: none;
        }

        .el-icon {
          margin-right: 0;
        }
      }
    }

    &:not(.el-menu--collapse) {
      width: var(--sideBarWidth);
    }

  }
</style>
<style>
</style>
