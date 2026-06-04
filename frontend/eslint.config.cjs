const vue = require('eslint-plugin-vue')
const tsParser = require('@typescript-eslint/parser')

module.exports = [
  {
    ignores: [
      'dist/**',
      'node_modules/**',
    ],
  },
  ...vue.configs['flat/essential'],
  {
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      parser: tsParser,
      parserOptions: {
        ecmaVersion: 'latest',
        sourceType: 'module',
      },
    },
  },
  {
    files: ['**/*.vue'],
    languageOptions: {
      parserOptions: {
        parser: tsParser,
        ecmaVersion: 'latest',
        sourceType: 'module',
      },
    },
  },
  {
    rules: {
      'vue/multi-word-component-names': 'off',
      'vue/no-reserved-component-names': 'off',
      'vue/no-side-effects-in-computed-properties': 'off',
      'vue/no-mutating-props': 'off',
      'vue/no-v-text-v-html-on-component': 'off',
      'vue/no-unused-components': 'off',
      'vue/no-unused-vars': 'off',
      'vue/no-use-v-if-with-v-for': 'off',
      'vue/require-v-for-key': 'off',
      'vue/valid-v-for': 'off',
      'vue/valid-v-slot': 'off',
    },
  },
]