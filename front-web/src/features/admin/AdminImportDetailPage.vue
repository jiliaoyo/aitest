<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
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
const state = ref<'loading' | 'ready' | 'error'>('loading')
const errorMessage = ref('')
const requestID = ref('')
const retrying = ref(false)
let timer: ReturnType<typeof setTimeout> | null = null

function shouldPoll(): boolean {
  return job.value?.status === 'uploaded' || job.value?.status === 'extracting' || job.value?.status === 'structuring'
}

function schedule(): void {
  if (timer) clearTimeout(timer)
  if (shouldPoll()) timer = setTimeout(() => void load(false), 3000)
}

async function load(initial = true): Promise<void> {
  if (initial) state.value = 'loading'
  try {
    const res = await request<{ job: ImportJobDTO; items: ImportItemDTO[] }>(`/admin/import-jobs/${jobID}`)
    job.value = res.job
    items.value = res.items
    state.value = 'ready'
    schedule()
  } catch (err) {
    errorMessage.value = err instanceof ApiError ? err.message : '加载失败'
    requestID.value = err instanceof ApiError ? err.requestId ?? '' : ''
    state.value = 'error'
  }
}

async function retry(): Promise<void> {
  retrying.value = true
  try {
    await request(`/admin/import-jobs/${jobID}/retry`, { method: 'POST' })
    await load()
  } catch (err) {
    errorMessage.value = err instanceof ApiError ? err.message : '重试失败'
  } finally {
    retrying.value = false
  }
}

onMounted(() => void load())
onBeforeUnmount(() => { if (timer) clearTimeout(timer) })
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
        <button v-if="job.status === 'failed'" class="primary" :disabled="retrying" @click="retry">{{ retrying ? '重试中…' : '从安全阶段重试' }}</button>
      </div>

      <div v-if="job.extractedText" class="card" style="margin-bottom: 18px">
        <h2 style="font-size: 18px">提取的原文</h2>
        <pre class="import-source" lang="ja">{{ job.extractedText }}</pre>
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
      </div>
      <p v-else-if="job.status === 'review_ready'" class="muted">没有生成可审核的题目。</p>
      <p v-if="job.status === 'uploaded' || job.status === 'extracting' || job.status === 'structuring'" class="muted" role="status">任务处理中，页面每 3 秒刷新。</p>
      <button class="ghost" style="margin-top: 14px" @click="router.push('/admin/imports')">返回任务列表</button>
    </template>
  </AppShell>
</template>
