import { createI18n } from 'vue-i18n'
import en from './en'
import zhcn from './zhcn'

const supportedLocales = ['zhHans', 'en']
const detectBrowserLocale = () => {
  const browserLocale = navigator.language.toLowerCase()
  return browserLocale.startsWith('zh') ? 'zhHans' : 'en'
}
const fallbackLocale = detectBrowserLocale()
const savedLocale = localStorage.getItem("locale")
const currentLocale = savedLocale && supportedLocales.includes(savedLocale) ? savedLocale : fallbackLocale

export const i18n = createI18n({
  legacy: false,
  locale: currentLocale,
  fallbackLocale: 'en',
  messages: {
    en: en,
    zhHans: zhcn,
  },
})

export const locale = (() => {
  const l = i18n.global.locale.value
  switch (l) {
    case "zhHans":
      return "zh-cn"
    default:
      return l
  }
})()

export const languages = [
  { title: '中文', value: 'zhHans' },
  { title: 'English', value: 'en' },
]
