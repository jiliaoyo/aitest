<script setup lang="ts">
import { ref } from 'vue'
import { request, ApiError, fieldErrors } from '@/api/client'

// 举报入口只发送 practiceItemId + 问题类型 + 可选说明；题目版本与判分上下文由后端补全。
const props = defineProps<{ practiceItemId: string }>()
const emit = defineEmits<{ submitted: [] }>()

const open = ref(false)
const targetType = ref<'stem' | 'answer' | 'explanation' | 'classification' | 'ai_grading'>('stem')
const description = ref('')
const submitting = ref(false)
const done = ref(false)
const error = ref('')

const typeLabels: Array<{ value: typeof targetType.value; label: string }> = [
  { value: 'stem', label: '题干有误' },
  { value: 'answer', label: '答案有误' },
  { value: 'explanation', label: '解析有误' },
  { value: 'classification', label: '分类不对' },
  { value: 'ai_grading', label: 'AI 判定不可信' },
]

async function submit(): Promise<void> {
  submitting.value = true
  error.value = ''
  try {
    await request('/issue-reports', {
      method: 'POST',
      body: { practiceItemId: props.practiceItemId, targetType: targetType.value, description: description.value },
    })
    done.value = true
    open.value = false
    emit('submitted')
  } catch (err) {
    const fields = fieldErrors(err)
    error.value = Object.values(fields)[0] ?? (err instanceof ApiError ? err.message : '提交失败，请重试')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <span>
    <button v-if="!done" type="button" class="ghost" style="min-height: 32px; padding: 0 8px; font-size: 13px" @click="open = true">
      举报本题问题
    </button>
    <span v-else class="tag">已提交反馈</span>

    <Teleport to="body">
      <div v-if="open" class="dialog-backdrop" @mousedown.self="open = false">
        <div class="dialog" role="dialog" aria-modal="true" aria-label="举报题目问题">
          <h2 style="font-size: 18px">举报本题问题</h2>
          <div class="field">
            <label for="report-type">问题类型</label>
            <select id="report-type" v-model="targetType">
              <option v-for="t in typeLabels" :key="t.value" :value="t.value">{{ t.label }}</option>
            </select>
          </div>
          <div class="field">
            <label for="report-desc">补充说明（可选）</label>
            <textarea id="report-desc" v-model="description" rows="3" maxlength="2000" />
          </div>
          <p v-if="error" class="error-summary" role="alert">{{ error }}</p>
          <div class="dialog-actions">
            <button type="button" @click="open = false">取消</button>
            <button type="button" class="primary" :disabled="submitting" @click="submit">
              {{ submitting ? '提交中' : '提交反馈' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </span>
</template>
