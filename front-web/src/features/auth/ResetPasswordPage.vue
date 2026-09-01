<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { request, fieldErrors } from '@/api/client'

const route = useRoute()
const router = useRouter()

const form = reactive({ password: '', confirm: '' })
const fieldErr = reactive<Record<string, string>>({})
const topError = ref('')
const submitting = ref(false)
const token = computed(() => (route.query.token as string | undefined) ?? '')

async function submit(): Promise<void> {
  for (const k of Object.keys(fieldErr)) delete fieldErr[k]
  if (form.password !== form.confirm) {
    fieldErr.confirm = '两次输入的密码不一致'
    return
  }
  submitting.value = true
  topError.value = ''
  try {
    await request('/auth/password-reset/confirm', {
      method: 'POST',
      body: { token: token.value, password: form.password },
    })
    router.replace('/login')
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
    <h1>设置新密码</h1>
    <form v-if="token" class="card" novalidate @submit.prevent="submit">
      <div v-if="topError" class="error-summary" role="alert">{{ topError }}</div>
      <div class="field">
        <label for="password">新密码（至少 8 位）</label>
        <input id="password" v-model="form.password" type="password" autocomplete="new-password" />
        <p v-if="fieldErr.password" class="error">{{ fieldErr.password }}</p>
      </div>
      <div class="field">
        <label for="confirm">确认新密码</label>
        <input id="confirm" v-model="form.confirm" type="password" autocomplete="new-password" />
        <p v-if="fieldErr.confirm" class="error">{{ fieldErr.confirm }}</p>
      </div>
      <button class="primary" type="submit" :disabled="submitting" style="width: 100%">
        {{ submitting ? '提交中' : '重置密码' }}
      </button>
    </form>
    <div v-else class="card">
      <p>缺少找回令牌，请从找回密码页面重新发起。</p>
      <RouterLink class="tag" to="/forgot-password">找回密码</RouterLink>
    </div>
  </div>
</template>
