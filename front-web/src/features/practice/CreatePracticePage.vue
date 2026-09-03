<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { request, ApiError } from '@/api/client'
import type { Exam, KnowledgePointItem, PracticeSource } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import AppStatus from '@/components/AppStatus.vue'
import { sessionUser } from '@/app/session'

const router = useRouter()

const exams = ref<Exam[]>([])
const loadState = ref<'loading' | 'ready' | 'error'>('loading')
const loadError = ref('')

const levelId = ref('')
const subjectId = ref('')
const sourceId = ref('')
const mode = ref<'comprehensive' | 'knowledge' | 'wrong_items'>('comprehensive')
const selectionOrder = ref<'source_order' | 'random'>('source_order')
const knowledgePointIds = ref<string[]>([])
const count = ref(20)

const availability = ref<number | null>(null)
const availabilityLoading = ref(false)
const availabilityError = ref('')
const creating = ref(false)
const createError = ref('')

const kps = ref<KnowledgePointItem[]>([])
const kpsLoading = ref(false)
const sources = ref<PracticeSource[]>([])
const sourcesLoading = ref(false)
const sourcesError = ref('')

onMounted(async () => {
  try {
    const res = await request<{ exams: Exam[] }>('/catalog')
    exams.value = res.exams
    levelId.value = sessionUser()?.defaultLevelId ?? res.exams[0]?.levels[0]?.id ?? ''
    loadState.value = 'ready'
  } catch (err) {
    loadError.value = err instanceof ApiError ? err.message : '加载失败'
    loadState.value = 'error'
  }
})

const levels = computed(() => exams.value.flatMap((e) => e.levels))
const subjects = computed(() => exams.value.flatMap((e) => e.subjects))

watch([levelId, subjectId, mode, selectionOrder, sourceId, knowledgePointIds], async () => {
  await refreshAvailability()
  if (mode.value === 'knowledge') {
    await loadKnowledgePoints()
  }
}, { deep: true })

watch([levelId, subjectId], async () => {
  if (!levelId.value) return
  sourcesLoading.value = true
  sourcesError.value = ''
  try {
    const params = new URLSearchParams({ levelId: levelId.value })
    if (subjectId.value) params.set('subjectId', subjectId.value)
    const res = await request<{ sources: PracticeSource[] }>(`/practice/sources?${params}`)
    sources.value = res.sources
    if (!sources.value.some((source) => source.id === sourceId.value)) {
      sourceId.value = ''
    }
  } catch (err) {
    sources.value = []
    sourceId.value = ''
    sourcesError.value = err instanceof ApiError ? err.message : '数据来源加载失败'
  } finally {
    sourcesLoading.value = false
  }
}, { immediate: true })

let availabilityTimer: ReturnType<typeof setTimeout> | null = null
function refreshAvailability(): void {
  if (!levelId.value) return
  if (availabilityTimer) clearTimeout(availabilityTimer)
  availabilityTimer = setTimeout(async () => {
    availabilityLoading.value = true
    availabilityError.value = ''
    try {
      const params = new URLSearchParams({
        levelId: levelId.value,
        mode: mode.value,
        selectionOrder: selectionOrder.value,
        count: '10',
      })
      if (subjectId.value) params.set('subjectId', subjectId.value)
      if (sourceId.value) params.set('sourceId', sourceId.value)
      if (mode.value === 'knowledge' && knowledgePointIds.value.length) {
        params.set('knowledgePointIds', knowledgePointIds.value.join(','))
      }
      const res = await request<{ available: number }>(`/practice/availability?${params}`)
      availability.value = res.available
    } catch (err) {
      availabilityError.value = err instanceof ApiError ? err.message : '可用题量查询失败'
      availability.value = null
    } finally {
      availabilityLoading.value = false
    }
  }, 250)
}

async function loadKnowledgePoints(): Promise<void> {
  if (!levelId.value) return
  kpsLoading.value = true
  try {
    const params = new URLSearchParams({ levelId: levelId.value })
    if (subjectId.value) params.set('subjectId', subjectId.value)
    const res = await request<{ knowledgePoints: KnowledgePointItem[] }>(`/knowledge-points?${params}`)
    kps.value = res.knowledgePoints
  } finally {
    kpsLoading.value = false
  }
}

const insufficient = computed(() => availability.value !== null && availability.value < count.value)
const canCreate = computed(
  () =>
    !!levelId.value &&
    (mode.value !== 'knowledge' || knowledgePointIds.value.length > 0) &&
    availability.value !== null &&
    !insufficient.value &&
    !availabilityLoading.value,
)

async function create(): Promise<void> {
  creating.value = true
  createError.value = ''
  try {
    const session = await request<{ id: string }>('/practice-sessions', {
      method: 'POST',
      body: {
        levelId: levelId.value,
        subjectId: subjectId.value,
        sourceId: sourceId.value,
        mode: mode.value,
        selectionOrder: selectionOrder.value,
        knowledgePointIds: knowledgePointIds.value,
        count: count.value,
      },
    })
    await router.push(`/practice/${session.id}`)
  } catch (err) {
    if (err instanceof ApiError && err.code === 'insufficient_questions') {
      availability.value = Number((err.details as { available?: number } | undefined)?.available ?? 0)
    }
    createError.value = err instanceof ApiError ? err.message : '创建失败，请重试'
  } finally {
    creating.value = false
  }
}
</script>

