<template>
  <Line v-if="loaded" :data="data" :options="<any>options" />
</template>

<script lang="ts">
import { ref } from 'vue'
import { Line } from 'vue-chartjs'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Filler,
} from 'chart.js'
import { HumanReadable } from '@/plugins/utils'
ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Filler
)
ChartJS.defaults.font.family = 'Vazirmatn'
export default {
  components: {
    Line
  },
  props: ['tilesData','type'],
  data() {
    return {
      loaded: false,
      labels: new Array(20).fill(''),
      oldValues: <any>{net: {}, dio: {}},
      options1: {
        animation: false,
        responsive: true,
        maintainAspectRatio: false,
        interaction: {
          intersect: false,
          mode: 'index',
        },
        plugins: {
          tooltip: {
            enabled: false
          },
          legend: {
              display: false,
          }
        },
        scales: {
          y: {
            min: 0,
            max: 100,
            grid: {
              color: '#777777',
            },
            beginAtZero: true,
            ticks: {
                beginAtZero: true,
                steps: 10,
                stepValue: 5,
                max: 100
            }
          }
        }
      },
      optionsNet: {
        animation: false,
        responsive: true,
        maintainAspectRatio: false,
        interaction: {
          intersect: false,
          mode: 'index',
        },
        plugins: {
          tooltip: {
            enabled: false
          },
          legend: {
              display: false,
          }
        },
        scales: {
          y: {
            grid: {
              color: '#777777',
            },
            beginAtZero: true,
            ticks: {
              callback: (label:any, index: number) => { return parseInt(label).toString() },
              count: 10
            }
          }
        }
      },
      data: ref(<any>{})
    }
  },
  computed: {
    options() {
      switch (this.$props.type){
        case "h-net":
          this.optionsNet.scales.y.ticks.callback = (label:any, index: number) => {
            return label == 0 ? "0" : HumanReadable.sizeFormat(label,0)
          }
          return this.optionsNet
        case "hp-net":
          this.optionsNet.scales.y.ticks.callback = (label:any, index: number) => {
            return label == 0 ? "0" : HumanReadable.packetFormat(label,0)
          }
          return this.optionsNet
        case "h-dio":
          this.optionsNet.scales.y.ticks.callback = (label:any, index: number) => {
            return label == 0 ? "0" : HumanReadable.sizeFormat(label,0)
          }
          return this.optionsNet
      }
      return this.options1
    }
  },
  methods: {
    updateRates(value: any, key: string, up: string, down: string) {
      const current = value?.[key]
      if (!Number.isFinite(current?.[up]) || !Number.isFinite(current?.[down])) {
        this.oldValues[key] = {}
        return
      }
      const previous = this.oldValues[key]
      if (Number.isFinite(previous?.[up]) && Number.isFinite(previous?.[down])) {
        // Counter resets/restarts are not negative traffic. Legacy tiles sample every 2s.
        this.updateData2(Math.max(0, current[up] - previous[up]) / 2, Math.max(0, current[down] - previous[down]) / 2)
      }
      this.oldValues[key] = { ...current }
    },
    updateData1(value1: number) {
      if (!Number.isFinite(value1)) return
      const newData = [...(this.data.datasets?.[0]?.data ?? []), value1].slice(-20)
      this.data = {
        labels: this.labels,
        datasets: [
          {
            label: '',
            backgroundColor: 'rgba(255, 165, 0, 0.2)',
            borderColor: 'rgba(255, 165, 0,0.8)',
            fill: true,
            data: newData
          }
        ],
      }
      this.loaded = true
    },
    updateData2(value1: number, value2:number) {
      if (!Number.isFinite(value1) || !Number.isFinite(value2)) return
      const newData1 = [...(this.data.datasets?.[0]?.data ?? []), value1].slice(-20)
      const newData2 = [...(this.data.datasets?.[1]?.data ?? []), value2].slice(-20)
      this.data = {
        labels: this.labels,
        datasets: [
          {
            label: '',
            backgroundColor: 'rgba(255, 165, 0, 0.2)',
            borderColor: 'rgba(255, 165, 0,0.8)',
            fill: true,
            data: newData1
          },
          {
            label: '',
            backgroundColor: 'rgba(0, 128, 0, 0.1)',
            borderColor: 'rgba(0, 128, 0,0.8)',
            fill: true,
            data: newData2
          }
        ],
      }
      this.loaded = true
    }
  },
  watch: {
    type() {
      this.data = {}
      this.loaded = false
      this.oldValues = { net: {}, dio: {} }
    },
    tilesData: {
      immediate: true,
      handler(value: any) {
        switch (this.type) {
          case 'h-cpu':
            if (Number.isFinite(value?.cpu)) this.updateData1(Math.max(0, Math.min(100, value.cpu)))
            break
          case 'h-mem':
            if (Number.isFinite(value?.mem?.current) && Number.isFinite(value?.mem?.total) && value.mem.total > 0) {
              this.updateData1(Math.max(0, Math.min(100, value.mem.current * 100 / value.mem.total)))
            }
            break
          case 'h-net':
            this.updateRates(value, 'net', 'sent', 'recv')
            break
          case 'hp-net':
            this.updateRates(value, 'net', 'psent', 'precv')
            break
          case 'h-dio':
            this.updateRates(value, 'dio', 'write', 'read')
            break
        }
      }
    }
  }
}
</script>