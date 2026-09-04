import type { AIGenerationCategory } from '@/api/types'

export interface AIGenerationCategoryOption {
  value: AIGenerationCategory
  label: string
}

export interface AIGenerationCategoryGroup {
  label: string
  options: AIGenerationCategoryOption[]
}

const categoryGroups: Record<string, AIGenerationCategoryGroup> = {
  grammar: {
    label: '语法分类',
    options: [
      { value: 'mixed', label: '全部语法' },
      { value: 'grammar_case_particle', label: '格助词' },
      { value: 'grammar_conjunctive_particle', label: '接续助词' },
      { value: 'grammar_adverbial_particle', label: '副助词 / 係助词' },
      { value: 'grammar_final_particle', label: '终助词' },
      { value: 'grammar_auxiliary', label: '助动词' },
      { value: 'grammar_verb', label: '动词及活用' },
      { value: 'grammar_adjective', label: '形容词 / 形容动词及活用' },
      { value: 'grammar_adverb', label: '副词' },
      { value: 'grammar_conjunction', label: '接续词' },
      { value: 'grammar_adnominal', label: '连体词 / 指示词' },
      { value: 'grammar_sentence_pattern', label: '基本句型与句型表达' },
      { value: 'grammar_tense_aspect', label: '时态、体与状态' },
      { value: 'grammar_condition', label: '条件、假定与逆接' },
      { value: 'grammar_voice', label: '可能、被动、使役' },
      { value: 'grammar_benefactive', label: '授受与请求' },
      { value: 'grammar_honorific', label: '敬语与礼貌体' },
      { value: 'grammar_negation', label: '否定、限制与程度' },
    ],
  },
  vocabulary: {
    label: '文字词汇分类',
    options: [
      { value: 'mixed', label: '全部文字词汇' },
      { value: 'vocabulary_kanji', label: '汉字读音与表记' },
      { value: 'vocabulary_noun', label: '名词' },
      { value: 'vocabulary_verb', label: '动词' },
      { value: 'vocabulary_adjective', label: '形容词 / 形容动词' },
      { value: 'vocabulary_adverb', label: '副词' },
      { value: 'vocabulary_conjunction', label: '接续词 / 连词' },
      { value: 'vocabulary_pronoun', label: '代词 / 指示词' },
      { value: 'vocabulary_counter', label: '数量词 / 量词' },
      { value: 'vocabulary_time_number', label: '时间、日期与数字' },
      { value: 'vocabulary_synonym', label: '近义词与反义词' },
      { value: 'vocabulary_polysemy', label: '多义词与同音异义词' },
      { value: 'vocabulary_collocation', label: '词语搭配与惯用表达' },
      { value: 'vocabulary_compound', label: '复合词与词族' },
      { value: 'vocabulary_affix', label: '接头词与接尾词' },
      { value: 'vocabulary_onoma', label: '拟声词与拟态词' },
      { value: 'vocabulary_katakana', label: '片假名与外来语' },
      { value: 'vocabulary_honorific', label: '敬语词汇' },
      { value: 'vocabulary_usage', label: '语体与语境' },
    ],
  },
  reading: {
    label: '阅读分类',
    options: [
      { value: 'mixed', label: '全部阅读' },
      { value: 'reading_information', label: '信息检索与细节' },
      { value: 'reading_main_idea', label: '主旨与主题' },
      { value: 'reading_reference', label: '指代与照应' },
      { value: 'reading_paraphrase', label: '同义替换与转述' },
      { value: 'reading_logic', label: '因果、转折、并列与让步' },
      { value: 'reading_inference', label: '推断与隐含信息' },
      { value: 'reading_author', label: '作者态度、观点与意图' },
      { value: 'reading_vocabulary', label: '生词词义推测' },
      { value: 'reading_structure', label: '文章结构与段落功能' },
      { value: 'reading_chart_notice', label: '图表、公告、通知、邮件与对话' },
      { value: 'reading_style', label: '文体、语域与语气' },
    ],
  },
}

export function aiCategoryGroupsForSubject(subjectCode: string): AIGenerationCategoryGroup[] {
  return [categoryGroups[subjectCode] ?? { label: '综合分类', options: [{ value: 'mixed', label: '全部分类' }] }]
}

export function aiSubjectCode(subjectName: string): string {
  if (subjectName === '语法') return 'grammar'
  if (subjectName === '文字词汇') return 'vocabulary'
  if (subjectName === '阅读') return 'reading'
  return ''
}
