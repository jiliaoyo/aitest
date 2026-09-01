<script setup lang="ts">
import type { MaterialDTO } from '@/api/types'

// 共享材料：切换小题时保持挂载与滚动位置；手机端可折叠并保持当前批次内的状态。
defineProps<{
  material: MaterialDTO
  collapsed: boolean
}>()

const emit = defineEmits<{ toggle: [] }>()
</script>

<template>
  <section class="card" style="background: var(--fg-soft); padding: 16px" :aria-label="material.title ?? '阅读材料'">
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px">
      <h3 style="font-size: 14px; margin: 0; font-family: var(--font-body); color: var(--muted)">
        共享材料{{ material.title ? ` · ${material.title}` : '' }}
      </h3>
      <button type="button" class="ghost" style="min-height: 32px; font-size: 13px" :aria-expanded="!collapsed" @click="emit('toggle')">
        {{ collapsed ? '展开材料 ▾' : '折叠材料 ▴' }}
      </button>
    </div>
    <p v-show="!collapsed" class="material-text" lang="ja" style="margin: 0; white-space: pre-wrap">{{ material.content }}</p>
  </section>
</template>
