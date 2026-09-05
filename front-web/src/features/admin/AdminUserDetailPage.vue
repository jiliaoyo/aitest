<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { request, ApiError } from '@/api/client'
import type { AdminAIUsageBreakdown, AdminUserDetail } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import AppStatus from '@/components/AppStatus.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { aiRunKindText, formatDateTime, formatInteger, formatUSD, practiceModeText, roleText } from '@/app/format'

const route = useRoute()
const router = useRouter()
const detail = ref<AdminUserDetail | null>(null)
const state = ref<'loading' | 'ready' | 'error'>('loading')
const errorMessage = ref('')
const requestID = ref('')

const filters = reactive({
  from: (route.query.from as string | undefined) ?? '',
  to: (route.query.to as string | undefined) ?? '',
})

function localDate(date: Date): string {
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

function setRange(days: number | null): void {
  if (days === null) {
    filters.from = ''
    filters.to = ''
    return
  }
  const to = new Date()
  const from = new Date(to)
  from.setDate(from.getDate() - days + 1)
  filters.from = localDate(from)
  filters.to = localDate(to)
}

async function load(): Promise<void> {
  state.value = 'loading'
  errorMessage.value = ''
  try {
    const params = new URLSearchParams()
    if (filters.from) params.set('from', filters.from)
    if (filters.to) params.set('to', filters.to)
    const suffix = params.toString() ? `?${params.toString()}` : ''
    detail.value = await request<AdminUserDetail>(`/admin/users/${String(route.params.userId)}${suffix}`)
    state.value = 'ready'
  } catch (err) {
    errorMessage.value = err instanceof ApiError ? err.message : '加载失败'
    requestID.value = err instanceof ApiError ? err.requestId ?? '' : ''
    state.value = 'error'
  }
}

function breakdownCost(row: AdminAIUsageBreakdown): string {
  return formatUSD(row.estimatedCostUsd)
}

function duration(value: number): string {
  return value < 1000 ? `${value} ms` : `${(value / 1000).toFixed(1)} s`
}

watch(filters, () => {
  void router.replace({ query: Object.fromEntries(Object.entries(filters).filter(([, value]) => value)) })
  void load()
}, { deep: true })

watch(() => route.params.userId, () => void load())
onMounted(() => void load())
</script>

<template>
  <AppShell>
    <div class="page-header">
      <div>
        <p style="margin: 0 0 8px"><RouterLink to="/admin/users">← 返回用户列表</RouterLink></p>
        <h1 style="font-size: 24px; margin: 0">用户详情</h1>
      </div>
      <div style="display: flex; gap: 8px; flex-wrap: wrap">
        <button type="button" @click="setRange(7)">近 7 天</button>
        <button type="button" @click="setRange(30)">近 30 天</button>
        <button type="button" @click="setRange(90)">近 90 天</button>
        <button type="button" @click="setRange(null)">全部</button>
      </div>
    </div>
    <div class="card">
      <div class="grid-2">
        <div class="field">
          <label for="detail-from">开始日期</label>
          <input id="detail-from" v-model="filters.from" type="date" />
        </div>
        <div class="field">
          <label for="detail-to">结束日期</label>
          <input id="detail-to" v-model="filters.to" type="date" />
        </div>
      </div>
    </div>

    <AppStatus v-if="state === 'loading'" state="loading" />
    <AppStatus v-else-if="state === 'error'" state="error" :message="errorMessage" :request-id="requestID" @action="load" />
    <template v-else-if="detail">
      <section class="card">
        <div class="page-header">
          <div>
            <h2 style="font-size: 20px; margin: 0">{{ detail.user.email }}</h2>
            <p class="muted" style="margin: 6px 0 0">
              <span class="tag" :data-tone="detail.user.role === 'admin' ? 'accent' : undefined">{{ roleText[detail.user.role] ?? detail.user.role }}</span>
              <span style="margin-left: 8px">默认级别：{{ detail.user.defaultLevelName || '未设置' }}</span>
            </p>
          </div>
          <div class="muted mono" style="font-size: 12px; text-align: right">
            <div>ID {{ detail.user.id }}</div>
            <div>注册 {{ formatDateTime(detail.user.createdAt) }} · 最近登录 {{ formatDateTime(detail.user.lastLoginAt) }} · 最近活跃 {{ formatDateTime(detail.user.lastActiveAt) }}</div>
          </div>
        </div>
      </section>

      <div class="metrics">
        <div class="metric"><p class="value">{{ formatInteger(detail.usage.activeDays) }}</p><p class="label">活跃天数</p></div>
        <div class="metric"><p class="value">{{ formatInteger(detail.usage.practiceSessions) }}</p><p class="label">练习批次（提交 {{ detail.usage.submittedSessions }}）</p></div>
        <div class="metric"><p class="value">{{ formatInteger(detail.usage.practiceItems) }}</p><p class="label">练习题数（已作答 {{ detail.usage.answeredItems }}）</p></div>
      </div>
      <div class="metrics">
        <div class="metric"><p class="value">{{ formatInteger(detail.usage.aiGenerationRequests) }}</p><p class="label">AI 出题请求批次</p></div>
        <div class="metric"><p class="value">{{ formatInteger(detail.usage.ai.generationCalls) }}</p><p class="label">AI 实际出题调用</p></div>
        <div class="metric"><p class="value">{{ formatInteger(detail.usage.ai.totalTokens) }}</p><p class="label">AI token · {{ formatUSD(detail.usage.ai.estimatedCostUsd) }}</p></div>
      </div>

      <div class="grid-2">
        <section class="card">
          <h2 style="font-size: 17px">练习状态</h2>
          <table class="data">
            <tbody>
              <tr><th>已完成批次</th><td class="num">{{ detail.usage.completedSessions }}</td></tr>
              <tr><th>进行中批次</th><td class="num">{{ detail.usage.activeSessions }}</td></tr>
              <tr><th>AI 出题失败批次</th><td class="num">{{ detail.usage.generationFailedSessions }}</td></tr>
              <tr><th>AI 分析失败批次</th><td class="num">{{ detail.usage.analysisFailedSessions }}</td></tr>
              <tr><th>AI 调用成功 / 失败</th><td class="num">{{ detail.usage.ai.successfulCalls }} / {{ detail.usage.ai.failedCalls }}</td></tr>
              <tr><th>AI 总耗时</th><td class="num">{{ formatInteger(detail.usage.ai.durationMs) }} ms</td></tr>
              <tr><th>登录次数</th><td class="num">{{ detail.usage.loginCount }}</td></tr>
              <tr><th>当前有效会话</th><td class="num">{{ detail.usage.activeAuthSessions }}</td></tr>
            </tbody>
          </table>
        </section>
        <section class="card">
          <h2 style="font-size: 17px">AI token 与费用</h2>
          <table class="data">
            <tbody>
              <tr><th>输入 token</th><td class="num">{{ formatInteger(detail.usage.ai.promptTokens) }}</td></tr>
              <tr><th>输出 token</th><td class="num">{{ formatInteger(detail.usage.ai.completionTokens) }}</td></tr>
              <tr><th>有费率记录的调用</th><td class="num">{{ detail.usage.ai.costedCalls }} / {{ detail.usage.ai.calls }}</td></tr>
              <tr><th>估算费用</th><td class="num">{{ formatUSD(detail.usage.ai.estimatedCostUsd) }}</td></tr>
              <tr><th>最后登录 / 活动</th><td class="num">{{ formatDateTime(detail.usage.lastLoginAt) }} / {{ formatDateTime(detail.usage.lastActiveAt) }}</td></tr>
            </tbody>
          </table>
        </section>
      </div>

      <div class="grid-2">
        <section class="card">
          <h2 style="font-size: 17px">按 AI 用途</h2>
          <div v-if="detail.aiByKind.length === 0" class="muted">当前区间没有 AI 调用。</div>
          <div v-else style="overflow-x: auto">
            <table class="data">
              <thead><tr><th>用途</th><th class="num">调用</th><th class="num">失败</th><th class="num">token</th><th class="num">费用</th></tr></thead>
              <tbody>
                <tr v-for="row in detail.aiByKind" :key="row.key">
                  <td>{{ aiRunKindText[row.key] ?? row.key }}</td><td class="num">{{ row.calls }}</td><td class="num">{{ row.failedCalls }}</td><td class="num">{{ formatInteger(row.totalTokens) }}</td><td class="num">{{ breakdownCost(row) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
        <section class="card">
          <h2 style="font-size: 17px">按模型</h2>
          <div v-if="detail.aiByModel.length === 0" class="muted">当前区间没有 AI 调用。</div>
          <div v-else style="overflow-x: auto">
            <table class="data">
              <thead><tr><th>模型</th><th class="num">调用</th><th class="num">成功 / 失败</th><th class="num">token</th><th class="num">费用</th></tr></thead>
              <tbody>
                <tr v-for="row in detail.aiByModel" :key="row.key">
                  <td class="mono">{{ row.key || '未记录' }}</td><td class="num">{{ row.calls }}</td><td class="num">{{ row.successfulCalls }} / {{ row.failedCalls }}</td><td class="num">{{ formatInteger(row.totalTokens) }}</td><td class="num">{{ breakdownCost(row) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>

      <section class="card">
        <h2 style="font-size: 17px">每日 AI 使用</h2>
        <div v-if="detail.aiDaily.length === 0" class="muted">当前区间没有 AI 调用。</div>
        <div v-else style="overflow-x: auto">
          <table class="data">
            <thead><tr><th>日期</th><th class="num">调用</th><th class="num">失败</th><th class="num">输入 token</th><th class="num">输出 token</th><th class="num">耗时</th><th class="num">费用</th></tr></thead>
            <tbody>
              <tr v-for="row in detail.aiDaily" :key="row.date">
                <td class="mono">{{ row.date }}</td><td class="num">{{ row.calls }}</td><td class="num">{{ row.failedCalls }}</td><td class="num">{{ formatInteger(row.promptTokens) }}</td><td class="num">{{ formatInteger(row.completionTokens) }}</td><td class="num">{{ formatInteger(row.durationMs) }} ms</td><td class="num">{{ formatUSD(row.estimatedCostUsd) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="card">
        <h2 style="font-size: 17px">最近 AI 调用（最多 100 条）</h2>
        <div v-if="detail.recentAiRuns.length === 0" class="muted">当前区间没有 AI 调用。</div>
        <div v-else style="overflow-x: auto">
          <table class="data">
            <thead><tr><th>时间</th><th>用途</th><th>模型 / prompt</th><th>状态</th><th class="num">token</th><th class="num">耗时</th><th class="num">费用</th><th>错误</th></tr></thead>
            <tbody>
              <tr v-for="run in detail.recentAiRuns" :key="run.id">
                <td class="mono">{{ formatDateTime(run.createdAt) }}</td>
                <td>{{ aiRunKindText[run.kind] ?? run.kind }}</td>
                <td class="mono">{{ run.model || '未记录' }}<br>{{ run.promptVersion }}</td>
                <td><StatusBadge :value="run.status" /></td>
                <td class="num">{{ formatInteger(run.totalTokens) }}</td>
                <td class="num">{{ duration(run.durationMs) }}</td>
                <td class="num">{{ formatUSD(run.estimatedCostUsd) }}</td>
                <td style="max-width: 280px; overflow-wrap: anywhere">{{ run.error || '—' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="card">
        <h2 style="font-size: 17px">最近练习批次（最多 50 条）</h2>
        <div v-if="detail.recentPracticeSessions.length === 0" class="muted">当前区间没有练习批次。</div>
        <div v-else style="overflow-x: auto">
          <table class="data">
            <thead><tr><th>时间</th><th>批次 ID</th><th>类型</th><th>状态</th><th class="num">题数</th><th class="num">已作答</th><th>AI 分析</th><th>删除</th></tr></thead>
            <tbody>
              <tr v-for="session in detail.recentPracticeSessions" :key="session.id">
                <td class="mono">{{ formatDateTime(session.createdAt) }}</td>
                <td class="mono">{{ session.id.slice(0, 12) }}</td>
                <td>{{ practiceModeText[session.mode] ?? session.mode }}</td>
                <td><StatusBadge :value="session.status" kind="session" /></td>
                <td class="num">{{ session.totalCount }} / {{ session.requestedCount }}</td>
                <td class="num">{{ session.answeredCount }}</td>
                <td><StatusBadge :value="session.aiSummaryStatus" /></td>
                <td>{{ session.deletedAt ? '已隐藏' : '—' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>
  </AppShell>
</template>
