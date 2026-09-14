<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { request, ApiError } from '@/api/client'
import type { DashboardDTO } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import AppStatus from '@/components/AppStatus.vue'
import FuriganaText from '@/components/FuriganaText.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { formatAIText, formatDateTime, formatPercent } from '@/app/format'
import { sessionUser } from '@/app/session'

const router = useRouter()
const dashboard = ref<DashboardDTO | null>(null)
const state = ref<'loading' | 'ready' | 'error'>('loading')
const errorBody = ref<ApiError | null>(null)
const reviewCreating = ref(false)
const reviewError = ref('')

async function load(): Promise<void> {
  state.value = 'loading'
  try {
    dashboard.value = await request<DashboardDTO>('/dashboard')
    state.value = 'ready'
  } catch (err) {
    errorBody.value = err instanceof ApiError ? err : null
    state.value = 'error'
  }
}

onMounted(load)

async function goPractice(rec: { knowledgePointIds: string[]; suggestedCount: number; levelId?: string; subjectId?: string }): Promise<void> {
  if (!rec.levelId) {
    await router.push('/practice/new')
    return
  }
  try {
    const session = await request<{ id: string }>('/practice-sessions', {
      method: 'POST',
      body: {
        levelId: rec.levelId,
        subjectId: rec.subjectId,
        mode: 'knowledge',
        knowledgePointIds: rec.knowledgePointIds,
        count: rec.suggestedCount,
      },
    })
    await router.push(`/practice/${session.id}`)
  } catch (err) {
    // 题量不足等情况下进入创建页让用户确认
    const params = new URLSearchParams({
      levelId: rec.levelId ?? '',
      subjectId: rec.subjectId ?? '',
      mode: 'knowledge',
      knowledgePointIds: rec.knowledgePointIds.join(','),
      recommendedCount: String(rec.suggestedCount),
    })
    await router.push(`/practice/new?${params}`)
    void err
  }
}

async function goReview(): Promise<void> {
  if (!dashboard.value || reviewCreating.value) return
  const levelId = sessionUser()?.defaultLevelId
  if (!levelId) {
    await router.push('/practice/new?mode=review')
    return
  }
  reviewCreating.value = true
  reviewError.value = ''
  try {
    const session = await request<{ id: string }>('/practice-sessions', {
      method: 'POST',
      body: {
        levelId,
        mode: 'review',
        count: Math.min(dashboard.value.reviewDueCount, 10),
      },
    })
    await router.push(`/practice/${session.id}`)
  } catch (err) {
    if (err instanceof ApiError && (err.code === 'no_due_reviews' || err.code === 'insufficient_questions')) {
      await router.push(`/practice/new?mode=review&levelId=${encodeURIComponent(levelId)}`)
      return
    }
    reviewError.value = err instanceof ApiError ? err.message : '创建复习练习失败，请重试'
  } finally {
    reviewCreating.value = false
  }
}
</script>

