<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { request, ApiError, fieldErrors } from '@/api/client'
import type { AdminKnowledgePoint, Exam, ImportAnswerDTO, ImportDraftDTO, ImportItemDTO, ImportSuggestionDTO, OptionDTO, SourceDTO } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import AppStatus from '@/components/AppStatus.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { questionTypeText } from '@/app/format'

const route = useRoute()
const itemID = route.params.importItemId as string
const item = ref<ImportItemDTO | null>(null)
const exams = ref<Exam[]>([])
const sources = ref<SourceDTO[]>([])
const kps = ref<AdminKnowledgePoint[]>([])
const kpNextCursor = ref('')
const kpLoadingMore = ref(false)
const state = ref<'loading' | 'ready' | 'error'>('loading')
const errorMessage = ref('')
const requestID = ref('')
const topError = ref('')
const info = ref('')
const saving = ref(false)
const approving = ref(false)
const publishing = ref(false)
const confirmPublish = ref(false)
const fieldErr = reactive<Record<string, string>>({})

const form = reactive({
  materialKey: '',
  type: 'single_choice' as ImportDraftDTO['type'],
  stem: '',
  options: [] as OptionDTO[],
  materialTitle: '',
  materialContent: '',
  levelId: '',
  subjectId: '',
  sourceSectionId: '',
  difficulty: 3,
  knowledgePointIds: [] as string[],
  hasAnswer: false,
  correctOptionIds: [] as string[],
  acceptableText: '',
  referenceText: '',
  authority: 'official' as ImportAnswerDTO['authority'],
  explanation: '',
})
const sourceAnswer = ref<ImportAnswerDTO | undefined>()
const aiSuggestedAnswer = ref<ImportSuggestionDTO | undefined>()

const isChoice = computed(() => form.type === 'single_choice' || form.type === 'multiple_choice')
const sections = computed(() => sources.value.flatMap((source) => source.sections.map((section) => ({ ...section, sourceName: source.name }))))
const suggestionText = computed(() => aiSuggestedAnswer.value ? JSON.stringify(aiSuggestedAnswer.value.value) : '')

function stringList(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((v): v is string => typeof v === 'string') : []
}

function optionIds(value: Record<string, unknown> | undefined): string[] {
  return stringList(value?.optionIds)
}

function acceptable(value: Record<string, unknown> | undefined): string[] {
  return stringList(value?.acceptable)
}

function applyDraft(draft: ImportDraftDTO): void {
  form.materialKey = draft.materialKey ?? ''
  form.type = draft.type
  form.stem = draft.stem
  form.options = draft.options ? JSON.parse(JSON.stringify(draft.options)) as OptionDTO[] : []
  form.materialTitle = draft.materialTitle ?? ''
  form.materialContent = draft.materialContent ?? ''
  form.levelId = draft.levelId
  form.subjectId = draft.subjectId
  form.sourceSectionId = draft.sourceSectionId ?? ''
  form.difficulty = draft.difficulty
  form.knowledgePointIds = [...draft.knowledgePointIds]
  sourceAnswer.value = draft.sourceAnswer
  aiSuggestedAnswer.value = draft.aiSuggestedAnswer
  form.hasAnswer = draft.answer !== undefined
  form.authority = draft.answer?.authority ?? 'official'
  form.explanation = draft.answer?.explanation ?? ''
  form.correctOptionIds = optionIds(draft.answer?.value)
  form.acceptableText = acceptable(draft.answer?.value).join('\n')
  form.referenceText = typeof draft.answer?.value.reference === 'string' ? draft.answer.value.reference : ''
}

async function load(): Promise<void> {
  state.value = 'loading'
  try {
    const [itemRes, catalogRes, sourceRes] = await Promise.all([
      request<{ item: ImportItemDTO }>(`/admin/import-items/${itemID}`),
      request<{ exams: Exam[] }>('/catalog'),
      request<{ sources: SourceDTO[] }>('/admin/sources'),
    ])
    item.value = itemRes.item
    exams.value = catalogRes.exams
    sources.value = sourceRes.sources
    if (item.value.draft) applyDraft(item.value.draft)
    await loadKPs()
    state.value = 'ready'
  } catch (err) {
    errorMessage.value = err instanceof ApiError ? err.message : '加载失败'
    requestID.value = err instanceof ApiError ? err.requestId ?? '' : ''
    state.value = 'error'
  }
}

