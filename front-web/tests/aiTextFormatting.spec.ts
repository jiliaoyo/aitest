import { describe, expect, it } from 'vitest'
import { formatAIText, formatAnswerValue, formatOverdueDays, splitAIExplanation } from '@/app/format'

describe('AI 文本排版', () => {
  it('把历史单行总结和字面量换行转换为可读段落', () => {
    expect(formatAIText('本批表现：答对 4 题。主要薄弱点：助词。下一步建议：继续练习。')).toBe(
      '本批表现：\n答对 4 题。\n主要薄弱点：\n助词。\n下一步建议：\n继续练习。',
    )
    expect(formatAIText('第一段\\n第二段')).toBe('第一段\n第二段')
    expect(formatAIText('答案依据：助词表示目的。知识点：に 的目的用法。常见误区：不要和で混淆。')).toBe(
      '答案依据：\n助词表示目的。\n知识点：\nに 的目的用法。\n常见误区：\n不要和で混淆。',
    )
  })

  it('把原文翻译与默认展开的解析分开', () => {
    expect(splitAIExplanation('原文翻译：这家店离车站很近。\n答案依据：考查助词。')).toEqual({
      translation: '这家店离车站很近。',
      analysis: '答案依据：考查助词。',
    })
    expect(splitAIExplanation('旧版解析。')).toEqual({ translation: '', analysis: '旧版解析。' })
  })
})

describe('答案格式', () => {
  it('保留题库中的填空可接受值和简答参考答案', () => {
    expect(formatAnswerValue({ acceptable: ['に', 'へ'] })).toBe('に、へ')
    expect(formatAnswerValue({ reference: '理由を説明する例文' })).toBe('理由を説明する例文')
  })
})

describe('复习逾期展示', () => {
  it('按整天计算逾期时长并处理今天与非法时间', () => {
    const now = Date.parse('2026-09-14T12:00:00Z')
    expect(formatOverdueDays('2026-09-14T00:00:00Z', now)).toBe('今天')
    expect(formatOverdueDays('2026-09-11T12:00:00Z', now)).toBe('3 天')
    expect(formatOverdueDays('not-a-date', now)).toBe('—')
  })
})
