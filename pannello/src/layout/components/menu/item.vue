<template>
  <!-- Un gruppo (Area personale, Amministrazione) e' un'intestazione con le voci sotto, sempre aperte. -->
  <el-menu-item-group v-if="route.children&&route.children.filter(c=>!c.meta?.hide).length>1&&route.children.some(r => !r.meta?.hide)"
                      :key="route.name"
                      :title="collassato ? '' : (T(route.meta?.title) || T(route.name))"
  >
    <menu-item v-for="(_route,_index) in route.children"
               :route="_route"
               :key="_route.name">
    </menu-item>
  </el-menu-item-group>
  <!-- Con la barra chiusa resta solo l'icona: il nome va nel title. -->
  <el-menu-item v-else-if="!parseRoute(route).meta?.hide" :route="parseRoute(route)" :index="parseRoute(route).name"
                :title="collassato ? (T(parseRoute(route).meta?.title) || T(parseRoute(route).name)) : undefined">
    <el-icon v-if="parseRoute(route).meta?.icon">
      <component :is="`el-icon-${parseRoute(route).meta.icon}`"></component>
    </el-icon>
    <span>{{ T(parseRoute(route).meta?.title) || T(parseRoute(route).name) }}</span>
  </el-menu-item>
</template>

<script>
  import { defineComponent, computed } from 'vue'
  import { T } from '@/utils/i18n'
  import { useAppStore } from '@/store/app'

  export default defineComponent({
    name: 'MenuItem',
    props: {
      route: {},
    },
    mounted () {
    },
    setup (props) {
      //判断仅有一个子项的route
      const parseRoute = (route) => {
        if (route.children && route.children.filter(c => !c.meta?.hide).length === 1) {
          return route.children.filter(c => !c.meta?.hide)[0]
        } else {
          return route
        }
      }
      const app = useAppStore()
      const collassato = computed(() => app.setting.sideIsCollapse)
      return {
        collassato,
        parseRoute,
        T,
      }
    },
  })
</script>

<style lang="scss" scoped>
</style>
