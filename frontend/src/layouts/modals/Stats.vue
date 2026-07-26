<template>
  <v-dialog transition="dialog-bottom-transition" width="800">
    <v-card class="rounded-lg" :loading="loading">
      <v-card-title>
        <v-row>
          <v-col cols="auto">
            {{ $t('stats.graphTitle') }}
          </v-col>
          <v-spacer></v-spacer>
          <v-col cols="auto"><v-icon icon="mdi-close" @click="$emit('close')"></v-icon></v-col>
        </v-row>
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text style="padding: 0 16px;">
        <div style="text-align: center; margin: 5px;">
          {{ $t('objects.' + resource) + " : " + tag }}
        </div>
        <v-radio-group v-model="limit" @change="loadData" density="compact" :loading="loading" inline hide-details>
          <v-radio v-for="p in periods" :label="p.title" :value="p.value"></v-radio>
        </v-radio-group>
          <v-container id="container" style="height:40vh;">
            <v-skeleton-loader
            class="mx-auto border"
            width="95%"
            type="image"
            v-if="loading"
          ></v-skeleton-loader>
          <template v-else>
            <v-alert :text="$t('noData')" type="warning" variant="outlined" v-if="alert"></v-alert>
            <Line v-if="loaded" :data="usage" :options="<any>options" />
          </template>
        </v-container>
      </v-card-text>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import { i18n } from '@/locales'
import HttpUtils from '@/plugins/httputil'
import { HumanReadable } from '@/plugins/utils'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler,
} from 'chart.js'
import { ref } from 'vue'
import { Line } from 'vue-chartjs'

import {
  beginStatsRequest,
  failStatsRequest,
  finishStatsRequest,
  getStatsRecords,
  invalidateStatsRequests,
  stopStatsPolling,
} from './Stats.request'
import { aggregateTrafficByBucket } from './Stats.traffic'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
)
ChartJS.defaults.font.family = 'Vazirmatn'
export default {
  components: {
    Line
  },
  props: ['visible','resource','tag'],
  data() {
    return {
      loading: false,
      loaded: false,
      alert: false,
      intervalId: <any>0,
      requestId: 0,
      limit: 1,
      periods: [
        { value: 1, title: i18n.global.n(1) + i18n.global.t('date.h')},
        { value: 6, title: i18n.global.n(6) + i18n.global.t('date.h')},
        { value: 12, title: i18n.global.n(12) + i18n.global.t('date.h')},
        { value: 24, title: i18n.global.n(1) + i18n.global.t('date.d')},
        { value: 48, title: i18n.global.n(2) + i18n.global.t('date.d')},
        { value: 240, title: i18n.global.n(10) + i18n.global.t('date.d')},
        { value: 480, title: i18n.global.n(20) + i18n.global.t('date.d')},
        { value: 720, title: i18n.global.n(30) + i18n.global.t('date.d')},
        { value: 1440, title: i18n.global.n(60) + i18n.global.t('date.d')},
        { value: 2160, title: i18n.global.n(90) + i18n.global.t('date.d')},
      ],
      options: {
        responsive: true,
        maintainAspectRatio: false,
        interaction: {
          intersect: false,
          mode: 'index',
        },
        elements: {
          point: { pointStyle: 'crossRot' }
        },
        plugins: {
          tooltip: {
            callbacks: {
              text: (ctx:any) => {
                const {axis = 'xy', intersect, mode} = ctx.chart.options.interaction
                return 'Mode: ' + mode + ', axis: ' + axis + ', intersect: ' + intersect
              },
              footer: (items:any[]) => {
                return HumanReadable.sizeFormat(items.reduce((acc, c) => acc + c.raw, 0))
              }
            }
          }
        },
        scales: {
          y: {
            grid: {
              color: '#777777',
            },
            beginAtZero: true,
            ticks: {
              callback: function(label:any, index: number) {
                return label == 0 ? 0 : HumanReadable.sizeFormat(label,0)
              },
              count: 10
            }
          }
        }
      },
      usage: ref(<any>{}),
    }
  },
  methods: {
    async loadData() {
      const requestId = beginStatsRequest(this)

      try {
        const data = await HttpUtils.get('api/stats', { resource: this.resource, tag: this.tag, limit: this.limit })
        const obj = getStatsRecords(this, requestId, data)
        if (obj === null) {
          return
        }
        const l = String(i18n.global.locale) == 'fa' ? "fa-IR" : "en-US"
        const oneStep = this.limit * 3600 * 1000 / 360 // Each 10 sec
        const now = new Date().getTime()
        const steps = <number[]>[]
        for (let i = 360; i >= 0; i--) {
          steps.push(now - (oneStep * i))
        }
        const labels = <string[]>[]
        for (let i = 1; i < 360; i++) {
          labels.push(this.genLable(steps[i], l))
        }
        const { uplink: uplinkData, downlink: downlinkData } = aggregateTrafficByBucket(obj, steps.slice(0, 360))
        this.usage = {
          labels: labels,
          datasets: [
            {
              label: i18n.global.t('stats.upload'),
              backgroundColor: 'rgba(255, 165, 0, 0.4)',
              borderColor: 'rgba(255, 165, 0)',
              fill: true,
              data: uplinkData
            },
            {
              label: i18n.global.t('stats.download'),
              backgroundColor: 'rgba(0, 128, 0, 0.2)',
              borderColor: 'rgba(0, 128, 0)',
              fill: true,
              data: downlinkData
            }
          ],
        }
        this.loaded = true
        this.alert = false
      } catch {
        failStatsRequest(this, requestId)
      } finally {
        finishStatsRequest(this, requestId)
      }
    },
    stopPolling() {
      this.intervalId = stopStatsPolling(this.intervalId)
    },
    resetStats() {
      invalidateStatsRequests(this)
      this.loaded = false
      this.alert = false
      this.usage = { labels: [], datasets: [] }
    },
    genLable(step:number, locale: string) {
      return new Date(step).toLocaleString(locale,{
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        hour12: false,
      })
    },
  },
  beforeUnmount() {
    this.stopPolling()
    invalidateStatsRequests(this)
  },
  watch: {
    visible(v) {
      if (v) {
        this.stopPolling()
        this.limit = 1
        this.loadData()
        this.intervalId = setInterval(() => {
          this.loadData()
        }, 10000)
      } else {
        this.stopPolling()
        this.resetStats()
      }
    }
  }
}
</script>
