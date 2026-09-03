<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { request, ApiError } from '@/api/client'
import type { IssueReportDTO } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import AppStatus from '@/components/AppStatus.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { formatDateTime } from '@/app/format'

const reports = ref<IssueReportDTO[]>([])
const nextCursor = ref('')
const loadingMore = ref(false)
const state = ref<'loading' | 'ready' | 'error'>('loading')
const errorMessage = ref('')
const requestID = ref('')
const statusFilter = ref('open')
const notes = reactive(new Map<string, string>())
const targetTypeText: Record<string, string> = {
  stem: '题干',
  answer: '答案',
  explanation: '解析',
  classification: '分类',
  ai_grading: 'AI 判定',
}

async function load(append = false): Promise<void> {
  if (append) loadingMore.value = true
  else state.value = 'loading'
  try {
    const params = new URLSearchParams({ status: statusFilter.value, limit: '20' })
    if (append && nextCursor.value) params.set('cursor', nextCursor.value)
    const res = await request<{ issueReports: IssueReportDTO[]; nextCursor?: string }>(`/admin/issue-reports?${params}`)
    reports.value = append ? [...reports.value, ...res.issueReports] : res.issueReports
    nextCursor.value = res.nextCursor ?? ''
    state.value = 'ready'
  } catch (err) {
    errorMessage.value = err instanceof ApiError ? err.message : '加载失败'
    requestID.value = err instanceof ApiError ? err.requestId ?? '' : ''
    state.value = 'error'
  } finally {
    loadingMore.value = false
  }
}

onMounted(load)

async function handle(report: IssueReportDTO, status: 'resolved' | 'dismissed'): Promise<void> {
  try {
    await request(`/admin/issue-reports/${report.id}`, {
      method: 'PATCH',
      body: { status, resolutionNote: notes.get(report.id) ?? '' },
    })
    await load()
  } catch (err) {
    errorMessage.value = err instanceof ApiError ? err.message : '处理失败'
  }
}
</script>

<template>
  <AppShell>
    <div class="page-header">
      <h1 style="font-size: 24px; margin: 0">举报处理</h1>
      <label style="display: flex; align-items: center; gap: 8px">
        <span class="muted">状态</span>
        <select v-model="statusFilter" @change="() => load()">
          <option value="open">待处理</option>
          <option value="resolved">已解决</option>
          <option value="dismissed">已驳回</option>
        </select>
      </label>
    </div>

    <AppStatus v-if="state === 'loading'" state="loading" />
    <AppStatus v-else-if="state === 'error'" state="error" :message="errorMessage" :request-id="requestID" @action="load" />
    <AppStatus v-else-if="reports.length === 0" state="empty" message="没有待处理的举报。" />
    <div v-else style="display: flex; flex-direction: column; gap: 18px">
      <article v-for="r in reports" :key="r.id" class="card">
        <div class="page-header">
          <div>
            <p style="margin: 0">
              <span class="tag" data-tone="accent">{{ targetTypeText[r.targetType] ?? r.targetType }}</span>
              <StatusBadge :value="r.status" kind="issue" style="margin-left: 8px" />
            </p>
            <p class="muted mono" style="font-size: 13px; margin: 6px 0 0">
              题目 {{ r.questionId.slice(0, 8) }} · 举报人 {{ r.userEmail }} · {{ formatDateTime(r.createdAt) }}
            </p>
          </div>
        </div>
        <p class="material-text" lang="ja" style="margin-top: 10px; background: var(--fg-soft); border-radius: 9px; padding: 10px 14px">
          {{ r.stem }}
        </p>
        <p v-if="r.description" style="margin: 10px 0 0">{{ r.description }}</p>
        <div v-if="r.status === 'open'" style="margin-top: 12px">
          <div class="field">
            <label :for="`note-${r.id}`">处理备注</label>
            <input :id="`note-${r.id}`" :value="notes.get(r.id) ?? ''" type="text" @input="notes.set(r.id, ($event.target as HTMLInputElement).value)" />
          </div>
          <div style="display: flex; gap: 10px">
            <button class="primary" @click="handle(r, 'resolved')">标记已解决</button>
            <button class="danger" @click="handle(r, 'dismissed')">驳回</button>
          </div>
        </div>
        <p v-else-if="r.resolutionNote" class="muted" style="margin-top: 10px">处理备注：{{ r.resolutionNote }}</p>
      </article>
      <p v-if="nextCursor" style="margin: 0; text-align: center">
        <button :disabled="loadingMore" @click="load(true)">{{ loadingMore ? '加载中…' : '加载更多' }}</button>
      </p>
    </div>
  </AppShell>
</template>
