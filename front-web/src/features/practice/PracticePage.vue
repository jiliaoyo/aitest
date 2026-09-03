<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { request, ApiError } from '@/api/client'
import type { AnswerValue, PreSubmitItem, PreSubmitSession } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import AppStatus from '@/components/AppStatus.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import QuestionCard from './QuestionCard.vue'
import QuestionNavigator from './QuestionNavigator.vue'
import SaveStatus from './SaveStatus.vue'
import { useAnswerAutosave } from './useAnswerAutosave'

const route = useRoute()
const router = useRouter()
const sessionID = computed(() => route.params.sessionId as string)

const session = ref<PreSubmitSession | null>(null)
const pageState = ref<'loading' | 'ready' | 'error' | 'notfound'>('loading')
const errorMessage = ref('')
const requestID = ref('')

const currentIndex = ref(0)
const collapsedMaterials = ref(new Set<string>())
const showLocalDraftNote = ref(false)
const confirmOpen = ref(false)
const submitting = ref(false)
const submitError = ref('')

const autosave = useAnswerAutosave(sessionID)
const currentEntry = computed(() => autosave.entryOf(currentItem.value?.id ?? ''))
let generationTimer: ReturnType<typeof setInterval> | null = null

const currentItem = computed<PreSubmitItem | null>(() => {
  const item = session.value?.items[currentIndex.value]
  return item ?? null
})

const answeredCount = computed(
  () => session.value?.items.filter((i) => autosave.entries.get(i.id)?.value != null).length ?? 0,
)
const markedCount = computed(
  () => session.value?.items.filter((i) => autosave.entries.get(i.id)?.marked).length ?? 0,
)
const unansweredCount = computed(() => (session.value?.totalCount ?? 0) - answeredCount.value)
const progressPercent = computed(() =>
  session.value ? (answeredCount.value / session.value.totalCount) : 0,
)

function isAnswered(item: PreSubmitItem): boolean {
  return autosave.entries.get(item.id)?.value != null
}
function isMarked(item: PreSubmitItem): boolean {
  return autosave.entries.get(item.id)?.marked ?? false
}

onMounted(load)

async function load(silent = false): Promise<void> {
  if (!silent) pageState.value = 'loading'
  try {
    const data = await request<PreSubmitSession>(`/practice-sessions/${sessionID.value}`)
    session.value = data
    showLocalDraftNote.value = autosave.init(data.items)
    pageState.value = 'ready'
    scheduleGenerationPolling()
  } catch (err) {
    if (silent) return
    if (err instanceof ApiError && err.status === 404) {
      pageState.value = 'notfound'
      return
    }
    errorMessage.value = err instanceof ApiError ? err.message : '加载失败'
    requestID.value = err instanceof ApiError ? err.requestId ?? '' : ''
    pageState.value = 'error'
  }
}

function scheduleGenerationPolling(): void {
  if (session.value?.status === 'generating' && generationTimer === null) {
    generationTimer = setInterval(() => void load(true), 2000)
  }
  if (session.value?.status !== 'generating' && generationTimer !== null) {
    clearInterval(generationTimer)
    generationTimer = null
  }
}

function setAnswer(value: AnswerValue): void {
  if (!currentItem.value) return
  autosave.setAnswer(currentItem.value.id, value)
}

function setMarked(marked: boolean): void {
  if (!currentItem.value) return
  autosave.setMarked(currentItem.value.id, marked)
}

function toggleMaterial(): void {
  const material = currentItem.value?.material
  if (!material) return
  const set = collapsedMaterials.value
  if (set.has(material.id)) {
    set.delete(material.id)
  } else {
    set.add(material.id)
  }
}

function materialCollapsed(): boolean {
  const material = currentItem.value?.material
  return material ? collapsedMaterials.value.has(material.id) : false
}

// ---- 批次提交 ----

function submitKey(): string {
  const storageKey = `practice-submit-key:${sessionID.value}`
  let key = localStorage.getItem(storageKey)
  if (!key) {
    key = crypto.randomUUID()
    localStorage.setItem(storageKey, key)
  }
  return key
}

function clearSubmitKey(): void {
  localStorage.removeItem(`practice-submit-key:${sessionID.value}`)
}

async function openConfirm(): Promise<void> {
  // 停止新的 debounce 并尽力冲刷未保存修改；提交请求会携带全部最终答案
  await autosave.flushPending(session.value?.items ?? [])
  submitError.value = ''
  confirmOpen.value = true
}

async function doSubmit(): Promise<void> {
  if (!session.value || submitting.value) return
  submitting.value = true
  submitError.value = ''
  try {
    const result = await request<unknown>(`/practice-sessions/${sessionID.value}/submit`, {
      method: 'POST',
      body: { answers: autosave.finalAnswers(session.value.items) },
      headers: { 'Idempotency-Key': submitKey() },
    })
    void result
    autosave.cleanup()
    clearSubmitKey()
    await router.replace(`/practice/${sessionID.value}/result`)
  } catch (err) {
    if (err instanceof ApiError && (err.code === 'practice_not_active' || err.code === 'idempotency_conflict')) {
      // 批次已在其他会话提交：直接进入现有结果，不产生第二份成绩
      autosave.cleanup()
      clearSubmitKey()
      await router.replace(`/practice/${sessionID.value}/result`)
      return
    }
    submitError.value = err instanceof ApiError ? `${err.message}（本批答案已保留，可重试）` : '提交失败，请重试'
    confirmOpen.value = false
  } finally {
    submitting.value = false
  }
}

