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
const deletingMemory = ref(false)
const memoryDeleted = ref(false)
const memoryError = ref('')
const passwordForm = reactive({ currentPassword: '', newPassword: '', confirmPassword: '' })
const passwordFields = reactive<Record<string, string>>({})
const passwordSaving = ref(false)
const passwordSaved = ref(false)
const passwordError = ref('')

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

async function deleteMemory(): Promise<void> {
  if (!window.confirm('确定删除账号的全局做题记忆和 AI 建议吗？练习历史与成绩会保留。')) return
  deletingMemory.value = true
  memoryDeleted.value = false
  memoryError.value = ''
  try {
    await request<void>('/learning-memory', { method: 'DELETE' })
    memoryDeleted.value = true
  } catch (err) {
    memoryError.value = err instanceof ApiError ? err.message : '删除记忆失败'
  } finally {
    deletingMemory.value = false
  }
}

async function changePassword(): Promise<void> {
  for (const key of Object.keys(passwordFields)) delete passwordFields[key]
  passwordSaved.value = false
  passwordError.value = ''
  if (passwordForm.newPassword !== passwordForm.confirmPassword) {
    passwordFields.confirmPassword = '两次输入的新密码不一致'
    return
  }
  passwordSaving.value = true
  try {
    await request('/auth/change-password', {
      method: 'POST',
      body: { currentPassword: passwordForm.currentPassword, newPassword: passwordForm.newPassword },
    })
    passwordForm.currentPassword = ''
    passwordForm.newPassword = ''
    passwordForm.confirmPassword = ''
    passwordSaved.value = true
  } catch (err) {
    const fields = fieldErrors(err)
    for (const [key, value] of Object.entries(fields)) passwordFields[key] = value
    if (!Object.keys(fields).length) passwordError.value = err instanceof ApiError ? err.message : '修改密码失败，请重试'
  } finally {
    passwordSaving.value = false
  }
}
</script>

<template>
  <AppShell>
    <h1 style="font-size: 24px">个人中心</h1>
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

    <form id="change-password-form" class="card" style="max-width: 520px" @submit.prevent="changePassword">
      <h2 style="font-size: 16px">修改密码</h2>
      <div class="field">
        <label for="current-password">当前密码</label>
        <input id="current-password" v-model="passwordForm.currentPassword" type="password" autocomplete="current-password" />
        <p v-if="passwordFields.currentPassword" class="error">{{ passwordFields.currentPassword }}</p>
      </div>
      <div class="field">
        <label for="new-password">新密码</label>
        <input id="new-password" v-model="passwordForm.newPassword" type="password" autocomplete="new-password" />
        <p v-if="passwordFields.newPassword" class="error">{{ passwordFields.newPassword }}</p>
      </div>
      <div class="field">
        <label for="confirm-password">确认新密码</label>
        <input id="confirm-password" v-model="passwordForm.confirmPassword" type="password" autocomplete="new-password" />
        <p v-if="passwordFields.confirmPassword" class="error">{{ passwordFields.confirmPassword }}</p>
      </div>
      <p v-if="passwordSaved" class="tag" data-tone="success" role="status" style="margin-bottom: 10px">密码已修改</p>
      <p v-if="passwordError" class="error-summary" role="alert">{{ passwordError }}</p>
      <button class="primary" type="submit" :disabled="passwordSaving">{{ passwordSaving ? '修改中…' : '修改密码' }}</button>
    </form>

    <div class="card" style="max-width: 520px">
      <h2 style="font-size: 16px">全局做题记忆</h2>
      <p class="muted">删除统计和 AI 建议后，会从下一批练习重新累计；已有练习历史与成绩不会删除。</p>
      <p v-if="memoryDeleted" class="tag" data-tone="success" role="status">已删除，新的进度会重新累计</p>
      <p v-if="memoryError" class="error-summary" role="alert">{{ memoryError }}</p>
      <button class="danger" :disabled="deletingMemory" @click="deleteMemory">
        {{ deletingMemory ? '删除中…' : '删除做题记忆' }}
      </button>
    </div>

    <div class="card" style="max-width: 520px">
      <h2 style="font-size: 16px">账号</h2>
      <p class="muted">{{ sessionUser()?.email }}</p>
      <button class="danger" @click="logout">退出登录</button>
    </div>
  </AppShell>
</template>
