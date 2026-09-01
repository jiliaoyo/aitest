<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { request } from '@/api/client'
import { fieldErrors } from '@/api/client'
import { refreshSession, safeRedirect } from '@/app/session'

const router = useRouter()
const route = useRoute()

const form = reactive({ email: '', password: '' })
const fieldErr = reactive<Record<string, string>>({})
const topError = ref('')
const submitting = ref(false)

async function submit(): Promise<void> {
  submitting.value = true
  topError.value = ''
  for (const k of Object.keys(fieldErr)) delete fieldErr[k]
  try {
    await request('/auth/login', { method: 'POST', body: form })
    await refreshSession()
    router.replace(safeRedirect(route.query.redirect as string | undefined))
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
    <h1>登录</h1>
    <p class="muted">登录后继续你的 JLPT 练习计划。</p>
    <form class="card" novalidate @submit.prevent="submit">
      <div v-if="topError" class="error-summary" role="alert">{{ topError }}</div>
      <div class="field">
        <label for="email">邮箱</label>
        <input id="email" v-model="form.email" type="email" autocomplete="email" :aria-describedby="fieldErr.email ? 'email-err' : undefined" />
        <p v-if="fieldErr.email" id="email-err" class="error">{{ fieldErr.email }}</p>
      </div>
      <div class="field">
        <label for="password">密码</label>
        <input id="password" v-model="form.password" type="password" autocomplete="current-password" :aria-describedby="fieldErr.password ? 'password-err' : undefined" />
        <p v-if="fieldErr.password" id="password-err" class="error">{{ fieldErr.password }}</p>
      </div>
      <button class="primary" type="submit" :disabled="submitting" style="width: 100%">
        {{ submitting ? '登录中' : '登录' }}
      </button>
    </form>
    <p style="margin-top: 14px">
      <RouterLink to="/forgot-password">找回密码</RouterLink>
      ·
      <RouterLink to="/register">注册新账号</RouterLink>
    </p>
  </div>
</template>
