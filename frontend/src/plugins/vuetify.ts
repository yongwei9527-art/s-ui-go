/**
 * plugins/vuetify.ts
 *
 * Framework documentation: https://vuetifyjs.com`
 */

// Styles
import '@mdi/font/css/materialdesignicons.css'
import 'vuetify/styles/main.css'

import colors from 'vuetify/util/colors'
import { en, zhHans } from 'vuetify/locale'

// Composables
import { createVuetify } from 'vuetify'

const supportedLocales = ['zhHans', 'en']
const detectBrowserLocale = () => {
  const browserLocale = navigator.language.toLowerCase()
  return browserLocale.startsWith('zh') ? 'zhHans' : 'en'
}
const fallbackLocale = detectBrowserLocale()
const savedLocale = localStorage.getItem("locale")
const currentLocale = savedLocale && supportedLocales.includes(savedLocale) ? savedLocale : fallbackLocale

// https://vuetifyjs.com/en/introduction/why-vuetify/#feature-guides
export default createVuetify({
  defaults: {
    VRow: { density: 'compact' },
    VTextField: {
      variant: 'solo-filled',
    },
    VSelect: {
      variant: 'solo-filled',
    },
    VCombobox: {
      variant: 'solo-filled',
    },
    VTextarea: {
      variant: 'solo-filled',
    },
  },
  theme: {
    defaultTheme: localStorage.getItem('theme') ?? 'system',
    themes: {
      light: {
        colors: {
          error: '#FF5252',
          background: '#e8f0f6',
          surface: '#f6f8fb',
          surfaceVariant: '#d5dee7',
        },
      },
      dark: {
        colors: {
          primary: colors.blue.darken4,
          error: colors.red.accent3,
        },
      },
    },
  },
  locale: {
    locale: currentLocale,
    fallback: 'en',
    messages: { en, zhHans },
  },
})
