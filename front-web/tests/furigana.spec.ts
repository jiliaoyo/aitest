import { afterEach, describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import FuriganaText from '@/components/FuriganaText.vue'
import { clearSession, setSessionUser } from '@/app/session'

afterEach(clearSession)

describe('假名排版', () => {
  it('默认把括号假名放到汉字上方，并可按账号设置关闭', async () => {
    const wrapper = mount(FuriganaText, { props: { text: 'お知（し）らせは10時（じ）に始（はじ）まります。' } })
    expect(wrapper.findAll('ruby').map((ruby) => [ruby.find('rt').text(), ruby.element.childNodes[0]?.textContent])).toEqual([
      ['し', '知'], ['じ', '時'], ['はじ', '始'],
    ])
    expect(wrapper.get('rt').attributes('style')).toContain('font-size: 70%')

    setSessionUser({ id: 'user-1', email: 'learner@example.com', role: 'learner', defaultLevelId: null, showFurigana: true, furiganaSize: 85 })
    await wrapper.vm.$nextTick()
    expect(wrapper.get('rt').attributes('style')).toContain('font-size: 85%')

    setSessionUser({ id: 'user-1', email: 'learner@example.com', role: 'learner', defaultLevelId: null, showFurigana: false, furiganaSize: 85 })
    await wrapper.vm.$nextTick()
    expect(wrapper.find('ruby').exists()).toBe(false)
    expect(wrapper.text()).toBe('お知（し）らせは10時（じ）に始（はじ）まります。')
  })
})
