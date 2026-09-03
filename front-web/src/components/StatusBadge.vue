<script setup lang="ts">
import { computed } from 'vue'
import { authorityText, explanationSourceText, gradingStatusText, sessionStatusText, statusText } from '@/app/format'

// 统一状态文案与语义颜色；状态不能只靠颜色（文字始终随标签展示）。
const props = defineProps<{ value: string; kind?: 'session' | 'grading' | 'authority' | 'explanation' | 'content' | 'issue' }>()

const text = computed(() => {
  switch (props.kind) {
    case 'session':
      return sessionStatusText[props.value] ?? props.value
    case 'grading':
      return gradingStatusText[props.value] ?? props.value
    case 'authority':
      return authorityText[props.value] ?? props.value
    case 'explanation':
      return explanationSourceText[props.value] ?? props.value
    case 'issue':
      return statusText[props.value] ?? props.value
    default:
      return statusText[props.value] ?? props.value
  }
})

const tone = computed(() => {
  switch (props.value) {
    case 'correct':
    case 'completed':
    case 'published':
    case 'official':
    case 'human_verified':
    case 'resolved':
    case 'approved':
      return 'success'
    case 'pending':
    case 'grading':
    case 'in_review':
    case 'generating':
    case 'draft':
    case 'unanswered':
    case 'uploaded':
    case 'extracting':
    case 'structuring':
    case 'review_ready':
      return 'warning'
    case 'incorrect':
    case 'failed':
    case 'analysis_failed':
    case 'generation_failed':
    case 'retired':
    case 'rejected':
      return 'danger'
    case 'active':
    case 'ai':
      return 'accent'
    default:
      return 'neutral'
  }
})
</script>

<template>
  <span class="tag" :data-tone="tone">{{ text }}</span>
</template>
