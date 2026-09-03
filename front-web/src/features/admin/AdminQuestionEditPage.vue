<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { request, ApiError, fieldErrors } from '@/api/client'
import type { AdminKnowledgePoint, Exam, OptionDTO, QuestionAdminDTO, SourceDTO } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import AppStatus from '@/components/AppStatus.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { questionTypeText } from '@/app/format'

const route = useRoute()
const router = useRouter()
const questionID = computed(() => route.params.questionId as string | undefined)

const exams = ref<Exam[]>([])
const sources = ref<SourceDTO[]>([])
const kps = ref<AdminKnowledgePoint[]>([])

const pageState = ref<'loading' | 'ready' | 'error' | 'notfound'>('loading')
const loadError = ref('')
const requestID = ref('')

const form = reactive({
  type: 'single_choice' as 'single_choice' | 'multiple_choice' | 'fill_blank' | 'short_answer',
  stem: '',
  options: [] as Array<{ id: string; label: string; text: string }>,
  useMaterial: false,
  materialTitle: '',
  materialContent: '',
  levelId: '',
  subjectId: '',
  sourceSectionId: '',
  difficulty: 3,
  knowledgePointIds: [] as string[],
  hasKey: true,
  correctOptionIds: [] as string[],
  acceptableText: '',
  referenceText: '',
  authority: 'official' as 'official' | 'human_verified',
  explanation: '',
})

const status = ref<'draft' | 'in_review' | 'published' | 'retired'>('draft')
const hasPublishedVersion = ref(false)
const saving = ref(false)
const publishing = ref(false)
const fieldErr = reactive<Record<string, string>>({})
const topError = ref('')
const info = ref('')
const confirmPublish = ref(false)

const isChoice = computed(() => form.type === 'single_choice' || form.type === 'multiple_choice')
onMounted(async () => {
  try {
    const [catalog, sourceRes] = await Promise.all([
      request<{ exams: Exam[] }>('/catalog'),
      request<{ sources: SourceDTO[] }>('/admin/sources'),
    ])
    exams.value = catalog.exams
    sources.value = sourceRes.sources
    if (questionID.value) {
      await loadQuestion()
    } else {
      form.levelId = exams.value[0]?.levels[0]?.id ?? ''
      form.subjectId = exams.value[0]?.subjects[0]?.id ?? ''
      ensureOptionCount()
    }
    await loadKPs()
    pageState.value = 'ready'
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      pageState.value = 'notfound'
      return
    }
    loadError.value = err instanceof ApiError ? err.message : '加载失败'
    requestID.value = err instanceof ApiError ? err.requestId ?? '' : ''
    pageState.value = 'error'
  }
})

async function loadQuestion(): Promise<void> {
  const res = await request<{ question: QuestionAdminDTO }>(`/admin/questions/${questionID.value}`)
  const q = res.question
  status.value = q.status
  hasPublishedVersion.value = q.publishedVersionId !== null
  const v = q.currentVersion
  if (!v) return
  form.type = v.type
  form.stem = v.stem
  form.options = v.options ? JSON.parse(JSON.stringify(v.options)) as OptionDTO[] : []
  if (v.materialContent) {
    form.useMaterial = true
    form.materialTitle = v.materialTitle ?? ''
    form.materialContent = v.materialContent
  }
  form.levelId = v.levelId
  form.subjectId = v.subjectId
  form.sourceSectionId = v.sourceSectionId ?? ''
  form.difficulty = v.difficulty
  form.knowledgePointIds = [...v.knowledgePointIds]
  const key = v.answerKey
  if (key) {
    form.hasKey = true
    form.authority = key.authority
    form.explanation = key.explanation
    if (Array.isArray(key.value.optionIds)) {
      form.correctOptionIds = [...(key.value.optionIds as string[])]
    }
    if (Array.isArray(key.value.acceptable)) {
      form.acceptableText = (key.value.acceptable as string[]).join('\n')
    }
    if (typeof key.value.reference === 'string') {
      form.referenceText = key.value.reference
    }
  } else {
    form.hasKey = false
  }
}

async function loadKPs(): Promise<void> {
  if (!form.levelId) return
  const params = new URLSearchParams({ levelId: form.levelId })
  if (form.subjectId) params.set('subjectId', form.subjectId)
  const res = await request<{ knowledgePoints: AdminKnowledgePoint[] }>(`/admin/knowledge-points?${params}`)
  kps.value = res.knowledgePoints
}

