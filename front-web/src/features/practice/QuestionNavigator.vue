<script setup lang="ts">
import type { PreSubmitItem } from '@/api/types'

// 题号导航：已答 / 标记 / 当前位置一目了然，不使用颜色暗示对错。
const props = defineProps<{
  items: PreSubmitItem[]
  currentIndex: number
  isAnswered: (item: PreSubmitItem) => boolean
  isMarked: (item: PreSubmitItem) => boolean
}>()

const emit = defineEmits<{ select: [index: number] }>()

function stateOf(item: PreSubmitItem, index: number): string {
  const parts: string[] = []
  if (props.isAnswered(item)) parts.push('answered')
  if (props.isMarked(item)) parts.push('marked')
  if (index === props.currentIndex) parts.push('current')
  return parts.join(' ') || 'unanswered'
}
</script>

<template>
  <nav aria-label="题目导航" class="navigator">
    <button
      v-for="(item, index) in items"
      :key="item.id"
      type="button"
      :data-state="stateOf(item, index)"
      :aria-label="`第 ${item.position} 题${props.isAnswered(item) ? '，已答' : '，未答'}${props.isMarked(item) ? '，已标记' : ''}`"
      :aria-current="index === currentIndex ? 'true' : undefined"
      @click="emit('select', index)"
    >
      {{ item.position }}
    </button>
  </nav>
</template>
