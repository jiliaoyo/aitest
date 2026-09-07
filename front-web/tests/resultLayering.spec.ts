import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import ResultItem from '@/features/practice/ResultItem.vue'
import type { ResultItem as ResultItemDTO } from '@/api/types'

const baseItem: ResultItemDTO = {
  id: 'item-1',
  position: 1,
  type: 'single_choice',
  material: null,
  stem: 'この店は、駅から近い（　　）、いつも混んでいる。',
  options: [
    { id: 'b', label: 'B', text: 'ことから' },
    { id: 'c', label: 'C', text: 'に沿って' },
  ],
  knowledgePoints: [{ id: 'kp', name: '助词 に 与 で' }],
  userAnswer: { optionIds: ['b'] },
  correctAnswer: { optionIds: ['c'] },
  explanation: null,
  answerAuthority: null,
  gradingSource: null,
  gradingStatus: 'incorrect',
}

describe('结果分层展示', () => {
  it('确定性结果展示权威来源标签，不与 AI 混淆', () => {
    const wrapper = mount(ResultItem, {
      props: {
        item: {
          ...baseItem,
          gradingSource: 'deterministic',
          answerAuthority: 'human_verified',
          explanation: { text: '人工解析内容', source: 'human_verified' },
        },
      },
    })
    const text = wrapper.text()
    expect(text).toContain('错误')
    expect(text).toContain('已审核答案')
    expect(text).toContain('人工解析')
    expect(text).toContain('B. ことから')
    expect(text).toContain('C. に沿って')
    expect(text).not.toContain('AI 判定')
    expect(text).not.toContain('AI 解析')
  })

  it('AI 判定与 AI 解析显示醒目来源标识', () => {
    const wrapper = mount(ResultItem, {
      props: {
        item: {
          ...baseItem,
          gradingSource: 'ai',
          gradingStatus: 'correct',
          explanation: { text: 'AI 第一句。AI 第二句。', source: 'ai' },
        },
      },
    })
    const text = wrapper.text()
    expect(text).toContain('AI 判定')
    expect(text).toContain('AI 解析（可能有误）')
    expect(text).toContain('正确')
    expect(wrapper.find('.ai-text').element.textContent).toContain('AI 第一句。\nAI 第二句。')
  })

  it('pending 题不显示标准答案，避免 AI 未完成时误导', () => {
    const wrapper = mount(ResultItem, {
      props: {
        item: {
          ...baseItem,
          gradingSource: 'ai',
          gradingStatus: 'pending',
          correctAnswer: null,
        },
      },
    })
    expect(wrapper.text()).toContain('待 AI 判定')
    expect(wrapper.text()).not.toContain('に沿って')
  })

  it('判定失败时展示出题时答案并保留仅供参考提示', () => {
    const wrapper = mount(ResultItem, {
      props: {
        item: {
          ...baseItem,
          gradingSource: 'ai',
          gradingStatus: 'failed',
				explanation: { text: '本题暂未完成 AI 判定，以下答案来自出题时的 AI 生成结果，仅供参考。', source: 'ai' },
        },
      },
    })
    expect(wrapper.text()).toContain('出题时答案')
    expect(wrapper.text()).toContain('C. に沿って')
		expect(wrapper.text()).toContain('本题暂未完成 AI 判定')
  })
})