watch(() => [form.levelId, form.subjectId], () => void loadKPs())

watch(() => form.type, () => {
  if (isChoice.value) ensureOptionCount()
})

function ensureOptionCount(): void {
  while (form.options.length < 4) {
    const letter = String.fromCharCode(65 + form.options.length)
    form.options.push({ id: letter.toLowerCase(), label: letter, text: '' })
  }
}

function addOption(): void {
  const n = form.options.length
  const letter = String.fromCharCode(65 + n)
  form.options.push({ id: `${letter.toLowerCase()}${n}`, label: letter, text: '' })
}

function removeOption(index: number): void {
  if (form.options.length <= 2) return
  form.options.splice(index, 1)
}

function buildRequestBody(): Record<string, unknown> {
  const body: Record<string, unknown> = {
    type: form.type,
    stem: form.stem,
    options: isChoice.value ? form.options : [],
    levelId: form.levelId,
    subjectId: form.subjectId,
    difficulty: form.difficulty,
    knowledgePointIds: form.knowledgePointIds,
    sourceSectionId: form.sourceSectionId || null,
  }
  if (form.useMaterial && form.materialContent) {
    body.materialTitle = form.materialTitle
    body.materialContent = form.materialContent
  }
  if (form.hasKey) {
    let value: Record<string, unknown> | null = null
    if (isChoice.value && form.correctOptionIds.length > 0) {
      value = { optionIds: [...form.correctOptionIds].sort() }
    } else if (form.type === 'fill_blank') {
      value = { acceptable: form.acceptableText.split('\n').map((s) => s.trim()).filter(Boolean) }
    } else if (form.type === 'short_answer' && form.referenceText.trim()) {
      value = { reference: form.referenceText.trim() }
    }
    if (value) {
      body.answer = { value, authority: form.authority, explanation: form.explanation }
    }
  }
  return body
}

async function save(): Promise<boolean> {
  saving.value = true
  topError.value = ''
  info.value = ''
  for (const k of Object.keys(fieldErr)) delete fieldErr[k]
  try {
    if (questionID.value) {
      await request<{ question: QuestionAdminDTO }>(`/admin/questions/${questionID.value}`, {
        method: 'PATCH',
        body: buildRequestBody(),
      })
      await loadQuestion()
    } else {
      const res = await request<{ question: QuestionAdminDTO }>('/admin/questions', {
        method: 'POST',
        body: buildRequestBody(),
      })
      await router.replace(`/admin/questions/${res.question.id}`)
    }
    info.value = '草稿已保存。发布是独立操作，需要再次确认。'
    return true
  } catch (err) {
    const fields = fieldErrors(err)
    for (const [k, v] of Object.entries(fields)) fieldErr[k] = v
    topError.value = Object.keys(fields).length ? '请修正表单错误' : err instanceof ApiError ? err.message : '保存失败，请重试'
    return false
  } finally {
    saving.value = false
  }
}

async function submitReview(): Promise<void> {
  if (!(await save())) return
  try {
    const res = await request<{ question: QuestionAdminDTO }>(`/admin/questions/${questionID.value}/submit-review`, { method: 'POST' })
    status.value = res.question.status
    info.value = '已提交审核。'
  } catch (err) {
    topError.value = err instanceof ApiError ? err.message : '操作失败'
  }
}

async function publish(): Promise<void> {
  if (!form.hasKey) {
    topError.value = '客观题没有标准答案也可以发布，但请先确认：本题将走 AI 判定。再次点击“确认发布”继续。'
    confirmPublish.value = true
    return
  }
  confirmPublish.value = true
}

async function doPublish(): Promise<void> {
  publishing.value = true
  topError.value = ''
  try {
    const res = await request<{ question: QuestionAdminDTO }>(`/admin/questions/${questionID.value}/publish`, { method: 'POST' })
    status.value = res.question.status
    hasPublishedVersion.value = res.question.publishedVersionId !== null
    info.value = '已发布新版本。历史练习仍引用旧版本，不受影响。'
  } catch (err) {
    topError.value = err instanceof ApiError ? err.message : '发布失败'
  } finally {
    publishing.value = false
    confirmPublish.value = false
  }
}

async function retire(): Promise<void> {
  if (!confirm('确认下架该题目？下架后不再进入新练习，历史记录保留。')) return
  try {
    const res = await request<{ question: QuestionAdminDTO }>(`/admin/questions/${questionID.value}/retire`, { method: 'POST' })
    status.value = res.question.status
    info.value = '已下架。'
  } catch (err) {
    topError.value = err instanceof ApiError ? err.message : '操作失败'
  }
}
</script>

