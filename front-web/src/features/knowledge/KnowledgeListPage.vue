<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { request, ApiError } from '@/api/client'
import type { Exam, KnowledgePointItem } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import AppStatus from '@/components/AppStatus.vue'
import { formatPercent, formatDateTime } from '@/app/format'

const exams = ref<Exam[]>([])
const items = ref<KnowledgePointItem[]>([])
const nextCursor = ref('')
const loadingMore = ref(false)
const levelId = ref('')
const subjectId = ref('')
const search = ref('')
const state = ref<'loading' | 'ready' | 'error'>('loading')
const errorMessage = ref('')
const requestID = ref('')

let timer: ReturnType<typeof setTimeout> | null = null

async function load(append = false): Promise<void> {
  if (append) loadingMore.value = true
  else state.value = 'loading'
  try {
    const params = new URLSearchParams({ limit: '20' })
    if (levelId.value) params.set('levelId', levelId.value)
    if (subjectId.value) params.set('subjectId', subjectId.value)
    if (search.value) params.set('q', search.value)
    if (append && nextCursor.value) params.set('cursor', nextCursor.value)
    const res = await request<{ knowledgePoints: KnowledgePointItem[]; nextCursor?: string }>(`/knowledge-points?${params}`)
    items.value = append ? [...items.value, ...res.knowledgePoints] : res.knowledgePoints
    nextCursor.value = res.nextCursor ?? ''
    state.value = 'ready'
  } catch (err) {
    errorMessage.value = err instanceof ApiError ? err.message : '加载失败'
    requestID.value = err instanceof ApiError ? err.requestId ?? '' : ''
    state.value = 'error'
  } finally {
    loadingMore.value = false
  }
}

onMounted(async () => {
  try {
    const res = await request<{ exams: Exam[] }>('/catalog')
    exams.value = res.exams
  } catch {
    // 分类筛选加载失败不阻塞列表
  }
  await load()
})

watch([levelId, subjectId], () => void load())
watch(search, () => {
  if (timer) clearTimeout(timer)
  timer = setTimeout(() => void load(), 300)
})

function mastery(k: KnowledgePointItem): string {
  const stats = k.stats
  if (!stats || stats.confirmedAnswered === 0) return '未练习'
  if (stats.consecutiveWrong >= 3) return '连续出错'
  const acc = stats.confirmedCorrect / stats.confirmedAnswered
  if (acc >= 0.8) return '掌握较好'
  return '需要巩固'
}
</script>

<template>
  <AppShell>
    <h1 style="font-size: 24px">知识点</h1>
    <div class="grid-3">
      <div class="field">
        <label for="level">级别</label>
        <select id="level" v-model="levelId">
          <option value="">全部</option>
          <option v-for="l in exams.flatMap((e) => e.levels)" :key="l.id" :value="l.id">{{ l.name }}</option>
        </select>
      </div>
      <div class="field">
        <label for="subject">科目</label>
        <select id="subject" v-model="subjectId">
          <option value="">全部</option>
          <option v-for="s in exams.flatMap((e) => e.subjects)" :key="s.id" :value="s.id">{{ s.name }}</option>
        </select>
      </div>
      <div class="field">
        <label for="search">搜索</label>
        <input id="search" v-model="search" type="text" placeholder="知识点名称" />
      </div>
    </div>

    <AppStatus v-if="state === 'loading'" state="loading" />
    <AppStatus v-else-if="state === 'error'" state="error" :message="errorMessage" :request-id="requestID" @action="load" />
    <AppStatus v-else-if="items.length === 0" state="empty" message="没有符合条件且已发布的知识点。" />
    <div v-else class="card" style="overflow-x: auto">
      <table class="data">
        <thead>
          <tr>
            <th>知识点</th>
            <th>级别</th>
            <th>科目</th>
            <th class="num">相关题目</th>
            <th class="num">已确认正确率</th>
            <th>掌握状态</th>
            <th>最近练习</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="k in items" :key="k.id">
            <td><RouterLink :to="`/knowledge/${k.id}`">{{ k.name }}</RouterLink></td>
            <td>{{ k.levelCode }}</td>
            <td>{{ k.subjectName }}</td>
            <td class="num">{{ k.questionCount }}</td>
            <td class="num">
              {{ k.stats && k.stats.confirmedAnswered > 0
                ? formatPercent(k.stats.confirmedCorrect / k.stats.confirmedAnswered) : '—' }}
            </td>
            <td>{{ mastery(k) }}</td>
            <td class="mono">{{ k.stats?.lastPracticedAt ? formatDateTime(k.stats.lastPracticedAt) : '—' }}</td>
            <td><RouterLink :to="`/knowledge/${k.id}`">详情</RouterLink></td>
          </tr>
        </tbody>
      </table>
      <p v-if="nextCursor" style="margin: 12px 0 0; text-align: center">
        <button :disabled="loadingMore" @click="load(true)">{{ loadingMore ? '加载中…' : '加载更多' }}</button>
      </p>
    </div>
  </AppShell>
</template>
