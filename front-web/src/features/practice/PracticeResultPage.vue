<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { request, ApiError } from '@/api/client'
import type { ResultSession } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import AppStatus from '@/components/AppStatus.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { aiAnalysisStatusText, formatDateTime, formatPercent } from '@/app/format'
import ResultItem from './ResultItem.vue'

const route = useRoute()
const sessionID = computed(() => route.params.sessionId as string)

const result = ref<ResultSession | null>(null)
const pageState = ref<'loading' | 'ready' | 'error' | 'notfound'>('loading')
const errorMessage = ref('')
const requestID = ref('')
const retrying = ref(false)
const retryError = ref('')

let timer: ReturnType<typeof setInterval> | null = null

async function load(silent = false): Promise<void> {
  if (!silent) pageState.value = 'loading'
  try {
    result.value = await request<ResultSession>(`/practice-sessions/${sessionID.value}/result`)
    pageState.value = 'ready'
    schedulePolling()
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      pageState.value = 'notfound'
      return
    }
    if (err instanceof ApiError && err.code === 'practice_not_submitted') {
      await routeReplacePractice()
      return
    }
    errorMessage.value = err instanceof ApiError ? err.message : '加载失败'
    requestID.value = err instanceof ApiError ? err.requestId ?? '' : ''
    pageState.value = 'error'
  }
}

async function routeReplacePractice(): Promise<void> {
  window.location.assign(`/practice/${sessionID.value}`)
}

// 批次为 grading 时每 3 秒轮询；页面不可见时暂停，恢复可见后立即请求一次。
function schedulePolling(): void {
  const grading = result.value?.status === 'grading'
  if (grading && timer === null) {
    timer = setInterval(() => {
      if (!document.hidden) {
        void load(true)
      }
    }, 3000)
  }
  if (!grading && timer !== null) {
    clearInterval(timer)
    timer = null
  }
}

function onVisibility(): void {
  if (!document.hidden && result.value?.status === 'grading') {
    void load(true)
  }
}

async function retryAnalysis(): Promise<void> {
  if (retrying.value || result.value?.aiAnalysis.status !== 'failed') return
  retrying.value = true
  retryError.value = ''
  try {
    result.value = await request<ResultSession>(`/practice-sessions/${sessionID.value}/analysis/retry`, { method: 'POST' })
    schedulePolling()
  } catch (err) {
    retryError.value = err instanceof ApiError ? err.message : '重新分析失败，请重试'
  } finally {
    retrying.value = false
  }
}

onMounted(() => {
  void load()
  document.addEventListener('visibilitychange', onVisibility)
})
onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
  document.removeEventListener('visibilitychange', onVisibility)
})

const summary = computed(() => result.value?.summary)
const confirmedAccuracy = computed(() => summary.value?.confirmed.accuracy ?? null)
const aiDone = computed(() => (summary.value?.ai.completed ?? 0) + (summary.value?.ai.pending ?? 0) + (summary.value?.ai.failed ?? 0))
</script>

<template>
  <AppShell>
    <AppStatus v-if="pageState === 'loading'" state="loading" />
    <AppStatus v-else-if="pageState === 'error'" state="error" :message="errorMessage" :request-id="requestID" @action="load()" />
    <AppStatus v-else-if="pageState === 'notfound'" state="empty" message="练习不存在，或它不属于当前账号。" action-label="返回学习概览" @action="() => $router.push('/')" />
    <template v-else-if="result">
      <div class="page-header" style="align-items: center">
        <h1 style="font-size: 24px; margin: 0">练习结果</h1>
        <StatusBadge :value="result.status" kind="session" />
      </div>
      <p class="muted mono">提交于 {{ formatDateTime(result.submittedAt) }}</p>

      <div v-if="result.status === 'grading'" class="card" role="status">
        <p>确定性判分已完成，AI 分析进行中…已确定的成绩如下，你可以离开页面稍后回来。</p>
      </div>

      <div class="metrics">
        <div class="metric">
          <p class="value">{{ summary?.confirmed.correct ?? 0 }} / {{ summary?.confirmed.total ?? 0 }}</p>
          <p class="label">已确认正确数（官方与已审核答案）</p>
          <p class="muted mono">正式正确率 {{ formatPercent(confirmedAccuracy) }}</p>
        </div>
        <div class="metric">
          <p class="value">{{ summary?.ai.completed ?? 0 }}</p>
          <p class="label">AI 判定完成<template v-if="aiDone > 0">（共 {{ aiDone }} 题走 AI）</template></p>
          <p class="muted mono">其中正确 {{ summary?.ai.correct ?? 0 }} · 不计入正式正确率</p>
        </div>
        <div class="metric">
          <p class="value">{{ (summary?.ai.pending ?? 0) + (summary?.ai.failed ?? 0) }}</p>
          <p class="label">待分析 / 失败</p>
          <p class="muted mono">AI 待定 {{ summary?.ai.pending ?? 0 }} · 失败 {{ summary?.ai.failed ?? 0 }}</p>
        </div>
      </div>

      <section class="card" aria-labelledby="ai-analysis-title">
        <div style="display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap">
          <h2 id="ai-analysis-title" style="font-size: 20px; margin: 0">本批 AI 总结</h2>
          <span class="tag" :data-tone="result.aiAnalysis.status === 'failed' ? 'danger' : result.aiAnalysis.status === 'completed' ? 'success' : 'accent'">
            {{ aiAnalysisStatusText[result.aiAnalysis.status] ?? result.aiAnalysis.status }}
          </span>
          <button v-if="result.aiAnalysis.status === 'failed' || result.aiAnalysis.status === 'pending'" type="button" :disabled="retrying || result.aiAnalysis.status === 'pending'" @click="retryAnalysis">
            {{ result.aiAnalysis.status === 'pending' || retrying ? '重新分析中…' : '重新分析' }}
          </button>
        </div>
        <p v-if="retryError" class="error" role="alert">{{ retryError }}</p>
        <p v-if="result.aiAnalysis.status === 'pending'" class="muted" style="margin: 12px 0 0">
          正在根据整批作答情况整理表现、薄弱点和下一步建议…
        </p>
        <p v-else-if="result.aiAnalysis.status === 'not_requested'" class="muted" style="margin: 12px 0 0">
          该批次没有生成 AI 总结。
        </p>
        <p v-else style="margin: 12px 0 0; white-space: pre-wrap">{{ result.aiAnalysis.text }}</p>
      </section>

      <section aria-label="逐题解析" style="display: flex; flex-direction: column; gap: 18px">
        <ResultItem v-for="item in result.items" :key="item.id" :item="item" />
      </section>
    </template>
  </AppShell>
</template>
