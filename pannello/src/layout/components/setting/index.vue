<template>
  <div class="setting">
    <div class="menu-item">
      <el-switch
          v-model="isDark"
          style="--el-switch-on-color:#18222c"
      >
        <template #active-action>
          <el-icon>
            <Moon/>
          </el-icon>
        </template>
        <template #inactive-action>
          <el-icon>
            <Sunny color="#000"/>
          </el-icon>
        </template>
      </el-switch>
    </div>
    <el-dropdown class="menu-item">
      <!-- La sigla della lingua attiva, come testo: il glifo di prima sembrava cinese. -->
      <div class="title lingua" :aria-label="`${T('Language')}: ${sigla}`">{{ sigla }}</div>
      <template #dropdown>
        <el-dropdown-menu>
          <el-dropdown-item v-for="(v, k) in appStore.setting.langs" @click="changeLang(k)" :key="k">{{ v.name }}</el-dropdown-item>
        </el-dropdown-menu>
      </template>
    </el-dropdown>
    <el-dropdown class="menu-item">
      <div class="title">
        <!--        <el-image class="avatar" :src="user.avatar"></el-image>-->
        <span class="nickname">{{ user.username }}</span>
        <el-icon>
          <el-icon-arrow-down/>
        </el-icon>

      </div>

      <template #dropdown>
        <el-dropdown-menu>
          <el-dropdown-item @click="showChangePwd">{{ T('ChangePassword') }}</el-dropdown-item>
          <el-dropdown-item @click="logout">{{ T('Logout') }}</el-dropdown-item>
        </el-dropdown-menu>
      </template>
    </el-dropdown>
    <changePwdDialog v-model:visible="changePwdVisible"></changePwdDialog>
  </div>
</template>

<script setup>
  import { useUserStore } from '@/store/user'
  import { useAppStore } from '@/store/app'
  import changePwdDialog from '@/components/changePwdDialog.vue'
  import { ref, computed } from 'vue'
  import { T } from '@/utils/i18n'
  import { useDark } from '@vueuse/core'
  import { Sunny, Moon } from '@element-plus/icons'

  const userStore = useUserStore()
  const user = userStore
  const appStore = useAppStore()

  const logout = async () => {
    await userStore.esci()
    window.location.reload()
  }

  const changePwdVisible = ref(false)
  const showChangePwd = () => {
    changePwdVisible.value = true
  }
  // L'etichetta accessibile contiene la sigla che si vede (WCAG 2.5.3).
  const sigla = computed(() => appStore.setting.lang.toUpperCase())
  const changeLang = (v) => {
    appStore.changeLang(v)
  }
  const isDark = useDark()
  // const toggleDark = useToggle(isDark)
</script>

<style lang="scss" scoped>
.setting {
  margin-left: auto;
  display: flex;
  align-items: center;
  justify-content: space-around;

  .menu-item {
    margin-left: 15px;

    * {
      outline: none;
    }
  }

  .title {
    color: #fff;
    display: flex;
    align-items: center;
    justify-content: space-around;


    .nickname {
      padding: 0 10px;
    }

    &.lingua {
      height: 24px;
      font-size: 14px;
      font-weight: 600;
      cursor: pointer;
    }
  }
}
</style>
