<script setup lang="ts">
import { reactive, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { request, fieldErrors } from '@/api/client'

const form = reactive({ email: '' })
const fieldErr = reactive<Record<string, string>>({})
const sent = ref(false)
const devToken = ref('')
const submitting = ref(false)

async function submit(): Promise<void> {
  for (const k of Object.keys(fieldErr)) delete fieldErr[k]
  submitting.value = true
  try {
    const res = await request<{ ok: boolean; resetToken?: string }>('/auth/password-reset/request', {
      method: 'POST',
      body: form,
    })
    devToken.value = res.resetToken ?? ''
    sent.value = true
  } catch (err) {
    const fields = fieldErrors(err)
    for (const [k, v] of Object.entries(fields)) fieldErr[k] = v
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="layout-shell" style="max-width: 460px; padding-top: 64px">
    <h1>找回密码</h1>
    <form v-if="!sent" class="card" novalidate @submit.prevent="submit">
      <div class="field">
        <label for="email">注册邮箱</label>
        <input id="email" v-model="form.email" type="email" />
        <p v-if="fieldErr.email" class="error">{{ fieldErr.email }}</p>
      </div>
      <button class="primary" type="submit" :disabled="submitting" style="width: 100%">
        {{ submitting ? '提交中' : '发送找回请求' }}
      </button>
    </form>
    <div v-else class="card">
      <p>如果该邮箱已注册，找回请求已创建。</p>
      <div v-if="devToken">
        <p class="muted">开发环境未接入邮件通道，请使用以下一次性令牌继续：</p>
        <p class="mono" style="word-break: break-all">{{ devToken }}</p>
        <RouterLink class="tag" :to="`/reset-password?token=${encodeURIComponent(devToken)}`">设置新密码</RouterLink>
      </div>
    </div>
    <p style="margin-top: 14px"><RouterLink to="/login">返回登录</RouterLink></p>
  </div>
</template>
