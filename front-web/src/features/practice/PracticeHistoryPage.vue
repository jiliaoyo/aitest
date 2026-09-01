<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { request, ApiError } from '@/api/client'
import type { SessionListItem } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import AppStatus from '@/components/AppStatus.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { formatDateTime } from '@/app/format'

const route = useRoute()
const router = useRouter()

const sessions = ref<SessionListItem[]>([])
const nextCursor = ref('')
const state = ref<'loading' | 'ready' | 'error'>('loading')
const errorMessage = ref('')
const requestID = ref('')
const statusFilter = ref((route.query.status as string | undefined) ?? '')

const statusOptions = [
  { value: '', label: '全部' },
  { value: 'active', label: '答题中' },
  { value: 'grading', label: '判分中' },
  { value: 'completed', label: '已完成' },
  { value: 'analysis_failed', label: '部分分析失败' },
]

async function load(append = false): Promise<void> {
  if (!append) state.value = 'loading'
  try {
    const params = new URLSearchParams({ limit: '20' })
    if (statusFilter.value) params.set('status', statusFilter.value)
    if (append && nextCursor.value) params.set('cursor', nextCursor.value)
    const res = await request<{ sessions: SessionListItem[]; nextCursor: string }>(`/practice-sessions?${params}`)
    sessions.value = append ? [...sessions.value, ...res.sessions] : res.sessions
    nextCursor.value = res.nextCursor
    state.value = 'ready'
  } catch (err) {
    errorMessage.value = err instanceof ApiError ? err.message : '加载失败'
    requestID.value = err instanceof ApiError ? err.requestId ?? '' : ''
    state.value = 'error'
  }
}

onMounted(() => load())
watch(statusFilter, () => {
  router.replace({ query: { ...route.query, status: statusFilter.value || undefined } })
  void load()
})

function linkFor(s: SessionListItem): string {
  return s.status === 'active' ? `/practice/${s.id}` : `/practice/${s.id}/result`
}
</script>

<template>
  <AppShell>
    <div class="page-header">
      <h1 style="font-size: 24px; margin: 0">练习历史</h1>
      <label style="display: flex; align-items: center; gap: 8px">
        <span class="muted">状态</span>
        <select v-model="statusFilter" style="min-width: 140px">
          <option v-for="o in statusOptions" :key="o.value" :value="o.value">{{ o.label }}</option>
        </select>
      </label>
    </div>

    <AppStatus v-if="state === 'loading'" state="loading" />
    <AppStatus v-else-if="state === 'error'" state="error" :message="errorMessage" :request-id="requestID" @action="load()" />
    <AppStatus v-else-if="sessions.length === 0" state="empty" message="还没有练习记录。" action-label="创建练习" @action="router.push('/practice/new')" />
    <div v-else class="card" style="overflow-x: auto">
      <table class="data">
        <thead>
          <tr>
            <th>批次</th>
            <th>状态</th>
            <th class="num">题数</th>
            <th>创建时间</th>
            <th>提交时间</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="s in sessions" :key="s.id">
            <td class="mono">{{ s.id.slice(0, 8) }}</td>
            <td><StatusBadge :value="s.status" kind="session" /></td>
            <td class="num">{{ s.totalCount }}</td>
            <td class="mono">{{ formatDateTime(s.createdAt) }}</td>
            <td class="mono">{{ formatDateTime(s.submittedAt) }}</td>
            <td>
              <RouterLink :to="linkFor(s)">{{ s.status === 'active' ? '继续答题' : '查看结果' }}</RouterLink>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="nextCursor" style="margin-top: 12px; text-align: center">
        <button @click="load(true)">加载更多</button>
      </p>
    </div>
  </AppShell>
</template>
