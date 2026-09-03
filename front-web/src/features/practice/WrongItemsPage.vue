<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { request, ApiError } from '@/api/client'
import type { KnowledgePointItem, WrongItem } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import AppStatus from '@/components/AppStatus.vue'
import { authorityText, gradingStatusText } from '@/app/format'

const router = useRouter()

const items = ref<WrongItem[]>([])
const kps = ref<KnowledgePointItem[]>([])
const kpFilter = ref('')
const state = ref<'loading' | 'ready' | 'error'>('loading')
const errorMessage = ref('')
const requestID = ref('')
const creating = ref(false)

async function load(): Promise<void> {
  state.value = 'loading'
  try {
    const params = new URLSearchParams()
    if (kpFilter.value) params.set('knowledgePointId', kpFilter.value)
    const res = await request<{ wrongItems: WrongItem[] }>(`/wrong-items?${params}`)
    items.value = res.wrongItems
    state.value = 'ready'
  } catch (err) {
    errorMessage.value = err instanceof ApiError ? err.message : '加载失败'
    requestID.value = err instanceof ApiError ? err.requestId ?? '' : ''
    state.value = 'error'
  }
}

onMounted(async () => {
  await load()
  try {
    const res = await request<{ knowledgePoints: KnowledgePointItem[] }>('/knowledge-points')
    kps.value = res.knowledgePoints.filter((k) => (k.stats?.confirmedAnswered ?? 0) > 0)
  } catch {
    // 筛选列表加载失败不阻塞主列表
  }
})

function optionText(item: WrongItem, ids: string[] | undefined): string {
  if (!ids) return '—'
  return ids.map((id) => item.options?.find((o) => o.id === id)?.label ?? id).join('、')
}

function answerText(item: WrongItem, answer: WrongItem['userAnswer']): string {
  if (!answer) return '未作答'
  if ('optionIds' in answer && answer.optionIds) return optionText(item, answer.optionIds)
  if ('text' in answer && answer.text) return answer.text
  return '未作答'
}

function correctText(item: WrongItem): string {
  const answer = item.correctAnswer
  if (!answer) return '—'
  if ('optionIds' in answer && answer.optionIds) return optionText(item, answer.optionIds)
  if ('text' in answer && answer.text) return answer.text
  return '—'
}

const canRetrain = computed(() => items.value.length > 0)

async function retrain(): Promise<void> {
  creating.value = true
  try {
    const me = await request<{ user: { defaultLevelId: string | null } }>('/me')
    if (!me.user.defaultLevelId) {
      await router.push('/practice/new')
      return
    }
    const session = await request<{ id: string }>('/practice-sessions', {
      method: 'POST',
      body: { levelId: me.user.defaultLevelId, mode: 'wrong_items', count: 10 },
    })
    await router.push(`/practice/${session.id}`)
  } catch (err) {
    errorMessage.value = err instanceof ApiError ? err.message : '创建错题练习失败'
    state.value = 'error'
  } finally {
    creating.value = false
  }
}
</script>

<template>
  <AppShell>
    <div class="page-header">
      <h1 style="font-size: 24px; margin: 0">错题本</h1>
      <button class="primary" :disabled="!canRetrain || creating" @click="retrain">
        {{ creating ? '创建中…' : '错题重练 10 题' }}
      </button>
    </div>

    <div class="field" style="max-width: 320px">
      <label for="kp-filter">按知识点筛选</label>
      <select id="kp-filter" v-model="kpFilter" @change="load">
        <option value="">全部知识点</option>
        <option v-for="k in kps" :key="k.id" :value="k.id">{{ k.name }}</option>
      </select>
    </div>

    <AppStatus v-if="state === 'loading'" state="loading" />
    <AppStatus v-else-if="state === 'error'" state="error" :message="errorMessage" :request-id="requestID" @action="load" />
    <AppStatus v-else-if="items.length === 0" state="empty" message="没有待复习的错题，继续保持。" action-label="创建新练习" @action="router.push('/practice/new')" />
    <div v-else style="display: flex; flex-direction: column; gap: 18px">
      <article v-for="item in items" :key="item.itemId" class="card" lang="ja">
        <header style="display: flex; flex-wrap: wrap; gap: 8px; justify-content: space-between; align-items: center">
          <span class="tag" data-tone="danger">{{ gradingStatusText[item.gradingStatus] ?? item.gradingStatus }}</span>
          <span v-if="item.answerAuthority" class="tag" data-tone="success">{{ authorityText[item.answerAuthority] }}</span>
          <span v-if="item.knowledgePoints.length" class="muted">{{ item.knowledgePoints.map((k) => k.name).join('、') }}</span>
        </header>
        <section v-if="item.material" class="card" style="background: var(--fg-soft); padding: 14px; margin-top: 10px">
          <p class="material-text" style="margin: 0; white-space: pre-wrap">{{ item.material.content }}</p>
        </section>
        <p style="font-size: 16px; margin: 12px 0 8px; white-space: pre-wrap">{{ item.stem }}</p>
        <p class="mono" style="margin: 0">你的答案：{{ answerText(item, item.userAnswer) }} · 标准答案：{{ correctText(item) }}</p>
        <div v-if="item.explanation" style="margin-top: 10px; border-top: 1px solid var(--border); padding-top: 10px">
          <p class="tag" style="margin-bottom: 6px">
            {{ item.explanation.source === 'ai' ? 'AI 解析（可能有误）' : item.explanation.source === 'official' ? '官方解析' : '人工解析' }}
          </p>
          <p style="margin: 0; white-space: pre-wrap">{{ item.explanation.text }}</p>
        </div>
      </article>
    </div>
  </AppShell>
</template>
