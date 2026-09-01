<script setup lang="ts">
import { computed } from 'vue'
import type { AnswerValue, PreSubmitItem } from '@/api/types'
import { questionTypeText } from '@/app/format'
import MaterialPanel from './MaterialPanel.vue'

// 按题型渲染原生表单控件；只展示答题前 DTO 字段，不含任何答案线索。
const props = defineProps<{
  item: PreSubmitItem
  answer: AnswerValue
  marked: boolean
  materialCollapsed: boolean
}>()

const emit = defineEmits<{
  'update:answer': [value: AnswerValue]
  'update:marked': [value: boolean]
  'toggle-material': []
}>()

const selectedIds = computed<string[]>(() =>
  props.answer && 'optionIds' in props.answer ? props.answer.optionIds : [],
)
const textValue = computed(() => (props.answer && 'text' in props.answer ? props.answer.text ?? '' : ''))

function selectSingle(optionID: string): void {
  emit('update:answer', { optionIds: [optionID] })
}

function toggleMulti(optionID: string): void {
  const current = new Set(selectedIds.value)
  if (current.has(optionID)) {
    current.delete(optionID)
  } else {
    current.add(optionID)
  }
  emit('update:answer', { optionIds: [...current] })
}

function inputText(event: Event): void {
  const value = (event.target as HTMLInputElement | HTMLTextAreaElement).value
  emit('update:answer', value === '' ? null : { text: value })
}
</script>

<template>
  <article class="card" lang="ja">
    <header style="display: flex; justify-content: space-between; align-items: baseline; gap: 12px; margin-bottom: 12px">
      <p class="mono muted" style="margin: 0">第 {{ item.position }} 题 · {{ questionTypeText[item.type] ?? item.type }}</p>
      <label class="muted" style="display: flex; align-items: center; gap: 6px; font-size: 13px">
        <input
          type="checkbox"
          :checked="marked"
          @change="emit('update:marked', ($event.target as HTMLInputElement).checked)"
        />
        标记待检查
      </label>
    </header>

    <MaterialPanel
      v-if="item.material"
      :material="item.material"
      :collapsed="materialCollapsed"
      @toggle="emit('toggle-material')"
    />

    <p class="stem" style="font-size: 17px; margin: 14px 0 18px" lang="ja">{{ item.stem }}</p>

    <fieldset style="border: 0; padding: 0; margin: 0">
      <legend class="visually-hidden-ish" style="position: absolute; width: 1px; height: 1px; overflow: hidden">
        {{ questionTypeText[item.type] }}
      </legend>

      <template v-if="item.type === 'single_choice'">
        <label v-for="opt in item.options" :key="opt.id" class="option-row" lang="ja">
          <input
            type="radio"
            :name="`q-${item.id}`"
            :value="opt.id"
            :checked="selectedIds.includes(opt.id)"
            @change="selectSingle(opt.id)"
          />
          <span><span class="mono">{{ opt.label }}.</span> {{ opt.text }}</span>
        </label>
      </template>

      <template v-else-if="item.type === 'multiple_choice'">
        <label v-for="opt in item.options" :key="opt.id" class="option-row" lang="ja">
          <input type="checkbox" :checked="selectedIds.includes(opt.id)" @change="toggleMulti(opt.id)" />
          <span><span class="mono">{{ opt.label }}.</span> {{ opt.text }}</span>
        </label>
      </template>

      <template v-else-if="item.type === 'fill_blank'">
        <div class="field">
          <label :for="`input-${item.id}`" lang="zh-CN">你的答案</label>
          <input
            :id="`input-${item.id}`"
            type="text"
            lang="ja"
            :value="textValue"
            autocomplete="off"
            @input="inputText"
          />
        </div>
      </template>

      <template v-else>
        <div class="field">
          <label :for="`input-${item.id}`" lang="zh-CN">你的回答</label>
          <textarea :id="`input-${item.id}`" rows="3" lang="ja" :value="textValue" @input="inputText" />
        </div>
      </template>
    </fieldset>
  </article>
</template>
