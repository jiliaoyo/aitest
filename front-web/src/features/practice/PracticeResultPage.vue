<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { request, ApiError } from '@/api/client'
import type { ResultSession } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import AppStatus from '@/components/AppStatus.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { aiAnalysisStatusText, formatAIText, formatDateTime, formatPercent } from '@/app/format'
import ResultItem from './ResultItem.vue'
import FuriganaText from '@/components/FuriganaText.vue'

const route = useRoute()
const sessionID = computed(() => route.params.sessionId as string)

const result = ref<ResultSession | null>(null)
const pageState = ref<'loading' | 'ready' | 'error' | 'notfound'>('loading')
const errorMessage = ref('')
const requestID = ref('')
const retrying = ref(false)
const retryError = ref('')
const pollingError = ref('')
const masteryUpdatingID = ref('')
const masteryError = ref('')

let timer: ReturnType<typeof setInterval> | null = null
let requestSequence = 0
let requestInFlight = false
let requestController: AbortController | null = null

async function load(silent = false): Promise<void> {
  if (silent && requestInFlight) return
  const sequence = ++requestSequence
  const targetSessionID = sessionID.value
  requestController?.abort()
  const controller = new AbortController()
  requestController = controller
  requestInFlight = true
  if (!silent) pageState.value = 'loading'
  try {
    const next = await request<ResultSession>(`/practice-sessions/${targetSessionID}/result`, { signal: controller.signal })
    if (sequence !== requestSequence) return
    result.value = next
    pageState.value = 'ready'
    pollingError.value = ''
    schedulePolling()
  } catch (err) {
    if (sequence !== requestSequence || controller.signal.aborted) return
    if (silent && result.value) {
      pollingError.value = err instanceof ApiError ? err.message : '刷新失败，稍后自动重试'
      return
    }
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
  } finally {
    if (sequence === requestSequence) {
      requestInFlight = false
      requestController = null
    }
  }
}

async function routeReplacePractice(): Promise<void> {
  window.location.assign(`/practice/${sessionID.value}`)
}

// 批次为 grading 时每 3 秒轮询；页面不可见时暂停，恢复可见后立即请求一次。
function schedulePolling(): void {
  const waiting = result.value?.status === 'grading' || result.value?.aiAnalysis.status === 'pending'
  if (waiting && timer === null) {
    timer = setInterval(() => {
      if (!document.hidden) {
        void load(true)
      }
    }, 3000)
  }
  if (!waiting && timer !== null) {
    clearInterval(timer)
    timer = null
  }
}

function onVisibility(): void {
  if (!document.hidden && (result.value?.status === 'grading' || result.value?.aiAnalysis.status === 'pending')) {
    void load(true)
  }
}

watch(sessionID, (next, previous) => {
  if (next === previous) return
  result.value = null
  pollingError.value = ''
  void load()
})

