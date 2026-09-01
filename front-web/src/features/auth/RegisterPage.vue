<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { request, fieldErrors } from '@/api/client'
import { refreshSession } from '@/app/session'

const router = useRouter()
const form = reactive({ email: '', password: '', confirm: '' })
const fieldErr = reactive<Record<string, string>>({})
const topError = ref('')
const submitting = ref(false)

async function submit(): Promise<void> {
  for (const k of Object.keys(fieldErr)) delete fieldErr[k]
  if (form.password !== form.confirm) {
    fieldErr.confirm = '两次输入的密码不一致'
    return
  }
  submitting.value = true
  topError.value = ''
  try {
    await request('/auth/register', { method: 'POST', body: { email: form.email, password: form.password } })
    await refreshSession()
    router.replace('/')
  } catch (err) {
    const fields = fieldErrors(err)
    for (const [k, v] of Object.entries(fields)) fieldErr[k] = v
    if (err instanceof Error && !Object.keys(fields).length) {
      topError.value = err.message
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="layout-shell" style="max-width: 460px; padding-top: 64px">
    <h1>注册</h1>
    <p class="muted">创建账号，开始批次练习。</p>
    <form class="card" novalidate @submit.prevent="submit">
      <div v-if="topError" class="error-summary" role="alert">{{ topError }}</div>
      <div class="field">
        <label for="email">邮箱</label>
        <input id="email" v-model="form.email" type="email" autocomplete="email" />
        <p v-if="fieldErr.email" class="error">{{ fieldErr.email }}</p>
      </div>
      <div class="field">
        <label for="password">密码（至少 8 位）</label>
        <input id="password" v-model="form.password" type="password" autocomplete="new-password" />
        <p v-if="fieldErr.password" class="error">{{ fieldErr.password }}</p>
      </div>
      <div class="field">
        <label for="confirm">确认密码</label>
        <input id="confirm" v-model="form.confirm" type="password" autocomplete="new-password" />
        <p v-if="fieldErr.confirm" class="error">{{ fieldErr.confirm }}</p>
      </div>
      <button class="primary" type="submit" :disabled="submitting" style="width: 100%">
        {{ submitting ? '注册中' : '注册' }}
      </button>
    </form>
    <p style="margin-top: 14px">
      已有账号？<RouterLink to="/login">直接登录</RouterLink>
    </p>
  </div>
</template>
