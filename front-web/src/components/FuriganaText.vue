<script setup lang="ts">
import { computed } from 'vue'
import { sessionUser } from '@/app/session'

const props = defineProps<{ text: string; size?: number }>()
const pattern = /([\p{Script=Han}々〆ヵヶ]+)[（(]([ぁ-ゖァ-ヺー]+)[）)]/gu

const parts = computed(() => {
  if (sessionUser()?.showFurigana === false) return [{ text: props.text }]
  const result: { text: string; reading?: string }[] = []
  let offset = 0
  for (const match of props.text.matchAll(pattern)) {
    if (match.index > offset) result.push({ text: props.text.slice(offset, match.index) })
    result.push({ text: match[1]!, reading: match[2]! })
    offset = match.index + match[0].length
  }
  if (offset < props.text.length) result.push({ text: props.text.slice(offset) })
  return result.length ? result : [{ text: props.text }]
})
const fontSize = computed(() => `${props.size ?? sessionUser()?.furiganaSize ?? 70}%`)
</script>

<template>
  <template v-for="(part, index) in parts" :key="index">
    <ruby v-if="part.reading">{{ part.text }}<rp>（</rp><rt :style="{ fontSize }">{{ part.reading }}</rt><rp>）</rp></ruby>
    <template v-else>{{ part.text }}</template>
  </template>
</template>
