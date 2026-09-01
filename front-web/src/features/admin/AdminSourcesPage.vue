<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { request, ApiError, fieldErrors } from '@/api/client'
import type { SourceDTO } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import AppStatus from '@/components/AppStatus.vue'
import { sourceKindText } from '@/app/format'

const sources = ref<SourceDTO[]>([])
const state = ref<'loading' | 'ready' | 'error'>('loading')
const errorMessage = ref('')
const requestID = ref('')

const form = reactive({
  name: '',
  kind: 'self_made' as SourceDTO['kind'],
  author: '',
  publisher: '',
  year: '' as number | '',
  licenseNote: '',
  internalNote: '',
})
const sectionNames = reactive(new Map<string, string>())
const fieldErr = reactive<Record<string, string>>({})
const topError = ref('')
const saving = ref(false)

async function load(): Promise<void> {
  state.value = 'loading'
  try {
    const res = await request<{ sources: SourceDTO[] }>('/admin/sources')
    sources.value = res.sources
    state.value = 'ready'
  } catch (err) {
    errorMessage.value = err instanceof ApiError ? err.message : '加载失败'
    requestID.value = err instanceof ApiError ? err.requestId ?? '' : ''
    state.value = 'error'
  }
}

onMounted(load)

async function create(): Promise<void> {
  saving.value = true
  topError.value = ''
  for (const k of Object.keys(fieldErr)) delete fieldErr[k]
  try {
    await request('/admin/sources', {
      method: 'POST',
      body: {
        name: form.name,
        kind: form.kind,
        author: form.author,
        publisher: form.publisher,
        year: form.year === '' ? null : Number(form.year),
        licenseNote: form.licenseNote,
        internalNote: form.internalNote,
      },
    })
    form.name = ''
    form.author = ''
    form.publisher = ''
    form.year = ''
    form.licenseNote = ''
    form.internalNote = ''
    await load()
  } catch (err) {
    const fields = fieldErrors(err)
    for (const [k, v] of Object.entries(fields)) fieldErr[k] = v
    topError.value = Object.keys(fields).length ? '请检查表单' : err instanceof ApiError ? err.message : '创建失败'
  } finally {
    saving.value = false
  }
}

async function addSection(source: SourceDTO): Promise<void> {
  const name = sectionNames.get(source.id)?.trim()
  if (!name) return
  try {
    await request(`/admin/sources/${source.id}/sections`, { method: 'POST', body: { name } })
    sectionNames.set(source.id, '')
    await load()
  } catch (err) {
    errorMessage.value = err instanceof ApiError ? err.message : '创建章节失败'
  }
}
</script>

<template>
  <AppShell>
    <h1 style="font-size: 24px">来源管理</h1>
    <p class="muted">每个来源必须记录授权或使用依据后才能被题目引用。</p>

    <form class="card" @submit.prevent="create">
      <h2 style="font-size: 16px">新建来源</h2>
      <p v-if="topError" class="error-summary" role="alert">{{ topError }}</p>
      <div class="grid-2">
        <div class="field">
          <label for="s-name">名称</label>
          <input id="s-name" v-model="form.name" type="text" />
          <p v-if="fieldErr.name" class="error">{{ fieldErr.name }}</p>
        </div>
        <div class="field">
          <label for="s-kind">类型</label>
          <select id="s-kind" v-model="form.kind">
            <option value="book">书籍</option>
            <option value="past_exam">真题</option>
            <option value="self_made">自建</option>
            <option value="ai_generated">AI 生成</option>
          </select>
        </div>
      </div>
      <div class="grid-3">
        <div class="field">
          <label for="s-author">作者</label>
          <input id="s-author" v-model="form.author" type="text" />
        </div>
        <div class="field">
          <label for="s-publisher">出版方</label>
          <input id="s-publisher" v-model="form.publisher" type="text" />
        </div>
        <div class="field">
          <label for="s-year">年份</label>
          <input id="s-year" v-model="form.year" type="number" min="1900" max="2100" />
        </div>
      </div>
      <div class="field">
        <label for="s-license">授权说明（必填）</label>
        <textarea id="s-license" v-model="form.licenseNote" rows="2" />
        <p v-if="fieldErr.licenseNote" class="error">{{ fieldErr.licenseNote }}</p>
      </div>
      <div class="field">
        <label for="s-note">内部备注</label>
        <textarea id="s-note" v-model="form.internalNote" rows="2" />
      </div>
      <button class="primary" type="submit" :disabled="saving">{{ saving ? '创建中…' : '创建来源' }}</button>
    </form>

    <AppStatus v-if="state === 'loading'" state="loading" />
    <AppStatus v-else-if="state === 'error'" state="error" :message="errorMessage" :request-id="requestID" @action="load" />
    <AppStatus v-else-if="sources.length === 0" state="empty" message="还没有来源。" />
    <div v-else style="display: flex; flex-direction: column; gap: 18px">
      <article v-for="s in sources" :key="s.id" class="card">
        <div class="page-header">
          <div>
            <h2 style="font-size: 16px; margin: 0">{{ s.name }}</h2>
            <p class="muted" style="margin: 4px 0 0">
              {{ sourceKindText[s.kind] ?? s.kind }}<template v-if="s.author"> · {{ s.author }}</template><template v-if="s.publisher"> · {{ s.publisher }}</template><template v-if="s.year"> · {{ s.year }}</template>
            </p>
            <p class="muted" style="font-size: 13px; margin: 4px 0 0">授权：{{ s.licenseNote || '（未填写）' }}</p>
          </div>
        </div>
        <ul style="margin: 10px 0; padding-left: 18px">
          <li v-for="sec in s.sections" :key="sec.id" class="mono" style="font-size: 13px">{{ sec.name }}</li>
        </ul>
        <div style="display: flex; gap: 8px">
          <input
            :value="sectionNames.get(s.id) ?? ''"
            type="text"
            placeholder="新章节名称"
            style="max-width: 260px"
            @input="sectionNames.set(s.id, ($event.target as HTMLInputElement).value)"
          />
          <button @click="addSection(s)">添加章节</button>
        </div>
      </article>
    </div>
  </AppShell>
</template>
