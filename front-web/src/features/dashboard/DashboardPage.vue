<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { request, ApiError } from '@/api/client'
import type { DashboardDTO } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import AppStatus from '@/components/AppStatus.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { formatDateTime, formatPercent } from '@/app/format'

const router = useRouter()
const dashboard = ref<DashboardDTO | null>(null)
const state = ref<'loading' | 'ready' | 'error'>('loading')
const errorBody = ref<ApiError | null>(null)

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

async function goPractice(rec: { knowledgePointIds: string[]; suggestedCount: number }): Promise<void> {
  const me = await request<{ user: { defaultLevelId: string | null } }>('/me')
  if (!me.user.defaultLevelId) {
    await router.push('/practice/new')
    return
  }
  try {
    const session = await request<{ id: string }>('/practice-sessions', {
      method: 'POST',
      body: {
        levelId: me.user.defaultLevelId,
        mode: 'knowledge',
        knowledgePointIds: rec.knowledgePointIds,
        count: rec.suggestedCount,
      },
    })
    await router.push(`/practice/${session.id}`)
  } catch (err) {
    // 题量不足等情况下进入创建页让用户确认
    await router.push('/practice/new')
    void err
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
          <p class="muted">基于你的真实作答统计生成，AI 只负责把数字转述成建议。</p>
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

      <section v-if="dashboard.memory" aria-labelledby="memory-title">
        <h2 id="memory-title" style="font-size: 17px">全局做题记忆</h2>
        <div class="card">
          <p class="mono">
            已确认作答 {{ dashboard.memory.confirmedAnswered }} 题，正确 {{ dashboard.memory.confirmedCorrect }} 题
          </p>
          <p v-if="dashboard.memory.aiAnswered > 0" class="muted">
            另有 AI 判定 {{ dashboard.memory.aiAnswered }} 题（不计入正式正确率）。
          </p>
          <div v-if="dashboard.memory.advice.status === 'completed' && dashboard.memory.advice.text">
            <p><strong>AI 学习建议</strong></p>
            <p class="muted" style="white-space: pre-wrap">{{ dashboard.memory.advice.text }}</p>
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
          <table class="data">
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
                <td class="mono">{{ s.id.slice(0, 8) }}</td>
                <td><StatusBadge :value="s.status" kind="session" /></td>
                <td class="num">{{ s.totalCount }}</td>
                <td class="mono">{{ formatDateTime(s.createdAt) }}</td>
                <td>
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
