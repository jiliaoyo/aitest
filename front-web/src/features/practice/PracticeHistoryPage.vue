<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { request, ApiError } from '@/api/client'
import type { SessionListItem } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import AppStatus from '@/components/AppStatus.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { formatDateTime, practiceModeText } from '@/app/format'

const route = useRoute()
const router = useRouter()

const sessions = ref<SessionListItem[]>([])
const nextCursor = ref('')
const state = ref<'loading' | 'ready' | 'error'>('loading')
const errorMessage = ref('')
const requestID = ref('')
const statusFilter = ref((route.query.status as string | undefined) ?? '')
const modeFilter = ref((route.query.mode as string | undefined) ?? '')
const deleteError = ref('')
const deletingID = ref('')

const statusOptions = [
  { value: '', label: '全部' },
  { value: 'generating', label: 'AI 出题中' },
  { value: 'active', label: '答题中' },
  { value: 'grading', label: '判分中' },
  { value: 'completed', label: '已完成' },
  { value: 'analysis_failed', label: '部分分析失败' },
  { value: 'generation_failed', label: 'AI 出题失败' },
]
const modeOptions = [
  { value: '', label: '全部分类' },
  { value: 'comprehensive', label: practiceModeText.comprehensive },
  { value: 'knowledge', label: practiceModeText.knowledge },
  { value: 'wrong_items', label: practiceModeText.wrong_items },
  { value: 'review', label: practiceModeText.review },
  { value: 'ai_generated', label: practiceModeText.ai_generated },
]

type HistoryGroupKey = 'today' | 'yesterday' | 'week' | 'earlier'
type HistoryGroup = { key: HistoryGroupKey; label: string; items: SessionListItem[] }

const groupMeta: Array<{ key: HistoryGroupKey; label: string }> = [
  { key: 'today', label: '今天' },
  { key: 'yesterday', label: '昨天' },
  { key: 'week', label: '近一周' },
  { key: 'earlier', label: '一周前及更早' },
]

function localDay(value: Date): number {
  return Date.UTC(value.getFullYear(), value.getMonth(), value.getDate())
}

function groupKey(createdAt: string): HistoryGroupKey {
  const created = new Date(createdAt)
  if (Number.isNaN(created.getTime())) return 'earlier'
  const distance = Math.floor((localDay(new Date()) - localDay(created)) / 86400000)
  if (distance <= 0) return 'today'
  if (distance === 1) return 'yesterday'
  if (distance < 7) return 'week'
  return 'earlier'
}

const groupedSessions = computed<HistoryGroup[]>(() => {
  const groups = new Map<HistoryGroupKey, SessionListItem[]>()
  for (const session of sessions.value) {
    const key = groupKey(session.createdAt)
    const items = groups.get(key) ?? []
    items.push(session)
    groups.set(key, items)
  }
  return groupMeta.filter((group) => groups.has(group.key)).map((group) => ({
    ...group,
    items: groups.get(group.key) ?? [],
  }))
})

async function load(append = false): Promise<void> {
  if (!append) state.value = 'loading'
  try {
    const params = new URLSearchParams({ limit: '20' })
    if (statusFilter.value) params.set('status', statusFilter.value)
    if (modeFilter.value) params.set('mode', modeFilter.value)
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
watch([statusFilter, modeFilter], () => {
  router.replace({ query: { ...route.query, status: statusFilter.value || undefined, mode: modeFilter.value || undefined } })
  void load()
})

function linkFor(s: SessionListItem): string {
  return ['active', 'generating', 'generation_failed'].includes(s.status) ? `/practice/${s.id}` : `/practice/${s.id}/result`
}

async function deleteSession(session: SessionListItem): Promise<void> {
  if (deletingID.value || session.status === 'generating') return
  if (!window.confirm('确定隐藏这条练习历史吗？原始答题记录和成绩会保留。')) return
  deletingID.value = session.id
  deleteError.value = ''
  try {
    await request(`/practice-sessions/${session.id}`, { method: 'DELETE' })
    sessions.value = sessions.value.filter((item) => item.id !== session.id)
  } catch (err) {
    deleteError.value = err instanceof ApiError ? err.message : '删除失败，请重试'
  } finally {
    deletingID.value = ''
  }
}
</script>

<template>
  <AppShell>
    <div class="page-header">
      <h1 style="font-size: 24px; margin: 0">练习历史</h1>
      <div style="display: flex; align-items: center; gap: 10px; flex-wrap: wrap">
        <label style="display: flex; align-items: center; gap: 8px">
          <span class="muted">状态</span>
          <select v-model="statusFilter" style="min-width: 140px">
            <option v-for="o in statusOptions" :key="o.value" :value="o.value">{{ o.label }}</option>
          </select>
        </label>
        <label style="display: flex; align-items: center; gap: 8px">
          <span class="muted">分类</span>
          <select v-model="modeFilter" style="min-width: 140px">
            <option v-for="o in modeOptions" :key="o.value" :value="o.value">{{ o.label }}</option>
          </select>
        </label>
      </div>
    </div>
    <p v-if="deleteError" class="error-summary" role="alert">{{ deleteError }}</p>

    <AppStatus v-if="state === 'loading'" state="loading" />
    <AppStatus v-else-if="state === 'error'" state="error" :message="errorMessage" :request-id="requestID" @action="load()" />
    <AppStatus v-else-if="sessions.length === 0" state="empty" message="还没有练习记录。" action-label="创建练习" @action="router.push('/practice/new')" />
    <div v-else class="history-groups" style="display: flex; flex-direction: column; gap: 14px">
      <section v-for="group in groupedSessions" :key="group.key" class="card history-group">
        <details :open="group.key === 'today'">
          <summary style="display: flex; align-items: center; justify-content: space-between; min-height: 44px; cursor: pointer"><strong>{{ group.label }}</strong><span class="muted">{{ group.items.length }} 批</span></summary>
          <div style="overflow-x: auto; margin-top: 12px">
            <table class="data mobile-card-table">
              <thead>
                <tr>
                  <th>批次</th>
                  <th>分类</th>
                  <th>状态</th>
                  <th class="num">题数</th>
                  <th>创建时间</th>
                  <th>提交时间</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="s in group.items" :key="s.id">
                  <td class="mono" data-label="批次">{{ s.id.slice(0, 8) }}</td>
                  <td data-label="分类">{{ practiceModeText[s.mode] ?? '综合练习' }}</td>
                  <td data-label="状态"><StatusBadge :value="s.status" kind="session" /></td>
                  <td class="num" data-label="题数">{{ s.totalCount }}</td>
                  <td class="mono" data-label="创建时间">{{ formatDateTime(s.createdAt) }}</td>
                  <td class="mono" data-label="提交时间">{{ formatDateTime(s.submittedAt) }}</td>
                  <td data-label="">
                    <div style="display: flex; align-items: center; gap: 12px; flex-wrap: wrap">
                      <RouterLink :to="linkFor(s)">{{ ['active', 'generating'].includes(s.status) ? '继续练习' : s.status === 'generation_failed' ? '查看生成状态' : '查看结果' }}</RouterLink>
                      <button
                        v-if="s.status !== 'generating'"
                        class="ghost danger"
                        type="button"
                        :disabled="deletingID === s.id"
                        @click="deleteSession(s)"
                      >
                        {{ deletingID === s.id ? '删除中…' : '删除' }}
                      </button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </details>
      </section>
      <p v-if="nextCursor" style="margin: 0; text-align: center">
        <button @click="load(true)">加载更多</button>
      </p>
    </div>
  </AppShell>
</template>
