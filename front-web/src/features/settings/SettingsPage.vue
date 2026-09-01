<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { request, ApiError, fieldErrors } from '@/api/client'
import type { Exam, Me } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import { clearSession, sessionUser } from '@/app/session'

const router = useRouter()

const exams = ref<Exam[]>([])
const form = reactive({ defaultLevelId: '' })
const saving = ref(false)
const saved = ref(false)
const errorMessage = ref('')

onMounted(async () => {
  const me = sessionUser()
  if (me?.defaultLevelId) form.defaultLevelId = me.defaultLevelId
  try {
    const res = await request<{ exams: Exam[] }>('/catalog')
    exams.value = res.exams
  } catch {
    // 忽略：级别下拉为空也可保存其他设置
  }
})

async function save(): Promise<void> {
  saving.value = true
  saved.value = false
  errorMessage.value = ''
  try {
    await request<Me>('/me', {
      method: 'PATCH',
      body: { defaultLevelId: form.defaultLevelId || null },
    })
    saved.value = true
  } catch (err) {
    errorMessage.value = Object.values(fieldErrors(err))[0] ?? (err instanceof ApiError ? err.message : '保存失败')
  } finally {
    saving.value = false
  }
}

async function logout(): Promise<void> {
  try {
    await request('/auth/logout', { method: 'POST' })
  } finally {
    clearSession()
    await router.replace('/login')
  }
}
</script>

<template>
  <AppShell>
    <h1 style="font-size: 24px">个人设置</h1>
    <form class="card" style="max-width: 520px" @submit.prevent="save">
      <div class="field">
        <label for="level">默认练习级别</label>
        <select id="level" v-model="form.defaultLevelId">
          <option value="">未设置</option>
          <option v-for="l in exams.flatMap((e) => e.levels)" :key="l.id" :value="l.id">{{ l.name }}</option>
        </select>
        <p class="muted" style="font-size: 13px">用于学习概览的推荐与快捷创建。</p>
      </div>
      <p v-if="saved" class="tag" data-tone="success" role="status" style="margin-bottom: 10px">已保存</p>
      <p v-if="errorMessage" class="error-summary" role="alert">{{ errorMessage }}</p>
      <button class="primary" type="submit" :disabled="saving">{{ saving ? '保存中…' : '保存' }}</button>
    </form>

    <div class="card" style="max-width: 520px">
      <h2 style="font-size: 16px">账号</h2>
      <p class="muted">{{ sessionUser()?.email }}</p>
      <button class="danger" @click="logout">退出登录</button>
    </div>
  </AppShell>
</template>
