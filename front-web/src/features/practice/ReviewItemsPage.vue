<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { request, ApiError } from '@/api/client'
import type { ReviewItem } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import AppStatus from '@/components/AppStatus.vue'
import FuriganaText from '@/components/FuriganaText.vue'
import { formatDateTime } from '@/app/format'

const route = useRoute()
const router = useRouter()
const items = ref<ReviewItem[]>([])
const state = ref<'loading' | 'ready' | 'error'>('loading')
const errorMessage = ref('')
const requestID = ref('')
const updatingQuestionID = ref('')

const mastered = computed(() => route.path === '/mastered-items')
const apiState = computed(() => mastered.value ? 'mastered' : 'due')
const title = computed(() => mastered.value ? '已掌握题目' : '待复习题目')

const lastStatusText: Record<string, string> = {
  correct: '上次答对',
  incorrect: '上次答错',
  unanswered: '上次未作答',
}

async function load(): Promise<void> {
  state.value = 'loading'
  try {
    const res = await request<{ reviewItems: ReviewItem[] }>(`/review-items?state=${apiState.value}&limit=100`)
    items.value = res.reviewItems
    state.value = 'ready'
  } catch (err) {
    errorMessage.value = err instanceof ApiError ? err.message : '加载失败'
    requestID.value = err instanceof ApiError ? err.requestId ?? '' : ''
    state.value = 'error'
  }
}

onMounted(() => void load())
watch(apiState, () => void load())

async function markMastered(item: ReviewItem): Promise<void> {
  if (updatingQuestionID.value) return
  updatingQuestionID.value = item.questionId
  try {
    await request(`/review-items/${item.questionId}/mastered`, { method: 'POST' })
    items.value = items.value.filter((current) => current.questionId !== item.questionId)
  } catch (err) {
    errorMessage.value = err instanceof ApiError ? err.message : '标记失败，请重试'
  } finally {
    updatingQuestionID.value = ''
  }
}

async function removeMastered(item: ReviewItem): Promise<void> {
  if (updatingQuestionID.value) return
  if (!window.confirm('确定移除这道题的已掌握标记吗？它会立即回到待复习列表。')) return
  updatingQuestionID.value = item.questionId
  try {
    await request(`/review-items/${item.questionId}/mastered`, { method: 'DELETE' })
    items.value = items.value.filter((current) => current.questionId !== item.questionId)
  } catch (err) {
    errorMessage.value = err instanceof ApiError ? err.message : '移除失败，请重试'
  } finally {
    updatingQuestionID.value = ''
  }
}
</script>

<template>
  <AppShell>
    <div class="page-header">
      <div>
        <h1 style="font-size: 24px; margin: 0">{{ title }}</h1>
        <p class="muted" style="margin-bottom: 0">
          {{ mastered ? '这些题已标记为掌握；需要重新练习时可以移除标记。' : '按到期时间排列，复习后可直接标记为已掌握。' }}
        </p>
      </div>
      <div style="display: flex; gap: 8px; flex-wrap: wrap">
        <button v-if="!mastered" class="primary" type="button" @click="router.push('/practice/new?mode=review')">开始复习</button>
        <button v-else class="primary" type="button" @click="router.push('/review-items')">查看待复习</button>
      </div>
    </div>

    <AppStatus v-if="state === 'loading'" state="loading" />
    <AppStatus v-else-if="state === 'error'" state="error" :message="errorMessage" :request-id="requestID" @action="load" />
    <AppStatus
      v-else-if="items.length === 0"
      state="empty"
      :message="mastered ? '还没有已掌握的题目。' : '当前没有到期的待复习题目。'"
      action-label="返回学习概览"
      @action="router.push('/')"
    />
    <div v-else style="display: flex; flex-direction: column; gap: 18px">
      <article v-for="item in items" :key="item.questionId" class="card" lang="ja">
        <header style="display: flex; flex-wrap: wrap; gap: 8px; justify-content: space-between; align-items: center">
          <div style="display: flex; gap: 8px; flex-wrap: wrap; align-items: center">
            <span class="mono muted">第 {{ item.position }} 题</span>
            <span class="tag" :data-tone="item.lastStatus === 'correct' ? 'success' : 'warning'">
              {{ lastStatusText[item.lastStatus] ?? item.lastStatus }}
            </span>
            <span v-if="item.gradingSource === 'ai'" class="tag" data-tone="accent">AI 判定（可能有误）</span>
          </div>
          <span class="muted mono">
            {{ mastered ? `标记于 ${formatDateTime(item.masteredAt ?? '')}` : `到期于 ${formatDateTime(item.nextReviewAt)}` }}
          </span>
        </header>
        <section v-if="item.material" class="card" style="background: var(--fg-soft); padding: 14px; margin-top: 10px">
          <p class="material-text" style="margin: 0; white-space: pre-wrap"><FuriganaText :text="item.material.content" /></p>
        </section>
        <p style="font-size: 16px; margin: 12px 0 8px; white-space: pre-wrap"><FuriganaText :text="item.stem" /></p>
        <div v-if="item.knowledgePoints.length" style="display: flex; gap: 8px; flex-wrap: wrap">
          <span v-for="point in item.knowledgePoints" :key="point.id" class="tag">{{ point.name }}</span>
        </div>
        <div style="display: flex; justify-content: flex-end; margin-top: 14px">
          <button
            v-if="mastered"
            class="ghost danger"
            type="button"
            :disabled="updatingQuestionID === item.questionId"
            @click="removeMastered(item)"
          >
            {{ updatingQuestionID === item.questionId ? '移除中…' : '移除已掌握' }}
          </button>
          <button
            v-else
            class="ghost"
            type="button"
            :disabled="updatingQuestionID === item.questionId"
            @click="markMastered(item)"
          >
            {{ updatingQuestionID === item.questionId ? '标记中…' : '已掌握' }}
          </button>
        </div>
      </article>
    </div>
  </AppShell>
</template>
