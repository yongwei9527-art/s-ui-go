// Plugins
import vue from '@vitejs/plugin-vue'
import vuetify, { transformAssetUrls } from 'vite-plugin-vuetify'

// Utilities
import { defineConfig } from 'vitest/config'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  base: '',
  plugins: [
    vue({
      template: { transformAssetUrls },
    }),
    vuetify({
      autoImport: true,
      styles: {
        configFile: 'src/styles/settings.scss',
      },
    })
  ],
  build: {
    manifest: false,
    outDir: 'dist',
    chunkSizeWarningLimit: 2000,
    rollupOptions: {
      output: {
        // Keep application routes as async chunks and split only stable, shared
        // third-party dependencies. Grouping package families avoids duplicate
        // modules while keeping rarely used features out of the first request.
        manualChunks (id) {
          const moduleId = id.replace(/\\/g, '/')

          if (!moduleId.includes('/node_modules/')) return

          if (
            moduleId.includes('/node_modules/vue/') ||
            moduleId.includes('/node_modules/@vue/') ||
            moduleId.includes('/node_modules/vue-router/') ||
            moduleId.includes('/node_modules/pinia/')
          ) return 'vendor-vue'

          if (moduleId.includes('/node_modules/vuetify/')) return 'vendor-vuetify'

          if (
            moduleId.includes('/node_modules/chart.js/') ||
            moduleId.includes('/node_modules/vue-chartjs/')
          ) return 'vendor-charts'

          if (
            moduleId.includes('/node_modules/moment/') ||
            moduleId.includes('/node_modules/vue3-persian-datetime-picker/')
          ) return 'vendor-date'

          if (
            moduleId.includes('/node_modules/axios/') ||
            moduleId.includes('/node_modules/clipboard/') ||
            moduleId.includes('/node_modules/notivue/') ||
            moduleId.includes('/node_modules/qrcode.vue/') ||
            moduleId.includes('/node_modules/vue-i18n/') ||
            moduleId.includes('/node_modules/yaml/')
          ) return 'vendor-utilities'
        },
        entryFileNames: 'assets/[name]-[hash].js',
        chunkFileNames: 'assets/[name]-[hash].js',
        assetFileNames: 'assets/[name]-[hash][extname]',
      },
    }
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.ts'],
  },
  define: { 'process.env': {} },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
    extensions: ['.js', '.json', '.jsx', '.mjs', '.ts', '.tsx', '.vue'],
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:2095',
        changeOrigin: true,
        rewrite: path => path.replace(/^\/api/, '/app/api'),
      },
      '/app/api': {
        target: 'http://localhost:2095',
        changeOrigin: true,
      },
    },
  }
})
