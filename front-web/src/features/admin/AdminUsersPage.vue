<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { request, ApiError } from '@/api/client'
import type { AdminAIUsageBreakdown, AdminUsersResponse } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import AppStatus from '@/components/AppStatus.vue'
import { aiRunKindText, formatDateTime, formatInteger, formatUSD, roleText } from '@/app/format'

const route = useRoute()
const router = useRouter()
const response = ref<AdminUsersResponse | null>(null)
const state = ref<'loading' | 'ready' | 'error'>('loading')
const loadingMore = ref(false)
const errorMessage = ref('')
const requestID = ref('')
const nextCursor = ref('')

const filters = reactive({
  q: (route.query.q as string | undefined) ?? '',
  role: (route.query.role as string | undefined) ?? '',
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

let timer: ReturnType<typeof setTimeout> | null = null

async function load(append = false): Promise<void> {
  if (append) loadingMore.value = true
  else state.value = 'loading'
  errorMessage.value = ''
  try {
    const params = new URLSearchParams({ limit: '20' })
    for (const [key, value] of Object.entries(filters)) {
      if (value) params.set(key, value)
    }
    if (append && nextCursor.value) params.set('cursor', nextCursor.value)
    const data = await request<AdminUsersResponse>(`/admin/users?${params.toString()}`)
    response.value = append && response.value
      ? { ...data, users: [...response.value.users, ...data.users] }
      : data
    nextCursor.value = data.nextCursor
    state.value = 'ready'
  } catch (err) {
    errorMessage.value = err instanceof ApiError ? err.message : '加载失败'
    requestID.value = err instanceof ApiError ? err.requestId ?? '' : ''
    state.value = 'error'
  } finally {
    loadingMore.value = false
  }
}

function breakdownCost(row: AdminAIUsageBreakdown): string {
  return formatUSD(row.estimatedCostUsd)
}

function averageDuration(calls: number, durationMs: number): string {
  if (!calls) return '—'
  return `${Math.round(durationMs / calls)} ms/次`
}

watch(filters, () => {
  const query = Object.fromEntries(Object.entries(filters).filter(([, value]) => value))
  void router.replace({ query })
  if (timer) clearTimeout(timer)
  timer = setTimeout(() => void load(), 250)
}, { deep: true })

onMounted(() => void load())
</script>

<template>
  <AppShell>
    <div class="page-header">
      <div>
        <h1 style="font-size: 24px; margin: 0">用户与用量</h1>
        <p class="muted" style="margin: 4px 0 0">AI 调用按实际模型请求计数；失败和重试也会保留，费用是调用时配置价格下的美元估算。</p>
      </div>
    </div>

    <div class="card">
      <div class="grid-3">
        <div class="field">
          <label for="user-q">搜索邮箱</label>
          <input id="user-q" v-model="filters.q" type="text" placeholder="输入邮箱关键词" />
        </div>
        <div class="field">
          <label for="user-role">角色</label>
          <select id="user-role" v-model="filters.role">
            <option value="">全部角色</option>
            <option value="learner">学习者</option>
            <option value="admin">管理员</option>
          </select>
        </div>
        <div class="field">
          <label>统计区间</label>
          <div style="display: flex; gap: 8px; flex-wrap: wrap">
            <button type="button" @click="setRange(7)">近 7 天</button>
            <button type="button" @click="setRange(30)">近 30 天</button>
            <button type="button" @click="setRange(90)">近 90 天</button>
            <button type="button" @click="setRange(null)">全部</button>
          </div>
        </div>
      </div>
      <div class="grid-2" style="margin-top: 4px">
        <div class="field">
          <label for="user-from">开始日期</label>
          <input id="user-from" v-model="filters.from" type="date" />
        </div>
        <div class="field">
          <label for="user-to">结束日期</label>
          <input id="user-to" v-model="filters.to" type="date" />
        </div>
      </div>
    </div>

    <AppStatus v-if="state === 'loading'" state="loading" />
    <AppStatus v-else-if="state === 'error'" state="error" :message="errorMessage" :request-id="requestID" @action="load" />
    <template v-else-if="response">
      <div class="metrics">
        <div class="metric">
          <p class="value">{{ formatInteger(response.summary.totalUsers) }}</p>
          <p class="label">用户总数（学习者 {{ response.summary.learnerUsers }} / 管理员 {{ response.summary.adminUsers }}）</p>
        </div>
        <div class="metric">
          <p class="value">{{ formatInteger(response.summary.newUsers) }}</p>
          <p class="label">区间新增用户</p>
        </div>
        <div class="metric">
          <p class="value">{{ formatInteger(response.summary.activeUsers) }}</p>
          <p class="label">区间活跃用户</p>
        </div>
      </div>

      <div class="metrics">
        <div class="metric">
          <p class="value">{{ formatInteger(response.summary.usage.practiceSessions) }}</p>
          <p class="label">练习批次（已提交 {{ response.summary.usage.submittedSessions }}）</p>
        </div>
        <div class="metric">
          <p class="value">{{ formatInteger(response.summary.usage.ai.calls) }}</p>
          <p class="label">AI 实际调用（失败 {{ response.summary.usage.ai.failedCalls }}）</p>
        </div>
        <div class="metric">
          <p class="value">{{ formatInteger(response.summary.usage.ai.totalTokens) }}</p>
          <p class="label">AI 总 token · {{ formatUSD(response.summary.usage.ai.estimatedCostUsd) }}</p>
        </div>
      </div>

      <div class="grid-2">
        <section class="card">
          <h2 style="font-size: 17px">练习使用情况</h2>
          <table class="data">
            <tbody>
              <tr><th>练习批次</th><td class="num">{{ formatInteger(response.summary.usage.practiceSessions) }}</td></tr>
              <tr><th>已完成 / 分析失败</th><td class="num">{{ response.summary.usage.completedSessions }} / {{ response.summary.usage.analysisFailedSessions }}</td></tr>
              <tr><th>进行中 / 生成失败</th><td class="num">{{ response.summary.usage.activeSessions }} / {{ response.summary.usage.generationFailedSessions }}</td></tr>
              <tr><th>题目数 / 已作答</th><td class="num">{{ formatInteger(response.summary.usage.practiceItems) }} / {{ formatInteger(response.summary.usage.answeredItems) }}</td></tr>
              <tr><th>活跃天数</th><td class="num">{{ response.summary.usage.activeDays }}</td></tr>
            </tbody>
          </table>
        </section>
        <section class="card">
          <h2 style="font-size: 17px">AI 出题与费用</h2>
          <table class="data">
            <tbody>
              <tr><th>出题请求批次</th><td class="num">{{ formatInteger(response.summary.usage.aiGenerationRequests) }}</td></tr>
              <tr><th>实际出题调用</th><td class="num">{{ formatInteger(response.summary.usage.ai.generationCalls) }}</td></tr>
              <tr><th>生成题目数</th><td class="num">{{ formatInteger(response.summary.usage.aiGeneratedQuestions) }}</td></tr>
              <tr><th>输入 / 输出 token</th><td class="num">{{ formatInteger(response.summary.usage.ai.promptTokens) }} / {{ formatInteger(response.summary.usage.ai.completionTokens) }}</td></tr>
              <tr><th>总耗时 / 平均</th><td class="num">{{ formatInteger(response.summary.usage.ai.durationMs) }} ms · {{ averageDuration(response.summary.usage.ai.calls, response.summary.usage.ai.durationMs) }}</td></tr>
            </tbody>
          </table>
        </section>
      </div>

      <div class="grid-2">
        <section class="card">
          <h2 style="font-size: 17px">按 AI 用途</h2>
          <AppStatus v-if="response.summary.aiByKind.length === 0" state="empty" message="当前区间没有 AI 调用。" />
          <div v-else style="overflow-x: auto">
            <table class="data">
              <thead><tr><th>用途</th><th class="num">调用</th><th class="num">失败</th><th class="num">token</th><th class="num">费用</th></tr></thead>
              <tbody>
                <tr v-for="row in response.summary.aiByKind" :key="row.key">
                  <td>{{ aiRunKindText[row.key] ?? row.key }}</td>
                  <td class="num">{{ row.calls }}</td>
                  <td class="num">{{ row.failedCalls }}</td>
                  <td class="num">{{ formatInteger(row.totalTokens) }}</td>
                  <td class="num">{{ breakdownCost(row) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
        <section class="card">
          <h2 style="font-size: 17px">按模型</h2>
          <AppStatus v-if="response.summary.aiByModel.length === 0" state="empty" message="当前区间没有 AI 调用。" />
          <div v-else style="overflow-x: auto">
            <table class="data">
              <thead><tr><th>模型</th><th class="num">调用</th><th class="num">成功 / 失败</th><th class="num">token</th><th class="num">费用</th></tr></thead>
              <tbody>
                <tr v-for="row in response.summary.aiByModel" :key="row.key">
                  <td class="mono">{{ row.key || '未记录' }}</td>
                  <td class="num">{{ row.calls }}</td>
                  <td class="num">{{ row.successfulCalls }} / {{ row.failedCalls }}</td>
                  <td class="num">{{ formatInteger(row.totalTokens) }}</td>
                  <td class="num">{{ breakdownCost(row) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>

      <section class="card">
        <h2 style="font-size: 17px">AI 调用趋势</h2>
        <div v-if="response.summary.aiDaily.length === 0" class="muted">当前区间没有 AI 调用。</div>
        <div v-else style="overflow-x: auto">
          <table class="data">
            <thead><tr><th>日期</th><th class="num">调用</th><th class="num">失败</th><th class="num">输入 token</th><th class="num">输出 token</th><th class="num">费用</th></tr></thead>
            <tbody>
              <tr v-for="row in response.summary.aiDaily" :key="row.date">
                <td class="mono">{{ row.date }}</td>
                <td class="num">{{ row.calls }}</td>
                <td class="num">{{ row.failedCalls }}</td>
                <td class="num">{{ formatInteger(row.promptTokens) }}</td>
                <td class="num">{{ formatInteger(row.completionTokens) }}</td>
                <td class="num">{{ formatUSD(row.estimatedCostUsd) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <AppStatus v-if="response.users.length === 0" state="empty" message="没有符合条件的用户。" />
      <section v-else class="card" style="overflow-x: auto">
        <h2 style="font-size: 17px">用户明细</h2>
        <table class="data">
          <thead>
            <tr>
              <th>邮箱</th><th>角色</th><th>注册时间</th><th>最近活跃</th>
              <th class="num">练习批次</th><th class="num">AI 出题请求</th><th class="num">AI 调用</th><th class="num">token</th><th class="num">费用</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="user in response.users" :key="user.id">
              <td><RouterLink :to="{ path: `/admin/users/${user.id}`, query: { from: filters.from || undefined, to: filters.to || undefined } }">{{ user.email }}</RouterLink></td>
              <td><span class="tag" :data-tone="user.role === 'admin' ? 'accent' : undefined">{{ roleText[user.role] ?? user.role }}</span></td>
              <td class="mono">{{ formatDateTime(user.createdAt) }}</td>
              <td class="mono">{{ formatDateTime(user.usage.lastActiveAt) }}</td>
              <td class="num">{{ user.usage.practiceSessions }}</td>
              <td class="num">{{ user.usage.aiGenerationRequests }}</td>
              <td class="num">{{ user.usage.ai.calls }}</td>
              <td class="num">{{ formatInteger(user.usage.ai.totalTokens) }}</td>
              <td class="num">{{ formatUSD(user.usage.ai.estimatedCostUsd) }}</td>
            </tr>
          </tbody>
        </table>
        <p v-if="nextCursor" style="margin: 14px 0 0; text-align: center">
          <button :disabled="loadingMore" @click="load(true)">{{ loadingMore ? '加载中…' : '加载更多' }}</button>
        </p>
      </section>
    </template>
  </AppShell>
</template>
