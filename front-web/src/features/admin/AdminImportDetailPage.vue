<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
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
const reviewFilter = ref<'all' | 'anomaly' | 'approved_unpublished' | 'pending' | 'approved' | 'published' | 'rejected'>('all')
const selectedIDs = ref<string[]>([])
const batchMessage = ref('')
const batchPublishing = ref(false)
const visibleItems = computed(() => {
  if (reviewFilter.value === 'all') return items.value
  if (reviewFilter.value === 'anomaly') return items.value.filter((item) => item.anomalies.length > 0)
  if (reviewFilter.value === 'approved_unpublished') return items.value.filter((item) => item.reviewStatus === 'approved' && !item.publishedQuestionId)
  return items.value.filter((item) => item.reviewStatus === reviewFilter.value)
})
const selectableItems = computed(() => visibleItems.value.filter((item) => item.reviewStatus === 'approved' && !item.publishedQuestionId))

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
    }
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

function loadMore(): void {
  void load(false, true)
}

function toggleItem(id: string, checked: boolean): void {
  selectedIDs.value = checked ? [...new Set([...selectedIDs.value, id])] : selectedIDs.value.filter((value) => value !== id)
}

function selectVisible(): void {
  selectedIDs.value = [...new Set([...selectedIDs.value, ...selectableItems.value.map((item) => item.id)])]
}

async function publishSelected(): Promise<void> {
  if (!selectedIDs.value.length) return
  batchPublishing.value = true
  batchMessage.value = ''
  try {
    if (selectedIDs.value.length > 50) {
      batchMessage.value = '一次最多发布 50 个导入项。'
      return
    }
    const result = await request<{ successCount: number; failureCount: number }>(`/admin/import-jobs/${jobID}/publish-approved`, {
      method: 'POST', body: { itemIds: selectedIDs.value },
    })
    batchMessage.value = `发布完成：${result.successCount} 成功，${result.failureCount} 失败。`
    selectedIDs.value = []
    await load()
  } catch (err) {
    batchMessage.value = err instanceof ApiError ? err.message : '批量发布失败'
  } finally {
    batchPublishing.value = false
  }
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
        <div class="grid-2">
          <div class="field"><label for="review-filter">审核筛选</label><select id="review-filter" v-model="reviewFilter"><option value="all">全部</option><option value="anomaly">有异常</option><option value="approved_unpublished">已审核未发布</option><option value="pending">待审核</option><option value="approved">已审核</option><option value="published">已发布</option><option value="rejected">已退回</option></select></div>
          <div style="display: flex; align-items: end; gap: 8px; flex-wrap: wrap"><button type="button" @click="selectVisible">选择当前已审核项</button><button class="primary" type="button" :disabled="batchPublishing || !selectedIDs.length" @click="publishSelected">{{ batchPublishing ? '发布中…' : `批量发布（${selectedIDs.length}）` }}</button></div>
        </div>
        <p v-if="batchMessage" class="muted" role="status">{{ batchMessage }}</p>
      </div>

      <div v-if="items.length" class="card" style="overflow-x: auto">
        <h2 style="font-size: 18px">结构化草稿</h2>
        <table class="data mobile-card-table">
          <thead><tr><th class="num">#</th><th>选择</th><th>题干</th><th>异常</th><th>审核状态</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="item in visibleItems" :key="item.id">
              <td class="num" data-label="#">{{ item.position }}</td>
              <td data-label="选择"><input type="checkbox" :checked="selectedIDs.includes(item.id)" :disabled="item.reviewStatus !== 'approved' || !!item.publishedQuestionId" @change="toggleItem(item.id, ($event.target as HTMLInputElement).checked)" /></td>
              <td data-label="题干" lang="ja">{{ item.draft?.stem ?? '—' }}</td>
              <td data-label="异常"><span v-if="item.anomalies.length" class="tag" data-tone="warning">{{ item.anomalies.length }} 项待确认</span><span v-else class="muted">—</span></td>
              <td data-label="审核状态"><StatusBadge :value="item.reviewStatus" /></td>
              <td data-label=""><RouterLink :to="`/admin/import-items/${item.id}`">对照审核</RouterLink></td>
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
