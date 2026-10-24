<template>
  <div class="login-container">
    <div class="login-card">
      <img :src="logo" alt="logo" class="login-logo"/>
      <h1 class="login-titolo">{{ appStore.setting.title }} · {{ T('Console') }}</h1>

      <!-- L'accesso principale e' l'OIDC; la password sta sotto, secondaria. -->
      <div class="oidc-options">
        <el-button v-for="(option, index) in options" :key="index" type="primary" size="large"
                   @click="handleOIDCLogin(option.name)" class="oidc-btn">
          <img :src="iconaAccesso(option.name)" alt="" class="oidc-icon"/>
          <span>{{ T('LoginWith', { name: nomeAccesso(option.name) }) }}</span>
        </el-button>
      </div>

      <div class="divider" v-if="options.length > 0 && !disablePwd">
        <span>{{ T('OrWithPassword') }}</span>
      </div>

      <el-form v-if="!disablePwd" label-position="top" class="login-form">
        <el-form-item :label="T('Username')">
          <el-input v-model="form.username" type="username" class="login-input"></el-input>
        </el-form-item>

        <el-form-item :label="T('Password')">
          <el-input v-model="form.password" type="password" @keyup.enter.native="login" show-password
                    class="login-input"></el-input>
        </el-form-item>
        <el-form-item :label="T('Captcha')" v-if="captchaCode">
          <el-input v-model="form.captcha" @keyup.enter.native="login"  class="login-input captcha-input">
            <template #append>
              <img :src="captchaCode.b64" @click="loadCaptcha" class="captcha" alt="captcha"/>
            </template>
          </el-input>
        </el-form-item>
        <el-form-item>
          <el-button @click="login" :type="options.length ? 'default' : 'primary'" class="login-button">{{ T('Login') }}</el-button>
          <el-button v-if="allowRegister" @click="register" class="login-button">{{ T('Register') }}</el-button>
        </el-form-item>
      </el-form>

      <div class="login-piede" v-if="marchio.ragioneSociale || marchio.informativa">
        <span v-if="marchio.ragioneSociale">{{ marchio.ragioneSociale }}</span>
        <a v-if="marchio.informativa" :href="marchio.informativa" target="_blank" rel="noopener noreferrer">{{ T('PrivacyNotice') }}</a>
      </div>
    </div>
  </div>
</template>

<script setup>
  import { reactive, onMounted, ref, computed } from 'vue'
  import { useUserStore } from '@/store/user'
  import { ElMessage } from 'element-plus'
  import { T } from '@/utils/i18n'
  import { useRoute, useRouter } from 'vue-router'
  import { loginOptions, captcha } from '@/api/login'
  import { getCode, removeCode } from '@/utils/auth'
  import { useAppStore } from '@/store/app'
  import { useDark } from '@vueuse/core'
  import { marchio, caricaMarchio } from '@/utils/marchio'

  const appStore = useAppStore()
  const scuro = useDark()
  caricaMarchio()
  const logo = computed(() => scuro.value && marchio.logoScuro ? marchio.logoScuro : '/brand/logo.svg')

  const oauthInfo = ref({})
  const userStore = useUserStore()
  const route = useRoute()
  const router = useRouter()
  const options = reactive([]) // 存储 OIDC 登录选项

  let platform = window.navigator.platform
  if (navigator.platform.indexOf('Mac') === 0) {
    platform = 'mac'
  } else if (navigator.platform.indexOf('Win') === 0) {
    platform = 'windows'
  } else if (navigator.platform.indexOf('Linux armv') === 0) {
    platform = 'android'
  } else if (navigator.platform.indexOf('Linux') === 0) {
    platform = 'linux'
  }
  const userAgent = navigator.userAgent
  let browser = 'Unknown Browser'
  if (/chrome|crios/i.test(userAgent)) browser = 'Chrome'
  else if (/firefox|fxios/i.test(userAgent)) browser = 'Firefox'
  else if (/safari/i.test(userAgent) && !/chrome/i.test(userAgent)) browser = 'Safari'
  else if (/edg/i.test(userAgent)) browser = 'Edge'

  const form = reactive({
    username: '',
    password: '',
    platform: platform,
    captcha: '',
    captcha_id: ''
  })

  const captchaCode = ref('')
  const redirect = route.query?.redirect
  const login = async () => {
    const res = await userStore.login(form).catch(e => e)
    if (!res.code) {
      ElMessage.success(T('LoginSuccess'))
      router.push({ path: redirect || '/', replace: true })
      return
    }
    if (res.code === 110) {
      // need captcha
      loadCaptcha()
    }
  }

  const loadCaptcha = async () => {
    const captchaRes = await captcha().catch(_ => false)
    console.log(captchaRes)
    captchaCode.value = captchaRes.data.captcha
    form.captcha_id = captchaRes.data.captcha.id
  }

  const handleOIDCLogin = (provider) => {
    userStore.oidc(provider, platform, browser)
  }

  import googleImage from '@/assets/google.png'
  import githubImage from '@/assets/github.png'
  import oidcImage from '@/assets/oidc.png'
  import webauthImage from '@/assets/webauth.png'
  import defaultImage from '@/assets/oidc.png'

  const providerImageMap = {
    google: googleImage,
    github: githubImage,
    oidc: oidcImage,
    // WebAuth: webauthImage,
    default: defaultImage,
  }

  const getProviderImage = (provider) => {
    return providerImageMap[provider.toLowerCase()] || providerImageMap.default
  }
  // Nome e icona del pulsante da brand.json solo con un provider: con piu'
  // provider ognuno tiene il suo nome, se no i pulsanti sarebbero uguali.
  const nomeAccesso = (provider) => options.length === 1 && marchio.nomeAccesso ? marchio.nomeAccesso : provider
  const iconaAccesso = (provider) => options.length === 1 && marchio.iconaAccesso ? marchio.iconaAccesso : getProviderImage(provider)

  const allowRegister = ref(false)
  const disablePwd = ref(false)
  // Accesso di emergenza (README): con ?password=1 il modulo della password
  // resta anche col login con password spento, senza andare all'OIDC, per
  // gli username di app.pwd-login-eccezioni.
  const emergenza = route.query?.password === '1'
  const loadLoginOptions = async () => {
    try {
      const res = await loginOptions().catch(_ => false)
      if (!res || !res.data) return console.error('No valid response received')
      res.data.ops.map(option => (options.push({ name: option }))) // 创建新的对象数组
      if (res.data.auto_oidc && !emergenza) {
        // 如果有自动OIDC登录选项，直接调用第一个
        handleOIDCLogin(res.data.ops[0])
      }
      disablePwd.value = res.data.disable_pwd && !emergenza
      allowRegister.value = res.data.register
      if (res.data.need_captcha) {
        loadCaptcha()
      }
    } catch (error) {
      console.error('Error loading login options:', error.message)
    }
  }

  onMounted(async () => {
    const code = getCode()
    if (code) {
      // 如果code存在，进行query获取user info
      const res = await userStore.query(code)
      if (res) {
        // 删除code，确保跳转之前对code进行清楚
        removeCode()
        ElMessage.success(T('LoginSuccess'))
        router.push({ path: redirect || '/', replace: true })
      }
    } else {
      // 如果code不存在, 现实登陆页面
      loadLoginOptions() // 组件挂载后调用登录选项加载函数
    }
  })

  const register = () => {
    router.push('/register')
  }