async function retryAnalysis(): Promise<void> {
  const aiStatus = result.value?.aiAnalysis.status
  const failedCount = result.value?.summary.ai.failed ?? 0
  if (retrying.value || aiStatus === 'pending' || (aiStatus !== 'failed' && failedCount === 0 && retryableExplanationCount.value === 0)) return
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

async function toggleMastered(item: ResultSession['items'][number]): Promise<void> {
  if (!item.questionId || item.masteryAvailable === false || masteryUpdatingID.value) return
  const mastered = item.mastered !== true
  masteryUpdatingID.value = item.id
  masteryError.value = ''
  try {
    await request(`/review-items/${item.questionId}/mastered`, { method: mastered ? 'POST' : 'DELETE' })
    item.mastered = mastered
  } catch (err) {
    masteryError.value = err instanceof ApiError ? err.message : '更新掌握状态失败，请重试'
  } finally {
    masteryUpdatingID.value = ''
  }
}

onMounted(() => {
  void load()
  document.addEventListener('visibilitychange', onVisibility)
})
onBeforeUnmount(() => {
  requestSequence++
  requestController?.abort()
  if (timer) clearInterval(timer)
  document.removeEventListener('visibilitychange', onVisibility)
})

const summary = computed(() => result.value?.summary)
const confirmedAccuracy = computed(() => summary.value?.confirmed.accuracy ?? null)
const aiDone = computed(() => (summary.value?.ai.completed ?? 0) + (summary.value?.ai.pending ?? 0) + (summary.value?.ai.failed ?? 0))
const failedAI = computed(() => summary.value?.ai.failed ?? 0)
const retryableExplanationCount = computed(() => result.value?.items.filter((item) => item.explanation?.source === 'ai' && item.explanation.text.startsWith('AI 解析语言异常')).length ?? 0)
const showRetryButton = computed(() => result.value?.aiAnalysis.status === 'failed' || result.value?.aiAnalysis.status === 'pending' || failedAI.value > 0 || retryableExplanationCount.value > 0)
const retryButtonLabel = computed(() => {
  if (retrying.value || result.value?.aiAnalysis.status === 'pending') return '重试中…'
  if (failedAI.value > 0) return '重试失败题目'
  return retryableExplanationCount.value > 0 ? '重试失败解析' : '重新分析'
})
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

      <div v-if="result.status === 'grading' || result.aiAnalysis.status === 'pending'" class="card" role="status">
        <p>已有的成绩已记录，AI 分析进行中…你可以离开页面稍后回来。</p>
        <p v-if="pollingError" class="muted" role="status">{{ pollingError }}</p>
      </div>

      <div class="metrics">
        <div class="metric">
          <p class="value">{{ summary?.confirmed.correct ?? 0 }} / {{ summary?.confirmed.total ?? 0 }}</p>
          <p class="label">已确认正确数（官方与已审核答案）</p>
          <p class="muted mono">正式正确率 {{ formatPercent(confirmedAccuracy) }}</p>
        </div>
        <div class="metric">
          <p class="value">{{ summary?.ai.completed ?? 0 }}</p>
          <p class="label">AI 来源结果<template v-if="aiDone > 0">（共 {{ aiDone }} 题）</template></p>
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
          <button v-if="showRetryButton" type="button" :disabled="retrying || result.aiAnalysis.status === 'pending'" @click="retryAnalysis">
            {{ retryButtonLabel }}
          </button>
        </div>
        <p v-if="retryError" class="error" role="alert">{{ retryError }}</p>
        <p v-if="retryableExplanationCount > 0 && result.aiAnalysis.status !== 'pending'" class="muted" style="margin: 12px 0 0">
          有 {{ retryableExplanationCount }} 条解析语言异常，可重新请求 AI 分析。
        </p>
        <p v-if="result.aiAnalysis.status === 'pending'" class="muted" style="margin: 12px 0 0">
          正在根据整批作答情况整理表现、薄弱点和下一步建议…
        </p>
        <p v-else-if="result.aiAnalysis.status === 'not_requested'" class="muted" style="margin: 12px 0 0">
          该批次没有生成 AI 总结。
        </p>
        <p v-else class="ai-text" style="margin: 12px 0 0"><FuriganaText :text="formatAIText(result.aiAnalysis.text)" /></p>
      </section>

      <section aria-label="逐题解析" style="display: flex; flex-direction: column; gap: 18px">
        <p v-if="masteryError" class="error-summary" role="alert">{{ masteryError }}</p>
        <ResultItem
          v-for="item in result.items"
          :key="item.id"
          :item="item"
          :mastery-updating="masteryUpdatingID === item.id"
          @toggle-mastered="toggleMastered(item)"
        />
      </section>

      <section class="card" aria-labelledby="next-practice-title">
        <h2 id="next-practice-title" style="font-size: 18px; margin-top: 0">继续练习</h2>
        <p class="muted">可以从本批知识点继续专项练习，或打开错题本复习当前账号的待重练题目。</p>
        <div style="display: flex; gap: 10px; flex-wrap: wrap">
          <RouterLink v-for="knowledgePoint in [...new Map(result.items.flatMap((item) => item.knowledgePoints).map((item) => [item.id, item])).values()]" :key="knowledgePoint.id" class="tag" :to="`/knowledge/${knowledgePoint.id}`">
            巩固{{ knowledgePoint.name }}
          </RouterLink>
          <RouterLink class="tag" to="/wrong-items">查看错题本</RouterLink>
        </div>
      </section>
    </template>
  </AppShell>
</template>
