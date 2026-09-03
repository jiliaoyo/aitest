<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { request, ApiError } from '@/api/client'
import type { Exam, QuestionAdminDTO } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import AppStatus from '@/components/AppStatus.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { formatDateTime, questionTypeText } from '@/app/format'

const route = useRoute()
const router = useRouter()

const questions = ref<QuestionAdminDTO[]>([])
const nextCursor = ref('')
const exams = ref<Exam[]>([])
const state = ref<'loading' | 'ready' | 'error'>('loading')
const errorMessage = ref('')
const requestID = ref('')

const filters = ref({
  status: (route.query.status as string | undefined) ?? '',
  levelId: (route.query.levelId as string | undefined) ?? '',
  subjectId: (route.query.subjectId as string | undefined) ?? '',
  q: (route.query.q as string | undefined) ?? '',
  hasAnswer: (route.query.hasAnswer as string | undefined) ?? '',
  quality: (route.query.quality as string | undefined) ?? '',
})

const statusOptions = [
  { value: '', label: '全部状态' },
  { value: 'draft', label: '草稿' },
  { value: 'in_review', label: '待审核' },
  { value: 'published', label: '已发布' },
  { value: 'retired', label: '已下架' },
]
const qualityOptions = [
  { value: '', label: '全部质量' },
  { value: 'no_knowledge', label: '无知识点' },
  { value: 'no_source', label: '无来源' },
  { value: 'no_answer', label: '无答案' },
]

let timer: ReturnType<typeof setTimeout> | null = null

async function load(append = false): Promise<void> {
  if (!append) state.value = 'loading'
  try {
    const params = new URLSearchParams({ limit: '20' })
    for (const [k, v] of Object.entries(filters.value)) {
      if (v) params.set(k, v)
    }
    if (append && nextCursor.value) params.set('cursor', nextCursor.value)
    const res = await request<{ questions: QuestionAdminDTO[]; nextCursor: string }>(`/admin/questions?${params}`)
    questions.value = append ? [...questions.value, ...res.questions] : res.questions
    nextCursor.value = res.nextCursor
    state.value = 'ready'
  } catch (err) {
    errorMessage.value = err instanceof ApiError ? err.message : '加载失败'
    requestID.value = err instanceof ApiError ? err.requestId ?? '' : ''
    state.value = 'error'
  }
}

onMounted(async () => {
  try {
    const res = await request<{ exams: Exam[] }>('/catalog')
    exams.value = res.exams
  } catch {
    // 筛选下拉为空不阻塞列表
  }
  await load()
})

watch(filters, () => {
  router.replace({ query: { ...route.query, ...Object.fromEntries(Object.entries(filters.value).filter(([, v]) => v)) } })
  if (timer) clearTimeout(timer)
  timer = setTimeout(() => void load(), 250)
}, { deep: true })
</script>

<template>
  <AppShell>
    <div class="page-header">
      <h1 style="font-size: 24px; margin: 0">题目</h1>
      <RouterLink to="/admin/questions/new"><button class="primary">新建题目</button></RouterLink>
    </div>

    <div class="grid-3">
      <div class="field">
        <label for="f-status">状态</label>
        <select id="f-status" v-model="filters.status">
          <option v-for="o in statusOptions" :key="o.value" :value="o.value">{{ o.label }}</option>
        </select>
      </div>
      <div class="field">
        <label for="f-level">级别</label>
        <select id="f-level" v-model="filters.levelId">
          <option value="">全部</option>
          <option v-for="l in exams.flatMap((e) => e.levels)" :key="l.id" :value="l.id">{{ l.name }}</option>
        </select>
      </div>
      <div class="field">
        <label for="f-q">搜索题干</label>
        <input id="f-q" v-model="filters.q" type="text" placeholder="输入关键词" />
      </div>
      <div class="field">
        <label for="f-quality">质量入口</label>
        <select id="f-quality" v-model="filters.quality">
          <option v-for="o in qualityOptions" :key="o.value" :value="o.value">{{ o.label }}</option>
        </select>
      </div>
    </div>

    <AppStatus v-if="state === 'loading'" state="loading" />
    <AppStatus v-else-if="state === 'error'" state="error" :message="errorMessage" :request-id="requestID" @action="load()" />
    <AppStatus v-else-if="questions.length === 0" state="empty" message="没有符合条件的题目。" action-label="新建题目" @action="router.push('/admin/questions/new')" />
    <div v-else class="card" style="overflow-x: auto">
      <table class="data">
        <thead>
          <tr>
            <th>题干</th>
            <th>题型</th>
            <th>状态</th>
            <th class="num">版本</th>
            <th>有答案</th>
            <th>更新时间</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="q in questions" :key="q.id">
            <td style="max-width: 320px">
              <RouterLink :to="`/admin/questions/${q.id}`" class="mono" lang="ja">
                {{ q.currentVersion?.stem.slice(0, 40) ?? '' }}{{ (q.currentVersion?.stem.length ?? 0) > 40 ? '…' : '' }}
              </RouterLink>
            </td>
            <td>{{ questionTypeText[q.currentVersion?.type ?? ''] ?? '—' }}</td>
            <td><StatusBadge :value="q.status" /></td>
            <td class="num">v{{ q.currentVersion?.versionNo ?? '-' }}</td>
            <td>{{ q.hasAnswer ? '✓' : '—' }}</td>
            <td class="mono">{{ formatDateTime(q.updatedAt) }}</td>
          </tr>
        </tbody>
      </table>
      <p v-if="nextCursor" style="margin-top: 12px; text-align: center">
        <button @click="load(true)">加载更多</button>
      </p>
    </div>
  </AppShell>
</template>
