<script setup lang="ts">
// 只覆盖四种通用页面状态；业务状态（grading、saving 等）放在所属页面。
const props = defineProps<{
  state: 'loading' | 'empty' | 'error' | 'forbidden'
  message?: string
  requestId?: string
  actionLabel?: string
}>()

const emit = defineEmits<{ action: [] }>()

function defaultEmpty(): string {
  return props.message ?? '这里还没有内容'
}
</script>

<template>
  <div class="card" role="status">
    <template v-if="state === 'loading'">
      <p class="muted">加载中…</p>
    </template>
    <template v-else-if="state === 'empty'">
      <p>{{ defaultEmpty() }}</p>
      <button v-if="actionLabel" class="primary" @click="emit('action')">{{ actionLabel }}</button>
    </template>
    <template v-else-if="state === 'forbidden'">
      <h2>没有访问权限</h2>
      <p class="muted">{{ message ?? '该页面仅管理员可用。' }}</p>
      <RouterLink class="tag" to="/">返回学习端</RouterLink>
    </template>
    <template v-else>
      <p>{{ message ?? '加载失败，请重试。' }}</p>
      <p v-if="requestId" class="muted mono" style="font-size: 12px">请求 ID：{{ requestId }}（可复制反馈）</p>
      <button class="primary" @click="emit('action')">重试</button>
    </template>
  </div>
</template>
