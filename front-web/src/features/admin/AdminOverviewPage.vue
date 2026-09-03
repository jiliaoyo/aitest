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
      <div class="card" style="margin-top: 18px">
        <p class="muted" style="margin-top: 0">快捷入口</p>
        <p style="display: flex; gap: 10px; flex-wrap: wrap">
          <RouterLink class="tag" to="/admin/questions/new">新建题目</RouterLink>
          <RouterLink class="tag" to="/admin/questions">题目列表</RouterLink>
          <RouterLink class="tag" to="/admin/knowledge">知识点管理</RouterLink>
          <RouterLink class="tag" to="/admin/sources">来源管理</RouterLink>
          <RouterLink class="tag" to="/admin/issues">举报处理</RouterLink>
        </p>
      </div>
    </template>
  </AppShell>
</template>