async function loadKPs(append = false): Promise<void> {
  if (!form.levelId) return
  if (!append) kpNextCursor.value = ''
  const params = new URLSearchParams({ levelId: form.levelId, limit: '20' })
  if (form.subjectId) params.set('subjectId', form.subjectId)
  if (append && kpNextCursor.value) params.set('cursor', kpNextCursor.value)
  if (append) kpLoadingMore.value = true
  try {
    const res = await request<{ knowledgePoints: AdminKnowledgePoint[]; nextCursor?: string }>(`/admin/knowledge-points?${params}`)
    kps.value = append ? [...kps.value, ...res.knowledgePoints] : res.knowledgePoints
    kpNextCursor.value = res.nextCursor ?? ''
  } finally {
    kpLoadingMore.value = false
  }
}

watch(() => [form.levelId, form.subjectId], () => void loadKPs())

function ensureOptionCount(): void {
  while (form.options.length < 2) {
    const index = form.options.length
    form.options.push({ id: String.fromCharCode(97 + index), label: String.fromCharCode(65 + index), text: '' })
  }
}

function addOption(): void {
  const index = form.options.length
  form.options.push({ id: `option-${index + 1}`, label: String.fromCharCode(65 + index), text: '' })
}

function removeOption(index: number): void {
  if (form.options.length <= 2) return
  form.options.splice(index, 1)
}

function answerValue(): Record<string, unknown> | null {
  if (isChoice.value) return form.correctOptionIds.length ? { optionIds: [...form.correctOptionIds] } : null
  if (form.type === 'fill_blank') {
    const values = form.acceptableText.split('\n').map((value) => value.trim()).filter(Boolean)
    return values.length ? { acceptable: values } : null
  }
  return form.referenceText.trim() ? { reference: form.referenceText.trim() } : null
}

function buildDraft(): ImportDraftDTO {
  const draft: ImportDraftDTO = {
    materialKey: form.materialKey || undefined,
    type: form.type,
    stem: form.stem,
    options: isChoice.value ? form.options : [],
    materialTitle: form.materialTitle,
    materialContent: form.materialContent,
    levelId: form.levelId,
    subjectId: form.subjectId,
    sourceSectionId: form.sourceSectionId || null,
    difficulty: form.difficulty,
    knowledgePointIds: form.knowledgePointIds,
    sourceAnswer: sourceAnswer.value,
    aiSuggestedAnswer: aiSuggestedAnswer.value,
  }
  const value = form.hasAnswer ? answerValue() : null
  if (value) draft.answer = { value, authority: form.authority, explanation: form.explanation }
  return draft
}

async function save(): Promise<void> {
  saving.value = true
  topError.value = ''
  info.value = ''
  for (const key of Object.keys(fieldErr)) delete fieldErr[key]
  try {
    const res = await request<{ item: ImportItemDTO }>(`/admin/import-items/${itemID}`, { method: 'PATCH', body: { draft: buildDraft() } })
    item.value = res.item
    info.value = '草稿已保存；保存后需要重新审核。'
  } catch (err) {
    const fields = fieldErrors(err)
    for (const [key, value] of Object.entries(fields)) fieldErr[key] = value
    topError.value = Object.keys(fields).length ? '请修正表单错误' : err instanceof ApiError ? err.message : '保存失败，请重试'
  } finally {
    saving.value = false
  }
}

async function approve(): Promise<void> {
  approving.value = true
  topError.value = ''
  try {
    const res = await request<{ item: ImportItemDTO }>(`/admin/import-items/${itemID}/approve`, { method: 'POST' })
    item.value = res.item
    info.value = '已审核，可以发布。'
  } catch (err) {
    topError.value = err instanceof ApiError ? err.message : '审核失败'
  } finally {
    approving.value = false
  }
}

