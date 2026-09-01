<script setup lang="ts">
import { computed } from 'vue'
import type { SaveState } from './useAnswerAutosave'
import { formatTime } from '@/app/format'

const props = defineProps<{
  state: SaveState
  savedAt: string | null
  localOnly: boolean
  anyError: boolean
}>()

const emit = defineEmits<{ retry: [] }>()

const text = computed(() => {
  if (props.state === 'saving') return '保存中…'
  if (props.state === 'error') return '保存失败'
  if (props.localOnly) return '尚未同步'
  if (props.state === 'saved') return `已保存 ${formatTime(props.savedAt)}`
  return ''
})
</script>

<template>
  <p class="muted" style="font-size: 13px; margin: 0; min-height: 20px" role="status" aria-live="polite">
    <template v-if="anyError && state === 'error'">
      保存失败，答案已暂存本地。
      <button type="button" class="ghost" style="min-height: 28px; padding: 0 8px; color: var(--danger)" @click="emit('retry')">
        重试保存
      </button>
    </template>
    <template v-else>{{ text }}</template>
  </p>
</template>
