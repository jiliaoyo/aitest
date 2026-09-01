<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { request, ApiError } from '@/api/client'
import type { KnowledgePointDetail } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import AppStatus from '@/components/AppStatus.vue'
import { formatPercent, formatDateTime } from '@/app/format'

const route = useRoute()
const router = useRouter()

const detail = ref<KnowledgePointDetail | null>(null)
const state = ref<'loading' | 'ready' | 'error' | 'notfound'>('loading')
const errorMessage = ref('')
const requestID = ref('')
const creating = ref(false)

async function load(): Promise<void> {
  state.value = 'loading'
  try {
    detail.value = await request<KnowledgePointDetail>(`/knowledge-points/${route.params.knowledgePointId as string}`)
    state.value = 'ready'
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      state.value = 'notfound'
      return
    }
    errorMessage.value = err instanceof ApiError ? err.message : '加载失败'
    requestID.value = err instanceof ApiError ? err.requestId ?? '' : ''
    state.value = 'error'
  }
}

onMounted(load)

const accuracy = computed(() => {
  const s = detail.value?.stats
  if (!s || s.confirmedAnswered === 0) return null
  return s.confirmedCorrect / s.confirmedAnswered
})

async function startPractice(): Promise<void> {
  if (!detail.value) return
  creating.value = true
  try {
    const session = await request<{ id: string }>('/practice-sessions', {
      method: 'POST',
      body: {
        levelId: detail.value.levelId,
        subjectId: detail.value.subjectId,
        mode: 'knowledge',
        knowledgePointIds: [detail.value.id],
        count: 10,
      },
    })
    await router.push(`/practice/${session.id}`)
  } catch (err) {
    errorMessage.value = err instanceof ApiError ? err.message : '创建练习失败'
  } finally {
    creating.value = false
  }
}
</script>

<template>
  <AppShell>
    <AppStatus v-if="state === 'loading'" state="loading" />
    <AppStatus v-else-if="state === 'error'" state="error" :message="errorMessage" :request-id="requestID" @action="load" />
    <AppStatus v-else-if="state === 'notfound' || !detail" state="empty" message="知识点不存在或尚未发布。" action-label="返回知识点列表" @action="router.push('/knowledge')" />
    <template v-else>
      <div class="page-header">
        <div>
          <h1 style="font-size: 24px; margin: 0">{{ detail.name }}</h1>
          <p class="muted">{{ detail.levelCode }} · {{ detail.subjectName }} · 相关题目 {{ detail.questionCount }} 题</p>
        </div>
        <button class="primary" :disabled="creating || detail.questionCount === 0" @click="startPractice">
          {{ creating ? '创建中…' : '专项练习 10 题' }}
        </button>
      </div>

      <div v-if="detail.questionCount === 0" class="card">
        <p class="muted">该知识点还没有已发布题目，先去其他知识点练习吧。</p>
      </div>

      <div v-if="detail.description || detail.commonMistakes || detail.examples" class="card">
        <section v-if="detail.description">
          <h2 style="font-size: 16px">说明</h2>
          <p style="white-space: pre-wrap">{{ detail.description }}</p>
        </section>
        <section v-if="detail.commonMistakes" style="margin-top: 12px">
          <h2 style="font-size: 16px">常见误区</h2>
          <p style="white-space: pre-wrap">{{ detail.commonMistakes }}</p>
        </section>
        <section v-if="detail.examples" style="margin-top: 12px">
          <h2 style="font-size: 16px">例句</h2>
          <p class="material-text" lang="ja" style="white-space: pre-wrap; margin: 0">{{ detail.examples }}</p>
        </section>
      </div>
      <div v-else class="card">
        <p class="muted">该知识点正文尚未发布，先通过练习积累数据。</p>
      </div>

      <div class="metrics">
        <div class="metric">
          <p class="value">{{ detail.stats?.confirmedAnswered ?? 0 }}</p>
          <p class="label">已确认作答（官方/已审核答案）</p>
        </div>
        <div class="metric">
          <p class="value">{{ formatPercent(accuracy) }}</p>
          <p class="label">正式正确率</p>
        </div>
        <div class="metric">
          <p class="value">{{ detail.stats?.consecutiveWrong ?? 0 }}</p>
          <p class="label">连续错误次数</p>
        </div>
      </div>

      <p v-if="detail.stats" class="muted mono">
        近 30 天作答 {{ detail.stats.recentAnswered }} 题 · 最近练习
        {{ detail.stats.lastPracticedAt ? formatDateTime(detail.stats.lastPracticedAt) : '—' }}
        <template v-if="detail.stats.aiAnswered > 0">
          · AI 判定 {{ detail.stats.aiAnswered }} 题（正确 {{ detail.stats.aiCorrect }}，不计入正式正确率）
        </template>
      </p>
      <p v-else class="muted">还没有本知识点的练习数据。</p>
    </template>
  </AppShell>
</template>
