<script lang="ts" setup>
import { HumanReadable } from '@/plugins/utils'
import { computed } from 'vue'

const props = defineProps({
  tilesData: <any>{},
  type: String
})

const data = computed(() => {
  const d = props.tilesData
  if (!d) return { percent: 0, text: '-' }
  switch (props.type) {
    case 'g-cpu': {
      const cpu = Number(d.cpu)
      if (d.cpu == null || !Number.isFinite(cpu)) return { percent: 0, text: '-' }
      const percent = Math.max(0, Math.min(100, cpu))
      return { percent, text: Math.ceil(percent) + '%' }
    }
    case 'g-mem':
      return gaugeData(d.mem)
    case 'g-dsk':
      return gaugeData(d.dsk)
    case 'g-swp':
      return gaugeData(d.swp)
  }
  return { percent: 0, text: '-'}
})

const gaugeData = (d:any) :any => {
  if (!d) return { percent: 0, text: '-' }
  const currentValue = Number(d.current)
  const totalValue = Number(d.total)
  if (d.current == null || !Number.isFinite(currentValue) || currentValue < 0 || !Number.isFinite(totalValue) || totalValue <= 0) return { percent: 0, text: '-' }
  const curr = currentValue === 0 ? ['0', ''] : HumanReadable.sizeFormat(currentValue,0).split(' ')
  const total = HumanReadable.sizeFormat(totalValue,0).split(' ')
  if (curr[1] == total[1]) curr[1] = ''
  return {
    percent: Math.min(100, Math.ceil(currentValue*100/totalValue)),
    text: `${curr[0]}${curr[1]?? ''}/${total[0]}${total[1]?? ''}`
  }
}

const cssTransformRotateValue = computed(() => {
  const percentageAsFraction = data.value.percent / 100
  const halfPercentage = percentageAsFraction / 2

  return `${halfPercentage}turn`
})

const gaugeColor = computed(() => {
  if (data.value.percent > 90) return 'error'
  if (data.value.percent > 70) return 'warning'
  return 'info'
})
</script>

<template>
  <div class="gauge__outer">
    <div class="gauge__inner">
      <div
        class="gauge__fill"
        :style="{
          transform: `rotate(${cssTransformRotateValue})`,
          background: `rgb(var(--v-theme-${gaugeColor}))`
          }">
      </div>
      <div class="gauge__cover"><span dir="ltr">{{ data.text }}</span></div>
    </div>
  </div>
</template>

<style scoped>
.gauge__outer {
  width: 100%;
  max-width: 250px;
}

.gauge__inner {
  width: 100%;
  height: 0;
  padding-bottom: 50%;
  background: rgb(var(--v-theme-surface));
  position: relative;
  border-top-left-radius: 100% 200%;
  border-top-right-radius: 100% 200%;
  overflow: hidden;
}

.gauge__fill {
  position: absolute;
  top: 100%;
  left: 0;
  width: inherit;
  height: 100%;
  background: rgb(var(--v-theme-primary));
  transform-origin: center top;
  transform: rotate(0turn);
  transition: transform 0.2s ease-out;
}

.gauge__cover {
  width: 75%;
  height: 150%;
  background: rgb(var(--v-theme-background));
  position: absolute;
  top: 25%;
  left: 50%;
  transform: translateX(-50%);
  border-radius: 50%;

  /* Text */
  display: flex;
  align-items: center;
  justify-content: center;
  padding-bottom: 25%;
  box-sizing: border-box;
  font-family: 'Lexend', sans-serif;
  font-weight: bold;
  font-size: 32px;
}

sup {
  font-size: 16px;
}
</style>
