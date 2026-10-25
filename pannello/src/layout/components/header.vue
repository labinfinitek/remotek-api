<template>
  <el-icon class="ex-icon" @click="expandOrFoldSlider">
    <el-icon-expand v-if="setting.sideIsCollapse"></el-icon-expand>
    <el-icon-fold v-else></el-icon-fold>
  </el-icon>
  <div class="header-logo">
    <img :src="logo" alt="" class="logo">
    <div class="title">{{setting.title}}</div>
  </div>
  <Setting></Setting>
</template>

<script>
  import { defineComponent, computed } from 'vue'
  import HeaderMenu from '@/layout/components/menu/index.vue'
  import Setting from '@/layout/components/setting/index.vue'
  import { useAppStore } from '@/store/app'
  import GTags from '@/layout/components/tags/index.vue'
  import { useDark } from '@vueuse/core'
  import { marchio, caricaMarchio } from '@/utils/marchio'

  export default defineComponent({
    name: 'LayerHeader',
    created () {
    },
    components: { HeaderMenu, Setting, GTags },
    watch: {},
    setup (props) {
      const appStore = useAppStore()
      const setting = computed(() => appStore.setting)
      // Nel tema scuro il logo per il fondo navy di brand.json, se c'e'.
      const scuro = useDark()
      caricaMarchio()
      const logo = computed(() => scuro.value && marchio.logoScuro ? marchio.logoScuro : setting.value.logo)
      const expandOrFoldSlider = () => {
        appStore.sideCollapse()
      }
      return {
        setting,
        logo,
        expandOrFoldSlider,
      }
    },

  })
</script>

<style scoped lang="scss">
  .ex-icon {
    height: 100%;
    display: flex;
    align-items: center;
    margin-right: 10px;
    font-size: 18px;
    cursor: pointer;
    color: var(--rk-barra-alto-testo);
  }

  .header-logo {
    display: flex;
    height: 100%;
    align-items: center;

    .title {
      display: block;
      margin-left: 12px;
      font-family: var(--rk-font-titoli);
      font-size: 20px;
      font-weight: 700;
      color: var(--rk-barra-alto-testo);
    }

    // Altezza fissa e larghezza dal logo: un marchio largo non si rimpicciolisce.
    .logo {
      display: block;
      height: 32px;
      width: auto;
      max-width: 160px;
      object-fit: contain;
    }
  }


</style>
<style lang="scss">

</style>
