import { defineStore, acceptHMRUpdate } from 'pinia'
const logo = '/brand/logo.svg'
import it from 'element-plus/es/locale/lang/it'
import en from 'element-plus/es/locale/lang/en'
import { admin, server } from '@/api/config'

export const langs = {
  'it': { name: 'Italiano', value: it, sideBarWidth: '260px' },
  'en': { name: 'English', value: en, sideBarWidth: '230px' },
}
// La lingua salvata vale solo se e' una delle due; altrimenti italiano,
// senza guardare navigator.language.
export const linguaIniziale = () => {
  const salvata = localStorage.getItem('lang')
  return Object.keys(langs).includes(salvata) ? salvata : 'it'
}
const defaultLang = linguaIniziale()
export const useAppStore = defineStore({
  id: 'App',
  state: () => ({
    setting: {
      title: 'Remotek',
      hello: '',
      sideIsCollapse: false,
      logo,
      langs: langs,
      lang: defaultLang,
      locale: langs[defaultLang],
      rustdeskConfig: {
        'id_server': '',
        'key': '',
        'relay_server': '',
        'api_server': '',
      },
    },
  }),

  actions: {
    sideCollapse () {
      this.setting.sideIsCollapse = !this.setting.sideIsCollapse
    },
    setLang (lang) {
      console.log('setLang', lang)
      this.setting.lang = lang
      this.setting.locale = langs[lang]
      localStorage.setItem('lang', lang)
    },
    changeLang (v) {
      this.setLang(v)
    },
    loadConfig () {
      this.getAdminConfig()
      this.loadRustdeskConfig()
    },
    getAdminConfig () {
      console.log('getAdminConfig')
      return admin().then(res => {
        this.replaceAdminTitle(res.data.title)
        this.setting.hello = res.data.hello
      })
    },
    replaceAdminTitle (newTitle) {
      document.title = document.title.replace(`- ${this.setting.title}`, `- ${newTitle}`)
      this.setting.title = newTitle
    },
    async loadRustdeskConfig () {
      console.log('loadRustdeskConfig')
      const res = await server().catch(_ => false)
      if (res) {
        this.setting.rustdeskConfig = res.data
      }

    },
  },
})

if (import.meta.hot) {
  import.meta.hot.accept(acceptHMRUpdate(useAppStore, import.meta.hot))
}