<template>
  <AppShell>
    <AppStatus v-if="pageState === 'loading'" state="loading" />
    <AppStatus v-else-if="pageState === 'error'" state="error" :message="loadError" :request-id="requestID" @action="pageState = 'ready'" />
    <AppStatus v-else-if="pageState === 'notfound'" state="empty" message="题目不存在。" action-label="返回题目列表" @action="router.push('/admin/questions')" />
    <template v-else>
      <div class="page-header">
        <h1 style="font-size: 24px; margin: 0">{{ questionID ? '编辑题目' : '新建题目' }}</h1>
        <div style="display: flex; gap: 8px; align-items: center">
          <span class="tag">{{ status === 'draft' ? '草稿' : status === 'in_review' ? '待审核' : status === 'published' ? '已发布' : '已下架' }}</span>
          <span v-if="hasPublishedVersion" class="tag" data-tone="success">对外提供已发布版本</span>
        </div>
      </div>
      <p v-if="info" class="tag" data-tone="success" role="status">{{ info }}</p>
      <p v-if="topError" class="error-summary" role="alert">{{ topError }}</p>

      <form class="card" @submit.prevent="save">
        <div class="grid-2">
          <div class="field">
            <label for="q-type">题型</label>
            <select id="q-type" v-model="form.type">
              <option v-for="(label, value) in questionTypeText" :key="value" :value="value">{{ label }}</option>
            </select>
          </div>
          <div class="field">
            <label for="q-difficulty">难度（1-5）</label>
            <input id="q-difficulty" v-model.number="form.difficulty" type="number" min="1" max="5" />
            <p v-if="fieldErr.difficulty" class="error">{{ fieldErr.difficulty }}</p>
          </div>
        </div>

        <div class="field">
          <label for="q-stem">题干</label>
          <textarea id="q-stem" v-model="form.stem" rows="3" lang="ja" :aria-describedby="fieldErr.stem ? 'err-stem' : undefined" />
          <p v-if="fieldErr.stem" id="err-stem" class="error">{{ fieldErr.stem }}</p>
        </div>

        <template v-if="isChoice">
          <div class="field">
            <span>选项</span>
            <div v-for="(opt, i) in form.options" :key="i" style="display: flex; gap: 8px; margin-bottom: 8px">
              <input v-model="opt.label" type="text" style="width: 64px; flex: none" aria-label="选项标号" />
              <input v-model="opt.text" type="text" lang="ja" aria-label="选项内容" />
              <button type="button" class="ghost danger" :disabled="form.options.length <= 2" @click="removeOption(i)">删除</button>
            </div>
            <button type="button" @click="addOption">添加选项</button>
            <p v-if="fieldErr.options" class="error">{{ fieldErr.options }}</p>
          </div>
        </template>

        <fieldset style="border: 0; padding: 0; margin: 0 0 14px">
          <legend style="font-weight: 600; margin-bottom: 6px">共享材料（可选）</legend>
          <label style="display: flex; gap: 8px; align-items: center; margin-bottom: 8px">
            <input v-model="form.useMaterial" type="checkbox" />
            <span>本题附带阅读材料</span>
          </label>
          <template v-if="form.useMaterial">
            <div class="field">
              <label for="m-title">材料标题</label>
              <input id="m-title" v-model="form.materialTitle" type="text" />
            </div>
            <div class="field">
              <label for="m-content">材料正文（日文原文）</label>
              <textarea id="m-content" v-model="form.materialContent" rows="5" lang="ja" />
            </div>
          </template>
        </fieldset>

        <div class="grid-2">
          <div class="field">
            <label for="q-level">级别</label>
            <select id="q-level" v-model="form.levelId">
              <option v-for="l in exams.flatMap((e) => e.levels)" :key="l.id" :value="l.id">{{ l.name }}</option>
            </select>
            <p v-if="fieldErr.levelId" class="error">{{ fieldErr.levelId }}</p>
          </div>
          <div class="field">
            <label for="q-subject">科目</label>
            <select id="q-subject" v-model="form.subjectId">
              <option v-for="s in exams.flatMap((e) => e.subjects)" :key="s.id" :value="s.id">{{ s.name }}</option>
            </select>
            <p v-if="fieldErr.subjectId" class="error">{{ fieldErr.subjectId }}</p>
          </div>
        </div>

        <div class="field">
          <label for="q-section">来源章节（可选）</label>
          <select id="q-section" v-model="form.sourceSectionId">
            <option value="">不关联</option>
            <optgroup v-for="source in sources" :key="source.id" :label="source.name">
              <option v-for="sec in source.sections" :key="sec.id" :value="sec.id">{{ sec.name }}</option>
            </optgroup>
          </select>
          <p v-if="fieldErr.sourceSectionId" class="error">{{ fieldErr.sourceSectionId }}</p>
        </div>

        <div class="field">
          <span>知识点（可多选）</span>
          <label v-for="k in kps" :key="k.id" class="option-row">
            <input v-model="form.knowledgePointIds" type="checkbox" :value="k.id" />
            <span>{{ k.name }} <span class="muted">{{ k.status === 'published' ? '已发布' : '草稿' }}</span></span>
          </label>
          <p v-if="fieldErr.knowledgePointIds" class="error">{{ fieldErr.knowledgePointIds }}</p>
        </div>

        <fieldset style="border: 1px solid var(--border); border-radius: 9px; padding: 14px; margin: 0 0 14px">
          <legend style="font-weight: 600; padding: 0 6px">标准答案（独立私有表，练习接口永不返回）</legend>
          <label style="display: flex; gap: 8px; align-items: center; margin-bottom: 10px">
            <input v-model="form.hasKey" type="checkbox" />
            <span>本题提供标准答案</span>
          </label>
          <template v-if="form.hasKey">
            <template v-if="isChoice">
              <div class="field">
                <span>正确选项（{{ form.type === 'multiple_choice' ? '可多选' : '单选' }}）</span>
                <label v-for="opt in form.options" :key="opt.id" class="option-row" style="margin-bottom: 6px">
                  <input
                    v-model="form.correctOptionIds"
                    :type="form.type === 'multiple_choice' ? 'checkbox' : 'radio'"
                    name="correct-option"
                    :value="opt.id"
                  />
                  <span class="mono">{{ opt.label }}. {{ opt.text }}</span>
                </label>
              </div>
            </template>
            <div v-else-if="form.type === 'fill_blank'" class="field">
              <label for="q-acceptable">可接受答案（每行一个）</label>
              <textarea id="q-acceptable" v-model="form.acceptableText" rows="3" lang="ja" />
            </div>
            <div v-else class="field">
              <label for="q-reference">参考答案（供 AI 判定约束）</label>
              <textarea id="q-reference" v-model="form.referenceText" rows="2" lang="ja" />
            </div>

            <div class="grid-2">
              <div class="field">
                <label for="q-authority">答案来源</label>
                <select id="q-authority" v-model="form.authority">
                  <option value="official">官方答案（来自来源材料）</option>
                  <option value="human_verified">人工审核答案</option>
                </select>
              </div>
              <div class="field">
                <label for="q-explanation">来源解析（可选）</label>
                <textarea id="q-explanation" v-model="form.explanation" rows="2" />
              </div>
            </div>
            <p class="muted" style="font-size: 13px; margin: 0">
              AI 只能在标准答案约束下生成解析；没有标准答案的题发布后练习时将走 AI 判定，不计入正式正确率。
            </p>
          </template>
        </fieldset>

        <div style="display: flex; gap: 10px; flex-wrap: wrap">
          <button class="primary" type="submit" :disabled="saving">{{ saving ? '保存中…' : '保存草稿' }}</button>
          <button v-if="questionID && status === 'draft'" type="button" :disabled="saving" @click="submitReview">提交审核</button>
          <button v-if="questionID && status !== 'retired'" type="button" :disabled="saving || publishing" @click="publish">发布新版本</button>
          <button v-if="questionID && status !== 'retired'" type="button" class="danger" @click="retire">下架</button>
        </div>
      </form>

      <ConfirmDialog
        :open="confirmPublish"
        title="确认发布？"
        confirm-label="确认发布"
        cancel-label="再检查一下"
        @confirm="doPublish"
        @cancel="confirmPublish = false"
      >
        <p v-if="!form.hasKey">本题没有标准答案：发布后练习中该题将标记为“AI 判定”，不计入正式正确率。</p>
        <p v-else>发布将创建新的当前版本；旧版本继续支撑历史练习复现。</p>
      </ConfirmDialog>
    </template>
  </AppShell>
</template>
