<template>
  <el-config-provider :locale="appStore.setting.locale.value">
    <el-container :style="{'--sideBarWidth': sideBarWidth}">
      <el-aside :width="leftWidth" class="app-left">
        <g-aside></g-aside>
      </el-aside>
      <el-container class="app-container ">
        <el-header class="app-header">
          <g-header></g-header>
        </el-header>
        <div class="header-tags">
          <tags></tags>
        </div>

        <el-main class="app-main">
          <router-view v-slot="{ Component }">
            <transition mode="out-in" name="el-fade-in-linear">
              <keep-alive :include="cachedTags">
                <component :is="Component"/>
              </keep-alive>
            </transition>
          </router-view>
        </el-main>
        <el-footer class="app-footer" height="auto">
          <g-footer></g-footer>
        </el-footer>
      </el-container>
    </el-container>
  </el-config-provider>
</template>

<script setup>
  import { useAppStore } from '@/store/app'
  import { useTagsStore } from '@/store/tags'
  import { ref, computed } from 'vue'
  import Tags from '@/layout/components/tags/index.vue'
  import GAside from '@/layout/components/aside.vue'
  import GHeader from '@/layout/components/header.vue'
  import GFooter from '@/layout/components/footer.vue'

  const appStore = useAppStore()
  const tagStore = useTagsStore()
  const sideBarWidth = computed(() => appStore.setting.locale.sideBarWidth)
  const leftWidth = computed(() => appStore.setting.sideIsCollapse ? '64px' : 'var(--sideBarWidth)')

  const cachedTags = ref([])

  cachedTags.value = tagStore.cached
</script>

<style lang="scss" scoped>
// Barra in alto chiara (navy nel tema scuro) con una riga blu sottile sotto.
.app-header {
  background-color: var(--rk-barra-alto);
  color: var(--rk-barra-alto-testo);
  border-bottom: 2px solid var(--el-color-primary);
  display: flex;
  height: 56px;
}

.header-tags {
  height: auto;
  background-color: var(--el-bg-color);
  border-bottom: 1px solid var(--el-border-color-light);
  display: flex;
  flex-wrap: wrap;
  padding: 0 8px;
}

.app-main {
  background-color: var(--el-bg-color-page);
}

.app-footer {
  background-color: var(--el-bg-color-page);
  padding: 0;
}

.app-left {
  transition: width 0.5s;
}

.app-container {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}
</style>