function askPublish(): void {
  confirmPublish.value = true
}

async function publish(): Promise<void> {
  publishing.value = true
  topError.value = ''
  try {
    const res = await request<{ item: ImportItemDTO }>(`/admin/import-items/${itemID}/publish`, { method: 'POST' })
    item.value = res.item
    info.value = '已发布到题库；题目后续编辑仍会创建新版本。'
  } catch (err) {
    topError.value = err instanceof ApiError ? err.message : '发布失败'
  } finally {
    publishing.value = false
    confirmPublish.value = false
  }
}

onMounted(() => void load())
</script>

<template>
  <AppShell>
    <AppStatus v-if="state === 'loading'" state="loading" />
    <AppStatus v-else-if="state === 'error'" state="error" :message="errorMessage" :request-id="requestID" @action="load" />
    <template v-else-if="item">
      <div class="page-header">
        <div>
          <p class="muted" style="margin: 0 0 4px"><RouterLink :to="`/admin/imports/${item.jobId}`">导入任务</RouterLink> / 第 {{ item.position }} 题</p>
          <h1 style="font-size: 24px; margin: 0">原文对照审核</h1>
        </div>
        <StatusBadge :value="item.reviewStatus" />
      </div>
      <p v-if="info" class="tag" data-tone="success" role="status">{{ info }}</p>
      <p v-if="topError" class="error-summary" role="alert">{{ topError }}</p>
      <div v-if="item.anomalies.length" class="error-summary">
        <strong>需要人工确认：</strong>
        <ul><li v-for="anomaly in item.anomalies" :key="anomaly">{{ anomaly }}</li></ul>
      </div>

      <div class="import-review-grid">
        <section class="card">
          <h2 style="font-size: 18px">原文片段</h2>
          <pre class="import-source" lang="ja">{{ item.rawExcerpt }}</pre>
          <div v-if="aiSuggestedAnswer" class="import-note">
            <strong>AI 建议答案（仅供参考）</strong>
            <p class="mono">{{ suggestionText }}</p>
            <p v-if="aiSuggestedAnswer.explanation">{{ aiSuggestedAnswer.explanation }}</p>
          </div>
        </section>

        <form class="card" @submit.prevent="save">
          <h2 style="font-size: 18px">结构化题目</h2>
          <div class="grid-2">
            <div class="field"><label for="import-type">题型</label><select id="import-type" v-model="form.type" @change="ensureOptionCount"><option v-for="(label, value) in questionTypeText" :key="value" :value="value">{{ label }}</option></select></div>
            <div class="field"><label for="import-difficulty">难度（1-5）</label><input id="import-difficulty" v-model.number="form.difficulty" type="number" min="1" max="5" /></div>
          </div>
          <div class="field"><label for="import-stem">题干</label><textarea id="import-stem" v-model="form.stem" rows="4" lang="ja" :aria-describedby="fieldErr.stem ? 'import-stem-error' : undefined" /><p v-if="fieldErr.stem" id="import-stem-error" class="error">{{ fieldErr.stem }}</p></div>

          <div v-if="isChoice" class="field">
            <span>选项</span>
            <div v-for="(option, index) in form.options" :key="option.id" class="import-option-row">
              <input v-model="option.label" type="text" aria-label="选项标号" />
              <input v-model="option.text" type="text" lang="ja" aria-label="选项内容" />
              <button type="button" class="ghost danger" :disabled="form.options.length <= 2" @click="removeOption(index)">删除</button>
            </div>
            <button type="button" @click="addOption">添加选项</button>
            <p v-if="fieldErr.options" class="error">{{ fieldErr.options }}</p>
          </div>

          <div class="field"><label for="import-material-key">共享材料标识（相同标识会复用材料）</label><input id="import-material-key" v-model="form.materialKey" type="text" /><label for="import-material-title" style="margin-top: 8px">材料标题</label><input id="import-material-title" v-model="form.materialTitle" type="text" /><label for="import-material-content" style="margin-top: 8px">材料正文</label><textarea id="import-material-content" v-model="form.materialContent" rows="5" lang="ja" /></div>

          <div class="grid-2">
            <div class="field"><label for="import-level">级别</label><select id="import-level" v-model="form.levelId"><option v-for="level in exams.flatMap((exam) => exam.levels)" :key="level.id" :value="level.id">{{ level.name }}</option></select><p v-if="fieldErr.levelId" class="error">{{ fieldErr.levelId }}</p></div>
            <div class="field"><label for="import-subject">科目</label><select id="import-subject" v-model="form.subjectId"><option v-for="subject in exams.flatMap((exam) => exam.subjects)" :key="subject.id" :value="subject.id">{{ subject.name }}</option></select><p v-if="fieldErr.subjectId" class="error">{{ fieldErr.subjectId }}</p></div>
          </div>
          <div class="field"><label for="import-section">来源章节</label><select id="import-section" v-model="form.sourceSectionId"><option value="">不关联</option><option v-for="section in sections" :key="section.id" :value="section.id">{{ section.sourceName }} / {{ section.name }}</option></select></div>
          <div class="field"><span>知识点</span><label v-for="kp in kps" :key="kp.id" class="option-row"><input v-model="form.knowledgePointIds" type="checkbox" :value="kp.id" />{{ kp.name }}</label><button v-if="kpNextCursor" type="button" :disabled="kpLoadingMore" @click="loadKPs(true)">{{ kpLoadingMore ? '加载中…' : '加载更多知识点' }}</button></div>

          <fieldset class="import-answer-fieldset">
            <legend>标准答案</legend>
            <label class="option-row"><input v-model="form.hasAnswer" type="checkbox" />确认本题有标准答案</label>
            <template v-if="form.hasAnswer">
              <div v-if="isChoice" class="field"><span>正确选项</span><label v-for="option in form.options" :key="option.id" class="option-row"><input v-model="form.correctOptionIds" :type="form.type === 'multiple_choice' ? 'checkbox' : 'radio'" name="import-correct-option" :value="option.id" />{{ option.label }}. {{ option.text }}</label></div>
              <div v-else-if="form.type === 'fill_blank'" class="field"><label for="import-acceptable">可接受答案（每行一个）</label><textarea id="import-acceptable" v-model="form.acceptableText" rows="3" lang="ja" /></div>
              <div v-else class="field"><label for="import-reference">参考答案</label><textarea id="import-reference" v-model="form.referenceText" rows="2" lang="ja" /></div>
              <div class="grid-2"><div class="field"><label for="import-authority">答案来源</label><select id="import-authority" v-model="form.authority"><option value="official">官方答案（原文）</option><option value="human_verified">人工审核答案</option></select></div><div class="field"><label for="import-explanation">来源解析</label><textarea id="import-explanation" v-model="form.explanation" rows="2" /></div></div>
            </template>
            <p class="muted" style="font-size: 13px">不确认标准答案时，本题发布后会走 AI 判定，不计入正式正确率。</p>
          </fieldset>

          <div class="import-actions"><button class="primary" type="submit" :disabled="saving || item.reviewStatus === 'published'">{{ saving ? '保存中…' : '保存草稿' }}</button><button v-if="item.reviewStatus === 'pending'" type="button" :disabled="saving || approving" @click="approve">{{ approving ? '审核中…' : '确认审核' }}</button><button v-if="item.reviewStatus === 'approved'" type="button" class="primary" :disabled="publishing" @click="askPublish">发布题目</button><RouterLink class="tag" :to="`/admin/imports/${item.jobId}`">返回导入任务</RouterLink></div>
        </form>
      </div>
      <ConfirmDialog :open="confirmPublish" title="确认发布题目？" confirm-label="确认发布" cancel-label="再检查一下" @confirm="publish" @cancel="confirmPublish = false"><p>发布后题目会进入公共题库；AI 建议不会自动变成标准答案。</p></ConfirmDialog>
    </template>
  </AppShell>
</template>
