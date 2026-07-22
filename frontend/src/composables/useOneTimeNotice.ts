import { ref } from 'vue'

export const useOneTimeNotice = (storageKey: string) => {
  const visible = ref(localStorage.getItem(storageKey) !== 'seen')

  if (visible.value) {
    localStorage.setItem(storageKey, 'seen')
  }

  const show = () => {
    visible.value = true
  }

  return { visible, show }
}
