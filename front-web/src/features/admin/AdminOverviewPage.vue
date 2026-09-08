<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { request, ApiError } from '@/api/client'
import type { OverviewDTO } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import AppStatus from '@/components/AppStatus.vue'

const overview = ref<OverviewDTO | null>(null)
const state = ref<'loading' | 'ready' | 'error'>('loading')
const errorMessage = ref('')
const requestID = ref('')

function percent(value: number | null): string {
  return value === null ? '数据不足' : `${value.toFixed(1)}%`
}

function questionFilter(row: OverviewDTO['coverage'][number]): { path: string; query: Record<string, string> } {
  return { path: '/admin/questions', query: { status: 'published', levelId: row.levelId, subjectId: row.subjectId } }
}

function knowledgeFilter(row: OverviewDTO['coverage'][number]): { path: string; query: Record<string, string> } {
  return { path: '/admin/knowledge', query: { levelId: row.levelId } }
}

async function load(): Promise<void> {
  state.value = 'loading'
  try {
    overview.value = await request<OverviewDTO>('/admin/overview')
    state.value = 'ready'
  } catch (err) {
    errorMessage.value = err instanceof ApiError ? err.message : '加载失败'
    requestID.value = err instanceof ApiError ? err.requestId ?? '' : ''
    state.value = 'error'
  }
}

onMounted(load)
</script>

