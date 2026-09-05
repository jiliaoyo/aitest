<script setup lang="ts">
import { computed } from 'vue'
import type { OptionDTO, ResultItem as ResultItemDTO } from '@/api/types'
import { authorityText, explanationSourceText, formatAIText, gradingStatusText, questionTypeText } from '@/app/format'
import ReportDialog from '@/features/issues/ReportDialog.vue'

// 逐题解析：正式分层（确定性）与 AI 判定分开呈现，来源标签始终可见。
const props = defineProps<{ item: ResultItemDTO }>()

function answerText(answer: ResultItemDTO['userAnswer'], options: OptionDTO[]): string {
  if (!answer) return '—'
  if ('optionIds' in answer && answer.optionIds) {
    return answer.optionIds
      .map((id) => {
        const option = options.find((o) => o.id === id)
        return option ? `${option.label}. ${option.text}` : id
      })
      .join('、')
  }
  if ('text' in answer && answer.text) return answer.text
  return '—'
}

const userText = computed(() => answerText(props.item.userAnswer, props.item.options))
const correctText = computed(() =>
  props.item.gradingStatus === 'pending' || props.item.gradingStatus === 'failed'
    ? '待 AI 判定'
    : answerText(props.item.correctAnswer, props.item.options),
)
const statusTone = computed(() =>
  props.item.gradingStatus === 'correct'
    ? 'success'
    : props.item.gradingStatus === 'pending'
      ? 'accent'
      : props.item.gradingStatus === 'failed'
        ? 'danger'
        : props.item.gradingStatus === 'unanswered'
          ? 'warning'
          : 'danger',
)
const isAI = computed(() => props.item.gradingSource === 'ai')
const reportItemID = computed(() => props.item.id)
</script>

<template>
  <article class="card" lang="ja">
    <header style="display: flex; flex-wrap: wrap; gap: 8px; align-items: center; justify-content: space-between">
      <p class="mono muted" style="margin: 0">第 {{ item.position }} 题 · {{ questionTypeText[item.type] ?? item.type }}</p>
      <p v-if="item.sourceSectionName" class="muted" style="margin: 0; font-size: 13px">{{ item.sourceSectionName }}</p>
      <div style="display: flex; gap: 6px; flex-wrap: wrap">
        <span class="tag" :data-tone="statusTone">{{ gradingStatusText[item.gradingStatus] ?? item.gradingStatus }}</span>
        <span v-if="isAI" class="tag" data-tone="accent">AI 判定（可能有误）</span>
        <span v-else-if="item.answerAuthority" class="tag" data-tone="success">{{ authorityText[item.answerAuthority] }}</span>
      </div>
    </header>

    <section v-if="item.material" class="card" style="background: var(--fg-soft); padding: 14px; margin-top: 12px">
      <p class="muted" style="font-size: 13px; margin: 0 0 6px">共享材料{{ item.material.title ? ` · ${item.material.title}` : '' }}</p>
      <p class="material-text" style="margin: 0; white-space: pre-wrap" lang="ja">{{ item.material.content }}</p>
    </section>

    <p style="font-size: 16px; margin: 14px 0 8px; white-space: pre-wrap" lang="ja">{{ item.stem }}</p>

    <dl style="margin: 0; display: grid; grid-template-columns: max-content 1fr; gap: 4px 16px">
      <dt class="muted">你的答案</dt>
      <dd class="mono" style="margin: 0">{{ userText }}</dd>
      <dt class="muted">标准答案</dt>
      <dd class="mono" style="margin: 0">{{ correctText }}</dd>
      <template v-if="item.knowledgePoints.length">
        <dt class="muted">知识点</dt>
        <dd style="margin: 0">{{ item.knowledgePoints.map((k) => k.name).join('、') }}</dd>
      </template>
    </dl>

    <div v-if="item.explanation" style="margin-top: 12px; border-top: 1px solid var(--border); padding-top: 10px">
      <p class="tag" data-tone="neutral" style="margin-bottom: 6px">
        {{ explanationSourceText[item.explanation.source] ?? item.explanation.source }}
      </p>
      <p class="ai-text" style="margin: 0">{{ item.explanation.source === 'ai' ? formatAIText(item.explanation.text) : item.explanation.text }}</p>
    </div>
    <p v-else-if="item.gradingStatus === 'failed'" class="muted" style="margin-top: 10px">
      分析失败，稍后可重试；确定性成绩不受影响。
    </p>

    <footer style="margin-top: 12px">
      <ReportDialog :practice-item-id="reportItemID" />
    </footer>
  </article>
</template>