</script>

<style scoped lang="scss">
// Fondo chiaro e card bianca (nel tema scuro fondo e superficie navy).
.login-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background-color: var(--el-bg-color-page);
  padding: 20px;
  box-sizing: border-box;
}

.login-card {
  width: 380px;
  background-color: var(--el-bg-color);
  border: 1px solid var(--el-border-color-light);
  padding: 40px;
  border-radius: 8px;
  box-shadow: 0 4px 16px rgba(14, 26, 46, 0.08);
  text-align: center;
}

.login-titolo {
  margin: 0 0 28px;
  font-family: var(--rk-font-titoli);
  font-size: 22px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}

.login-form {
  margin-bottom: 8px;
  text-align: left;
}

.login-input {
  width: 100%;
  .captcha{
    cursor: pointer;
    width: 150px;
  }
}
.captcha-input{
  :deep(.el-input-group__append) {
    border-radius: 5px;
    padding: 0;
    overflow: hidden;
  }
}

.login-button {
  width: 100%;
  height: 40px;
  margin-bottom: 12px;
  margin-left: 0;
}

.divider {
  display: flex;
  align-items: center;
  margin: 24px 0 16px;
  font-size: 13px;
  color: var(--el-text-color-secondary);

  &::before,
  &::after {
    content: '';
    flex: 1;
    height: 1px;
    background-color: var(--el-border-color);
  }

  &::before {
    margin-right: 10px;
  }

  &::after {
    margin-left: 10px;
  }
}

.oidc-options {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.oidc-btn {
  width: 100%;
  height: 48px;
  margin-left: 0;
  font-size: 15px;
  font-weight: 600;
}

// L'icona su un tondo chiaro (--rk-fondo-icona).
.oidc-icon {
  width: 22px;
  height: 22px;
  margin-right: 10px;
  padding: 2px;
  border-radius: 50%;
  background-color: var(--rk-fondo-icona);
  object-fit: contain;
}

// Come .logo nella barra: altezza fissa, larghezza dal logo con un massimo.
.login-logo {
  height: 64px;
  width: auto;
  max-width: 300px;
  object-fit: contain;
  margin: 0 auto 16px;
  display: block;
}

.login-piede {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 4px 12px;
  margin-top: 20px;
  font-size: 12px;
  color: var(--el-text-color-secondary);

  a {
    color: var(--rk-accento-testo);
  }
}
</style>