onBeforeUnmount(() => {
  if (generationTimer) clearInterval(generationTimer)
  for (const entry of autosave.entries.values()) {
    if (entry.timer) clearTimeout(entry.timer)
  }
})
</script>

<template>
  <AppShell>
    <AppStatus v-if="pageState === 'loading'" state="loading" />
    <AppStatus v-else-if="pageState === 'error'" state="error" :message="errorMessage" :request-id="requestID" @action="load" />
    <AppStatus v-else-if="pageState === 'notfound'" state="empty" message="练习不存在，或刷新后已被提交。" action-label="返回学习概览" @action="router.push('/')" />
    <template v-else-if="session">
      <template v-if="session.status === 'generating'">
        <div class="card" role="status">
          <h2>AI 正在生成个性化题目</h2>
          <p class="muted">正在根据你的全局做题记忆和薄弱知识点随机出题，请稍候。</p>
          <button type="button" @click="load()">刷新生成状态</button>
        </div>
      </template>
      <template v-else-if="session.status === 'generation_failed'">
        <div class="card">
          <h2>AI 题目生成失败</h2>
          <p class="muted">本次生成没有写入公共题库，请返回重新开始。</p>
          <button class="primary" @click="router.push('/practice/new')">重新开始</button>
        </div>
      </template>
      <template v-else-if="session.status !== 'active'">
        <div class="card">
          <h2>本批练习已提交</h2>
          <p class="muted">答案已锁定，去看看结果与解析。</p>
          <button class="primary" @click="router.push(`/practice/${sessionID}/result`)">查看结果</button>
        </div>
      </template>
      <template v-else>
        <div class="page-header" style="align-items: center">
          <h1 style="font-size: 24px; margin: 0">练习进行中</h1>
          <SaveStatus
            :state="currentEntry.state"
            :saved-at="currentEntry.savedAt"
            :local-only="currentEntry.localOnly"
            :any-error="autosave.anyError.value"
            @retry="autosave.retryFailed"
          />
        </div>

        <p v-if="showLocalDraftNote" class="error-summary" role="status">
          检测到上次未同步到服务器的答案，已按本地最新内容恢复；保存成功后会自动清除。
        </p>

        <div class="progress-bar" role="img" :aria-label="`已答 ${answeredCount} / ${session.totalCount} 题`">
          <span :style="{ width: `${Math.round(progressPercent * 100)}%` }" />
        </div>
        <p class="mono muted" style="margin: 6px 0 0">
          进度 {{ answeredCount }} / {{ session.totalCount }} · 标记 {{ markedCount }}
        </p>

        <div class="practice-layout" style="margin-top: 18px">
          <div>
            <QuestionCard
              v-if="currentItem"
              :item="currentItem"
              :answer="currentEntry.value"
              :marked="currentEntry.marked"
              :material-collapsed="materialCollapsed()"
              @update:answer="setAnswer"
              @update:marked="setMarked"
              @toggle-material="toggleMaterial"
            />
            <div style="display: flex; justify-content: space-between; margin-top: 14px; gap: 10px">
              <button type="button" :disabled="currentIndex === 0" @click="currentIndex--">上一题</button>
              <button
                v-if="currentIndex < session.items.length - 1"
                type="button"
                class="primary"
                @click="currentIndex++"
              >
                下一题
              </button>
            </div>
          </div>

          <aside class="practice-side">
            <div class="card">
              <h2 style="font-size: 15px">题目导航</h2>
              <QuestionNavigator
                :items="session.items"
                :current-index="currentIndex"
                :is-answered="isAnswered"
                :is-marked="isMarked"
                @select="currentIndex = $event"
              />
            </div>
            <div class="card">
              <p class="muted" style="margin-top: 0">
                还有 {{ unansweredCount }} 题未作答。答题过程中不显示对错与解析，本批提交后统一判分。
              </p>
              <button class="primary" style="width: 100%" @click="openConfirm">提交本批练习</button>
            </div>
          </aside>
        </div>

        <ConfirmDialog
          :open="confirmOpen"
          title="确认提交本批练习？"
          confirm-label="确认提交"
          cancel-label="继续答题"
          @confirm="doSubmit"
          @cancel="confirmOpen = false"
        >
          <p v-if="unansweredCount > 0">
            还有 <strong>{{ unansweredCount }}</strong> 题未作答{{ markedCount > 0 ? `、${markedCount} 题已标记待检查` : '' }}。
            提交后答案不可修改。
          </p>
          <p v-else>全部题目已作答，提交后答案不可修改。</p>
          <p v-if="submitError" class="error-summary" role="alert">{{ submitError }}</p>
        </ConfirmDialog>
      </template>
    </template>
  </AppShell>
</template>
