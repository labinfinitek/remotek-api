import it from '@/utils/i18n/it.json'
import en from '@/utils/i18n/en.json'
import { useAppStore } from '@/store/app'

const trans = {
  'it': it,
  'en': en,
}
export function T (key, params, num = 0) {
  const appStore = useAppStore()
  const lang = appStore.setting.lang
  const tran = trans[lang]?.[key]
  if (!tran) {
    return key
  }
  const msg = num > 1 ? (tran.Other ? tran.Other : tran.One) : tran.One
  //msg 是这样 {name} is name
  //params 是这样 {name: 'zhangsan'}
  //替换
  return msg.replace(/{(\w+)}/g, function (match, key) {
    return params[key] || match
  })
}
