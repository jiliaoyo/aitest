<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { request, ApiError } from '@/api/client'
import type { ImportItemDTO, ImportJobDTO } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import AppStatus from '@/components/AppStatus.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { formatDateTime } from '@/app/format'

const route = useRoute()
const router = useRouter()
const jobID = route.params.importJobId as string
const job = ref<ImportJobDTO | null>(null)
const items = ref<ImportItemDTO[]>([])
const nextCursor = ref('')
const loadingMore = ref(false)
const state = ref<'loading' | 'ready' | 'error'>('loading')
const errorMessage = ref('')
const requestID = ref('')

async function load(initial = true, append = false): Promise<void> {
  if (initial) state.value = 'loading'
  if (append) loadingMore.value = true
  try {
    const params = new URLSearchParams({ limit: '20' })
    if (append && nextCursor.value) params.set('cursor', nextCursor.value)
    const res = await request<{ job: ImportJobDTO; items: ImportItemDTO[]; nextCursor?: string }>(`/admin/import-jobs/${jobID}?${params}`)
    job.value = res.job
    if (append) items.value = [...items.value, ...res.items]
    else if (initial || items.value.length === 0) {
      items.value = res.items
      nextCursor.value = res.nextCursor ?? ''
    }
    state.value = 'ready'
  } catch (err) {
    errorMessage.value = err instanceof ApiError ? err.message : '加载失败'
    requestID.value = err instanceof ApiError ? err.requestId ?? '' : ''
    state.value = 'error'
  } finally {
    loadingMore.value = false
  }
}

function loadMore(): void {
  void load(false, true)
}

onMounted(() => void load())
</script>

<template>
  <AppShell>
    <AppStatus v-if="state === 'loading'" state="loading" />
    <AppStatus v-else-if="state === 'error'" state="error" :message="errorMessage" :request-id="requestID" @action="load" />
    <template v-else-if="job">
      <div class="page-header">
        <div>
          <p class="muted" style="margin: 0 0 4px"><RouterLink to="/admin/imports">导入任务</RouterLink> / 详情</p>
          <h1 style="font-size: 24px; margin: 0">{{ job.fileName }}</h1>
        </div>
        <StatusBadge :value="job.status" />
      </div>
      <p v-if="job.stageError" class="error-summary" role="alert">{{ job.stageError }}</p>
      <div class="card" style="margin-bottom: 18px">
        <p style="margin: 0 0 8px"><strong>处理状态：</strong><StatusBadge :value="job.status" /> <span class="muted">{{ job.itemCount }} 个导入项 · 更新于 {{ formatDateTime(job.updatedAt) }}</span></p>
      </div>

      <div v-if="items.length" class="card" style="overflow-x: auto">
        <h2 style="font-size: 18px">结构化草稿</h2>
        <table class="data">
          <thead><tr><th class="num">#</th><th>题干</th><th>异常</th><th>审核状态</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="item in items" :key="item.id">
              <td class="num">{{ item.position }}</td>
              <td lang="ja">{{ item.draft?.stem ?? '—' }}</td>
              <td><span v-if="item.anomalies.length" class="tag" data-tone="warning">{{ item.anomalies.length }} 项待确认</span><span v-else class="muted">—</span></td>
              <td><StatusBadge :value="item.reviewStatus" /></td>
              <td><RouterLink :to="`/admin/import-items/${item.id}`">对照审核</RouterLink></td>
            </tr>
          </tbody>
        </table>
        <p v-if="nextCursor" style="margin: 12px 0 0; text-align: center">
          <button :disabled="loadingMore" @click="loadMore">{{ loadingMore ? '加载中…' : '加载更多' }}</button>
        </p>
      </div>
      <p v-else-if="job.status === 'review_ready'" class="muted">没有生成可审核的题目。</p>
      <button class="ghost" style="margin-top: 14px" @click="router.push('/admin/imports')">返回任务列表</button>
    </template>
  </AppShell>
</template>