<template>
  <AppShell>
    <AppStatus v-if="loadState === 'loading'" state="loading" />
    <AppStatus v-else-if="loadState === 'error'" state="error" :message="loadError" @action="loadState = 'ready'" />
    <template v-else>
      <h1>创建练习</h1>
      <p class="muted">系统只从已发布题目中选题；答题过程中不显示答案，整批提交后统一判分。</p>

      <form class="card" @submit.prevent="create">
        <fieldset class="field" style="border: 0; padding: 0; margin: 0 0 14px">
          <legend style="font-weight: 600; margin-bottom: 6px">级别</legend>
          <div style="display: flex; gap: 10px; flex-wrap: wrap">
            <label v-for="l in levels" :key="l.id" class="option-row" style="margin-bottom: 0">
              <input v-model="levelId" type="radio" name="level" :value="l.id" />
              <span>{{ l.name }}</span>
            </label>
          </div>
        </fieldset>

        <div class="field">
          <label for="subject">科目（可选）</label>
          <select id="subject" v-model="subjectId">
            <option value="">全部科目</option>
            <option v-for="s in subjects" :key="s.id" :value="s.id">{{ s.name }}</option>
          </select>
        </div>

        <div class="field">
          <label for="source">数据来源（可选）</label>
          <select id="source" v-model="sourceId" :disabled="sourcesLoading">
            <option value="">全部题目</option>
            <option v-for="source in sources" :key="source.id" :value="source.id">
              {{ source.name }}（{{ source.questionCount }}题）
            </option>
          </select>
          <p v-if="sourcesLoading" class="muted">加载数据来源…</p>
          <p v-else-if="sourcesError" class="error">{{ sourcesError }}</p>
          <p v-else-if="sources.length === 0" class="muted">当前级别暂无可用数据来源。</p>
        </div>

        <fieldset class="field" style="border: 0; padding: 0; margin: 0 0 14px">
          <legend style="font-weight: 600; margin-bottom: 6px">练习范围</legend>
          <div style="display: flex; gap: 10px; flex-wrap: wrap">
            <label class="option-row" style="margin-bottom: 0">
              <input v-model="mode" type="radio" name="mode" value="comprehensive" />
              <span>综合</span>
            </label>
            <label class="option-row" style="margin-bottom: 0">
              <input v-model="mode" type="radio" name="mode" value="knowledge" />
              <span>按知识点</span>
            </label>
            <label class="option-row" style="margin-bottom: 0">
              <input v-model="mode" type="radio" name="mode" value="wrong_items" />
              <span>错题重练</span>
            </label>
          </div>
        </fieldset>

        <fieldset class="field" style="border: 0; padding: 0; margin: 0 0 14px">
          <legend style="font-weight: 600; margin-bottom: 6px">出题顺序</legend>
          <div style="display: flex; gap: 10px; flex-wrap: wrap">
            <label class="option-row" style="margin-bottom: 0">
              <input v-model="selectionOrder" type="radio" name="selectionOrder" value="source_order" />
              <span>按 PDF 章节顺序</span>
            </label>
            <label class="option-row" style="margin-bottom: 0">
              <input v-model="selectionOrder" type="radio" name="selectionOrder" value="random" />
              <span>随机</span>
            </label>
          </div>
        </fieldset>

        <div v-if="mode === 'knowledge'" class="field">
          <span style="font-weight: 600; display: block; margin-bottom: 6px">知识点（可多选）</span>
          <p v-if="kpsLoading" class="muted">加载知识点…</p>
          <template v-else>
            <label v-for="k in kps" :key="k.id" class="option-row">
              <input v-model="knowledgePointIds" type="checkbox" :value="k.id" />
              <span>
                {{ k.name }}
                <span class="muted" style="margin-left: 6px">{{ k.subjectName }} · {{ k.questionCount }} 题可用</span>
              </span>
            </label>
            <p v-if="kps.length === 0" class="muted">该级别下暂无已发布知识点。</p>
          </template>
        </div>

        <fieldset class="field" style="border: 0; padding: 0; margin: 0 0 14px">
          <legend style="font-weight: 600; margin-bottom: 6px">题量</legend>
          <div style="display: flex; gap: 10px">
            <label v-for="c in [10, 20, 30]" :key="c" class="option-row" style="margin-bottom: 0">
              <input v-model.number="count" type="radio" name="count" :value="c" />
              <span class="mono">{{ c }} 题</span>
            </label>
          </div>
        </fieldset>

        <p role="status" class="mono muted" style="min-height: 22px">
          <template v-if="availabilityLoading">查询可用题量…</template>
          <template v-else-if="availabilityError">{{ availabilityError }}</template>
          <template v-else-if="availability !== null">当前范围可用 {{ availability }} 题</template>
        </p>

        <p v-if="insufficient" class="error-summary" role="alert">
          该范围只有 {{ availability }} 题，不足 {{ count }} 题。系统不会自动放宽级别或科目，请调整范围或选择较小题量。
        </p>
        <p v-if="createError" class="error-summary" role="alert">{{ createError }}</p>

        <button class="primary" type="submit" :disabled="!canCreate || creating">
          {{ creating ? '创建中…' : '开始练习' }}
        </button>
      </form>
    </template>
  </AppShell>
</template>