<template>
  <AppShell>
    <AppStatus v-if="state === 'loading'" state="loading" />
    <AppStatus
      v-else-if="state === 'error'"
      state="error"
      :message="errorBody?.message"
      :request-id="errorBody?.requestId"
      @action="load"
    />
    <template v-else-if="dashboard">
      <div class="page-header">
        <div>
          <h1>学习概览</h1>
          <p class="muted">基于你的全部真实作答生成，权威结果与 AI 来源结果会分层展示。</p>
        </div>
        <RouterLink class="primary" to="/practice/new" custom v-slot="{ navigate }">
          <button class="primary" @click="navigate">开始新练习</button>
        </RouterLink>
      </div>

      <div v-if="dashboard.activeSession" class="card" data-tone="accent">
        <div class="page-header">
          <div>
            <h2 style="font-size: 17px">
              {{ dashboard.activeSession.status === 'generating' ? 'AI 正在生成个性化题目' : '有一批未完成的练习' }}
            </h2>
            <p class="muted mono">
              <template v-if="dashboard.activeSession.status === 'generating'">正在根据全局做题记忆准备题目</template>
              <template v-else>已答 {{ dashboard.activeSession.answeredCount }} / {{ dashboard.activeSession.totalCount }} 题</template>
            </p>
          </div>
          <button class="primary" @click="router.push(`/practice/${dashboard.activeSession!.id}`)">
            {{ dashboard.activeSession.status === 'generating' ? '查看生成进度' : '继续答题' }}
          </button>
        </div>
      </div>

      <section aria-labelledby="rec-title">
        <h2 id="rec-title" style="font-size: 17px">今日建议</h2>
        <div v-if="dashboard.recommendations.length === 0 && dashboard.comprehensive" class="card">
          <p><strong>{{ dashboard.comprehensive.name }}</strong></p>
          <p class="muted">{{ dashboard.comprehensive.reason }}</p>
          <button class="primary" @click="router.push('/practice/new')">创建综合练习</button>
        </div>
        <div v-for="rec in dashboard.recommendations" :key="rec.knowledgePointId ?? rec.name" class="card" style="margin-top: 18px">
          <div class="page-header">
            <div>
              <p>
                <strong>{{ rec.name }}</strong>
                <span class="muted mono" style="margin-left: 8px">
                  近 30 天正确率 {{ formatPercent(rec.accuracy) }} · 作答 {{ rec.recentAnswered }} 题 · 连续错误
                  {{ rec.consecutiveWrong }}
                </span>
              </p>
              <p class="muted">{{ rec.reason }}</p>
            </div>
            <button class="primary" @click="goPractice(rec)">练习 {{ rec.suggestedCount }} 题</button>
          </div>
        </div>
      </section>

      <section v-if="dashboard.reviewDueCount > 0" aria-labelledby="review-title">
        <h2 id="review-title" style="font-size: 17px">到期复习</h2>
        <div class="card">
          <p class="muted">有 {{ dashboard.reviewDueCount }} 道题到期，包含题库题和你做过的 AI 生成题。</p>
          <button class="primary" :disabled="reviewCreating" @click="goReview">
            {{ reviewCreating ? '准备复习…' : `复习 ${Math.min(dashboard.reviewDueCount, 10)} 题` }}
          </button>
          <p v-if="reviewError" class="error-summary" role="alert">{{ reviewError }}</p>
        </div>
      </section>

      <section v-if="dashboard.memory" aria-labelledby="memory-title">
        <h2 id="memory-title" style="font-size: 17px">全局做题记忆</h2>
        <div class="card">
          <p class="mono">
            累计纳入学习记忆 {{ dashboard.memory.confirmedAnswered + dashboard.memory.aiAnswered }} 题
          </p>
          <p class="muted">
            其中权威或人工审核结果 {{ dashboard.memory.confirmedAnswered }} 题，正确 {{ dashboard.memory.confirmedCorrect }} 题。
          </p>
          <p v-if="dashboard.memory.aiAnswered > 0" class="muted">
            AI 来源结果 {{ dashboard.memory.aiAnswered }} 题，正确 {{ dashboard.memory.aiCorrect }} 题（可能有误，不计入正式正确率）。
          </p>
          <p v-if="dashboard.memory.aiAnswered > 0 && dashboard.memory.estimatedAccuracy != null" class="muted">
            含 AI 判定的估算正确率 {{ formatPercent(dashboard.memory.estimatedAccuracy) }}（可能有误）
          </p>
          <div v-if="dashboard.memory.advice.status === 'completed' && dashboard.memory.advice.text">
            <p><strong>AI 学习建议（基于累计进度）</strong></p>
            <p class="muted ai-text"><FuriganaText :text="formatAIText(dashboard.memory.advice.text)" /></p>
          </div>
          <p v-else-if="dashboard.memory.advice.status === 'pending'" class="muted" role="status">
            AI 正在根据最新进度整理建议。
          </p>
          <p v-else-if="dashboard.memory.advice.status === 'failed'" class="muted">
            AI 建议暂时不可用，统计和专项推荐仍可正常使用。
          </p>
          <p v-else class="muted">完成一批练习后，AI 会结合你的累计进度给出建议。</p>
        </div>
      </section>

      <section aria-labelledby="recent-title">
        <h2 id="recent-title" style="font-size: 17px">最近练习</h2>
        <div v-if="dashboard.recentSessions.length === 0" class="card">
          <p class="muted">还没有练习记录。完成第一批练习后，这里会展示成绩与状态。</p>
          <button class="primary" @click="router.push('/practice/new')">创建练习</button>
        </div>
        <div v-else class="card" style="overflow-x: auto">
          <table class="data mobile-card-table">
            <thead>
              <tr>
                <th>批次</th>
                <th>状态</th>
                <th class="num">题数</th>
                <th>创建时间</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="s in dashboard.recentSessions" :key="s.id">
                <td class="mono" data-label="批次">{{ s.id.slice(0, 8) }}</td>
                <td data-label="状态"><StatusBadge :value="s.status" kind="session" /></td>
                <td class="num" data-label="题数">{{ s.totalCount }}</td>
                <td class="mono" data-label="创建时间">{{ formatDateTime(s.createdAt) }}</td>
                <td data-label="">
                  <RouterLink :to="`/practice/${s.id}/result`">查看结果</RouterLink>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>
  </AppShell>
</template>
