<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { request, ApiError, fieldErrors } from '@/api/client'
import type { AdminKnowledgePoint, Exam } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import AppStatus from '@/components/AppStatus.vue'
import StatusBadge from '@/components/StatusBadge.vue'

const kps = ref<AdminKnowledgePoint[]>([])
const nextCursor = ref('')
const loadingMore = ref(false)
const exams = ref<Exam[]>([])
const levelFilter = ref('')
const state = ref<'loading' | 'ready' | 'error'>('loading')
const errorMessage = ref('')
const requestID = ref('')

const form = reactive({
  name: '',
  levelId: '',
  subjectId: '',
  parentId: '',
  description: '',
  commonMistakes: '',
  examples: '',
})
const fieldErr = reactive<Record<string, string>>({})
const topError = ref('')
const saving = ref(false)

const subjects = ref<Exam['subjects']>([])

async function load(append = false): Promise<void> {
  if (append) loadingMore.value = true
  else state.value = 'loading'
  try {
    const params = new URLSearchParams({ limit: '20' })
    if (levelFilter.value) params.set('levelId', levelFilter.value)
    if (append && nextCursor.value) params.set('cursor', nextCursor.value)
    const res = await request<{ knowledgePoints: AdminKnowledgePoint[]; nextCursor?: string }>(
      `/admin/knowledge-points?${params}`,
    )
    kps.value = append ? [...kps.value, ...res.knowledgePoints] : res.knowledgePoints
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
    // 忽略
  }
  await load()
})

function onLevelChange(): void {
  const level = exams.value.flatMap((e) => e.levels).find((l) => l.id === form.levelId)
  const exam = exams.value.find((e) => e.levels.some((l) => l.id === form.levelId))
  form.subjectId = ''
  subjects.value = exam?.subjects ?? []
  void level
}

async function create(): Promise<void> {
  saving.value = true
  topError.value = ''
  for (const k of Object.keys(fieldErr)) delete fieldErr[k]
  try {
    await request('/admin/knowledge-points', {
      method: 'POST',
      body: {
        name: form.name,
        levelId: form.levelId,
        subjectId: form.subjectId,
        parentId: form.parentId || null,
        description: form.description,
        commonMistakes: form.commonMistakes,
        examples: form.examples,
      },
    })
    form.name = ''
    form.parentId = ''
    form.description = ''
    form.commonMistakes = ''
    form.examples = ''
    await load()
  } catch (err) {
    const fields = fieldErrors(err)
    for (const [k, v] of Object.entries(fields)) fieldErr[k] = v
    topError.value = Object.keys(fields).length ? '请检查表单' : err instanceof ApiError ? err.message : '创建失败'
  } finally {
    saving.value = false
  }
}

async function publish(k: AdminKnowledgePoint): Promise<void> {
  try {
    await request(`/admin/knowledge-points/${k.id}`, {
      method: 'PATCH',
      body: { status: 'published' },
    })
    await load()
  } catch (err) {
    errorMessage.value = err instanceof ApiError ? err.message : '操作失败'
  }
}

async function unpublish(k: AdminKnowledgePoint): Promise<void> {
  try {
    await request(`/admin/knowledge-points/${k.id}`, {
      method: 'PATCH',
      body: { status: 'draft' },
    })
    await load()
  } catch (err) {
    errorMessage.value = err instanceof ApiError ? err.message : '操作失败'
  }
}
</script>

<template>
  <AppShell>
    <h1 style="font-size: 24px">知识点管理</h1>

    <form class="card" @submit.prevent="create">
      <h2 style="font-size: 16px">新建知识点</h2>
      <p v-if="topError" class="error-summary" role="alert">{{ topError }}</p>
      <div class="grid-2">
        <div class="field">
          <label for="k-level">级别</label>
          <select id="k-level" v-model="form.levelId" required @change="onLevelChange">
            <option v-for="l in exams.flatMap((e) => e.levels)" :key="l.id" :value="l.id">{{ l.name }}</option>
          </select>
          <p v-if="fieldErr.levelId" class="error">{{ fieldErr.levelId }}</p>
        </div>
        <div class="field">
          <label for="k-subject">科目</label>
          <select id="k-subject" v-model="form.subjectId" required>
            <option v-for="s in subjects" :key="s.id" :value="s.id">{{ s.name }}</option>
          </select>
          <p v-if="fieldErr.subjectId" class="error">{{ fieldErr.subjectId }}</p>
        </div>
      </div>
      <div class="grid-2">
        <div class="field">
          <label for="k-name">名称</label>
          <input id="k-name" v-model="form.name" type="text" />
          <p v-if="fieldErr.name" class="error">{{ fieldErr.name }}</p>
        </div>
        <div class="field">
          <label for="k-parent">父知识点（可选，需同级别同科目）</label>
          <select id="k-parent" v-model="form.parentId">
            <option value="">无</option>
            <option v-for="k in kps.filter((x) => x.levelId === form.levelId && x.subjectId === form.subjectId)" :key="k.id" :value="k.id">
              {{ k.name }}
            </option>
          </select>
        </div>
      </div>
      <div class="field">
        <label for="k-desc">说明</label>
        <textarea id="k-desc" v-model="form.description" rows="3" />
      </div>
      <div class="grid-2">
        <div class="field">
          <label for="k-mistakes">常见误区</label>
          <textarea id="k-mistakes" v-model="form.commonMistakes" rows="2" />
        </div>
        <div class="field">
          <label for="k-examples">例句</label>
          <textarea id="k-examples" v-model="form.examples" rows="2" lang="ja" />
        </div>
      </div>
      <button class="primary" type="submit" :disabled="saving">{{ saving ? '创建中…' : '创建（草稿）' }}</button>
      <p class="muted" style="font-size: 13px; margin-top: 8px">创建后为草稿；发布后学习者可见。AI 生成草稿需人工审核后才可发布。</p>
    </form>

    <div class="field" style="max-width: 260px">
      <label for="k-filter">按级别筛选</label>
      <select id="k-filter" v-model="levelFilter" @change="() => load()">
        <option value="">全部</option>
        <option v-for="l in exams.flatMap((e) => e.levels)" :key="l.id" :value="l.id">{{ l.name }}</option>
      </select>
    </div>

    <AppStatus v-if="state === 'loading'" state="loading" />
    <AppStatus v-else-if="state === 'error'" state="error" :message="errorMessage" :request-id="requestID" @action="load" />
    <AppStatus v-else-if="kps.length === 0" state="empty" message="还没有知识点。" />
    <div v-else class="card" style="overflow-x: auto">
      <table class="data">
        <thead>
          <tr>
            <th>名称</th>
            <th>级别</th>
            <th>科目</th>
            <th>状态</th>
            <th class="num">关联题目</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="k in kps" :key="k.id">
            <td>{{ k.name }}</td>
            <td class="mono">{{ k.levelId.slice(0, 6) }}</td>
            <td class="mono">{{ k.subjectId.slice(0, 6) }}</td>
            <td><StatusBadge :value="k.status" /></td>
            <td class="num">{{ k.questionCount }}</td>
            <td>
              <button v-if="k.status === 'draft'" style="min-height: 32px; font-size: 13px" @click="publish(k)">发布</button>
              <button v-else-if="k.status === 'published'" style="min-height: 32px; font-size: 13px" @click="unpublish(k)">转回草稿</button>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="nextCursor" style="margin: 12px 0 0; text-align: center">
        <button :disabled="loadingMore" @click="load(true)">{{ loadingMore ? '加载中…' : '加载更多' }}</button>
      </p>
    </div>
  </AppShell>
</template>
