<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue'

// 提交、发布、下架等不可轻易撤销操作的确认弹窗：焦点约束 + Esc 取消。
const props = defineProps<{
  open: boolean
  title: string
  confirmLabel?: string
  cancelLabel?: string
  tone?: 'primary' | 'danger'
}>()

const emit = defineEmits<{ confirm: []; cancel: [] }>()

const panel = ref<HTMLElement | null>(null)
const previouslyFocused = ref<HTMLElement | null>(null)

async function onKeydown(event: KeyboardEvent): Promise<void> {
  if (event.key === 'Escape') {
    event.preventDefault()
    emit('cancel')
    return
  }
  if (event.key === 'Tab' && panel.value) {
    const focusables = panel.value.querySelectorAll<HTMLElement>('button, a[href], input, textarea, select')
    if (focusables.length === 0) return
    const first = focusables[0]!
    const last = focusables[focusables.length - 1]!
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault()
      last.focus()
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault()
      first.focus()
    }
  }
}

onMounted(async () => {
  if (props.open) {
    previouslyFocused.value = document.activeElement as HTMLElement | null
    await nextTick()
    panel.value?.querySelector<HTMLElement>('button')?.focus()
  }
})

defineExpose({
  focusPanel: async () => {
    previouslyFocused.value = document.activeElement as HTMLElement | null
    await nextTick()
    panel.value?.querySelector<HTMLElement>('button')?.focus()
  },
  restoreFocus: () => {
    previouslyFocused.value?.focus()
  },
})
</script>

<template>
  <div v-if="open" class="dialog-backdrop" @keydown="onKeydown" @mousedown.self="emit('cancel')">
    <div ref="panel" class="dialog" role="dialog" aria-modal="true" :aria-label="title">
      <h2 style="font-size: 18px">{{ title }}</h2>
      <slot />
      <div class="dialog-actions">
        <button type="button" @click="emit('cancel')">{{ cancelLabel ?? '继续答题' }}</button>
        <button
          type="button"
          :class="tone === 'danger' ? 'danger' : 'primary'"
          @click="emit('confirm')"
        >
          {{ confirmLabel ?? '确认' }}
        </button>
      </div>
    </div>
  </div>
</template>
