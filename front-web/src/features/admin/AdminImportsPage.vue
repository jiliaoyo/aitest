<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { request, upload, ApiError } from '@/api/client'
import type { ImportJobDTO } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import AppStatus from '@/components/AppStatus.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { formatDateTime } from '@/app/format'

const jobs = ref<ImportJobDTO[]>([])
const nextCursor = ref('')
const loadingMore = ref(false)
const state = ref<'loading' | 'ready' | 'error'>('loading')
const file = ref<File | null>(null)
const uploading = ref(false)
const message = ref('')
const errorMessage = ref('')
const requestID = ref('')

async function load(append = false): Promise<void> {
  if (append) loadingMore.value = true
  else state.value = 'loading'
  try {
    const params = new URLSearchParams({ limit: '20' })
    if (append && nextCursor.value) params.set('cursor', nextCursor.value)
    const res = await request<{ jobs: ImportJobDTO[]; nextCursor?: string }>(`/admin/import-jobs?${params}`)
    jobs.value = append ? [...jobs.value, ...res.jobs] : res.jobs
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

function chooseFile(event: Event): void {
  file.value = (event.target as HTMLInputElement).files?.[0] ?? null
  message.value = ''
}

async function createJob(): Promise<void> {
  if (!file.value) {
    message.value = '请先选择文件。'
    return
  }
  uploading.value = true
  message.value = ''
  try {
    const res = await upload<{ job: ImportJobDTO }>('/admin/import-jobs', file.value)
    file.value = null
    const input = document.getElementById('import-file') as HTMLInputElement | null
    if (input) input.value = ''
    message.value = '已创建导入任务，正在处理。'
    await load()
    void res
  } catch (err) {
    message.value = err instanceof ApiError ? err.message : '上传失败，请重试'
  } finally {
    uploading.value = false
  }
}

onMounted(load)
</script>

<template>
  <AppShell>
    <div class="page-header">
      <div>
        <h1 style="font-size: 24px; margin: 0">导入任务</h1>
        <p class="muted" style="margin: 4px 0 0">上传本地 OCR 服务导出的结构化 JSON，逐题审核后发布上架；不会自动发布题目。</p>
      </div>
    </div>

    <form class="card" style="margin-bottom: 18px" @submit.prevent="createJob">
      <div class="field">
        <label for="import-file">选择题库文件</label>
        <input id="import-file" type="file" accept=".json" @change="chooseFile" />
        <p class="muted" style="font-size: 13px">仅支持本地 OCR 服务导出的结构化 JSON，上传后直接生成待审核草稿，单文件最大 10 MB。</p>
      </div>
      <button class="primary" type="submit" :disabled="uploading">{{ uploading ? '上传中…' : '上传并生成草稿' }}</button>
      <p v-if="message" class="tag" data-tone="success" role="status" style="margin: 10px 0 0">{{ message }}</p>
    </form>

    <AppStatus v-if="state === 'loading'" state="loading" />
    <AppStatus v-else-if="state === 'error'" state="error" :message="errorMessage" :request-id="requestID" @action="load" />
    <AppStatus v-else-if="jobs.length === 0" state="empty" message="还没有导入任务。" />
    <div v-else class="card" style="overflow-x: auto">
      <table class="data">
        <thead><tr><th>文件</th><th>状态</th><th class="num">题目数</th><th>更新时间</th></tr></thead>
        <tbody>
          <tr v-for="job in jobs" :key="job.id">
            <td><RouterLink :to="`/admin/imports/${job.id}`">{{ job.fileName }}</RouterLink></td>
            <td><StatusBadge :value="job.status" /></td>
            <td class="num">{{ job.itemCount }}</td>
            <td class="mono">{{ formatDateTime(job.updatedAt) }}</td>
          </tr>
        </tbody>
      </table>
      <p v-if="nextCursor" style="margin: 12px 0 0; text-align: center">
        <button :disabled="loadingMore" @click="load(true)">{{ loadingMore ? '加载中…' : '加载更多' }}</button>
      </p>
    </div>
  </AppShell>
</template>
