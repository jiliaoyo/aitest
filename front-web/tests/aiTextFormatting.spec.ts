import { describe, expect, it } from 'vitest'
import { formatAIText } from '@/app/format'

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
})