<template>
  <AppShell>
    <h1 style="font-size: 24px">内容概览</h1>
    <AppStatus v-if="state === 'loading'" state="loading" />
    <AppStatus v-else-if="state === 'error'" state="error" :message="errorMessage" :request-id="requestID" @action="load" />
    <template v-else-if="overview">
      <div class="metrics">
        <div class="metric">
          <p class="value">{{ overview.draft }}</p>
          <p class="label">草稿</p>
        </div>
        <div class="metric">
          <p class="value">{{ overview.inReview }}</p>
          <p class="label">待审核</p>
        </div>
        <div class="metric">
          <p class="value">{{ overview.published }}</p>
          <p class="label">已发布</p>
        </div>
      </div>
      <div class="metrics" style="margin-top: 18px">
        <div class="metric">
          <p class="value">{{ overview.retired }}</p>
          <p class="label">已下架</p>
        </div>
        <div class="metric" :data-tone="overview.publishedNoAnswer > 0 ? 'warning' : undefined">
          <p class="value" :style="overview.publishedNoAnswer > 0 ? 'color: var(--warning)' : ''">{{ overview.publishedNoAnswer }}</p>
          <p class="label">已发布但无标准答案</p>
        </div>
        <div class="metric" :data-tone="overview.openIssues > 0 ? 'warning' : undefined">
          <p class="value" :style="overview.openIssues > 0 ? 'color: var(--warning)' : ''">{{ overview.openIssues }}</p>
          <p class="label">待处理举报</p>
        </div>
      </div>
      <div class="metrics" style="margin-top: 18px">
        <RouterLink class="metric metric-link" :to="{ path: '/admin/questions', query: { status: 'published', quality: 'no_knowledge' } }">
          <p class="value" :style="overview.publishedNoKnowledge > 0 ? 'color: var(--warning)' : ''">{{ overview.publishedNoKnowledge }}</p>
          <p class="label">无知识点</p>
        </RouterLink>
        <RouterLink class="metric metric-link" :to="{ path: '/admin/questions', query: { status: 'published', quality: 'no_source' } }">
          <p class="value" :style="overview.publishedNoSource > 0 ? 'color: var(--warning)' : ''">{{ overview.publishedNoSource }}</p>
          <p class="label">无来源</p>
        </RouterLink>
        <RouterLink class="metric metric-link" :to="{ path: '/admin/questions', query: { status: 'published', hasAnswer: 'no' } }">
          <p class="value" :style="overview.publishedNoAnswer > 0 ? 'color: var(--warning)' : ''">{{ overview.publishedNoAnswer }}</p>
          <p class="label">无答案</p>
        </RouterLink>
        <RouterLink class="metric metric-link" :to="{ path: '/admin/issues', query: { status: 'open' } }">
          <p class="value" :style="overview.openIssues > 0 ? 'color: var(--warning)' : ''">{{ overview.openIssues }}</p>
          <p class="label">开放举报</p>
        </RouterLink>
      </div>
      <section class="card" style="margin-top: 18px; overflow-x: auto">
        <h2 style="font-size: 18px; margin-top: 0">内容覆盖</h2>
        <p class="muted">只统计当前已发布、可进入普通练习的公共题；答案覆盖率的分母是已发布题数。</p>
        <table v-if="overview.coverage.length" class="data mobile-card-table">
          <thead><tr><th>级别 / 科目</th><th class="num">公共题</th><th class="num">权威答案</th><th class="num">答案覆盖率</th><th class="num">知识点 ≥1题</th><th class="num">知识点 ≥5题</th><th class="num">无题知识点</th><th class="num">待处理举报</th></tr></thead>
          <tbody>
            <tr v-for="row in overview.coverage" :key="`${row.levelId}-${row.subjectId}`">
              <td data-label="级别 / 科目">{{ row.levelName }} / {{ row.subjectName }}<span class="muted mono">（{{ row.levelCode }} / {{ row.subjectCode }}）</span></td>
              <td class="num" data-label="公共题"><RouterLink :to="questionFilter(row)">{{ row.publishedQuestions }}</RouterLink></td>
              <td class="num" data-label="权威答案"><RouterLink :to="questionFilter(row)">{{ row.authorityAnsweredQuestions }}</RouterLink></td>
              <td class="num" data-label="答案覆盖率">{{ percent(row.authorityAnswerRate) }}</td>
              <td class="num" data-label="知识点 ≥1题"><RouterLink :to="knowledgeFilter(row)">{{ row.knowledgePointsWithQuestion }}</RouterLink></td>
              <td class="num" data-label="知识点 ≥5题"><RouterLink :to="knowledgeFilter(row)">{{ row.knowledgePointsWithFiveQuestions }}</RouterLink></td>
              <td class="num" data-label="无题知识点"><RouterLink :to="knowledgeFilter(row)">{{ row.knowledgePointsWithoutQuestions }}</RouterLink></td>
              <td class="num" data-label="待处理举报"><RouterLink to="/admin/issues?status=open">{{ row.openIssues }}</RouterLink></td>
            </tr>
          </tbody>
        </table>
        <p v-else class="muted">暂无级别和科目数据。</p>
      </section>
      <section class="card" style="margin-top: 18px">
        <h2 style="font-size: 18px; margin-top: 0">学习闭环指标</h2>
        <p class="muted">按现有全部历史事实计算；7 日再练只纳入首次提交已满 7 天的用户，数据不足时不展示比例。</p>
        <div class="metrics">
          <div class="metric"><p class="value">{{ percent(overview.learningMetrics.ordinarySubmissionRate) }}</p><p class="label">普通练习提交率（{{ overview.learningMetrics.ordinarySessionsSubmitted }}/{{ overview.learningMetrics.ordinarySessionsStarted }}）</p></div>
          <div class="metric"><p class="value">{{ percent(overview.learningMetrics.sevenDayRepracticeRate) }}</p><p class="label">首次提交用户 7 日再练（{{ overview.learningMetrics.firstSubmitUsersReturned }}/{{ overview.learningMetrics.firstSubmitUsersObserved }}）</p></div>
          <div class="metric" :data-tone="overview.learningMetrics.aiGenerationFailed > 0 ? 'warning' : undefined"><p class="value">{{ overview.learningMetrics.aiGenerationFailed }}</p><p class="label">AI 出题失败批次</p></div>
        </div>
      </section>
      <div class="card" style="margin-top: 18px">
        <p class="muted" style="margin-top: 0">快捷入口</p>
        <p style="display: flex; gap: 10px; flex-wrap: wrap">
          <RouterLink class="tag" to="/admin/questions/new">新建题目</RouterLink>
          <RouterLink class="tag" to="/admin/users">用户与用量</RouterLink>
          <RouterLink class="tag" to="/admin/questions">题目列表</RouterLink>
          <RouterLink class="tag" to="/admin/knowledge">知识点管理</RouterLink>
          <RouterLink class="tag" to="/admin/sources">来源管理</RouterLink>
          <RouterLink class="tag" to="/admin/issues">举报处理</RouterLink>
        </p>
      </div>
    </template>
  </AppShell>
</template>
