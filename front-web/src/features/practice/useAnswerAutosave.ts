import { reactive, ref, type Ref } from 'vue'
import { request } from '@/api/client'
import type { AnswerValue, PreSubmitItem } from '@/api/types'

export type SaveState = 'idle' | 'saving' | 'saved' | 'error'

interface AnswerEntry {
  value: AnswerValue
  marked: boolean
  state: SaveState
  savedAt: string | null
  dirty: boolean
  timer: ReturnType<typeof setTimeout> | null
  inFlight: boolean
  localOnly: boolean
}

const DEBOUNCE_MS = 500

/**
 * 自动保存：同一题只允许一个请求进行中；请求期间再次修改标记 dirty，
 * 完成后立即保存最新值，避免旧请求覆盖新答案（规范 §11.3）。
 * 保存失败时把最新答案写入本地草稿，成功后清理。
 */
export function useAnswerAutosave(sessionId: Ref<string>) {
  const entries = reactive(new Map<string, AnswerEntry>())
  const anyError = ref(false)

  function draftKey(): string {
    return `practice-draft:${sessionId.value}`
  }

  function readDraft(): Record<string, { value: AnswerValue; marked: boolean }> {
    try {
      const raw = localStorage.getItem(draftKey())
      return raw ? (JSON.parse(raw) as Record<string, { value: AnswerValue; marked: boolean }>) : {}
    } catch {
      return {}
    }
  }

  function writeDraft(itemID: string, value: AnswerValue, marked: boolean): void {
    try {
      const draft = readDraft()
      draft[itemID] = { value, marked }
      localStorage.setItem(draftKey(), JSON.stringify(draft))
    } catch {
      // 隐私模式下 localStorage 不可用：忽略，答案仍保留在内存中
    }
  }

  function clearDraftItem(itemID: string): void {
    try {
      const draft = readDraft()
      delete draft[itemID]
      if (Object.keys(draft).length === 0) {
        localStorage.removeItem(draftKey())
      } else {
        localStorage.setItem(draftKey(), JSON.stringify(draft))
      }
    } catch {
      // 忽略
    }
  }

  /** 用服务端数据初始化；有本地未同步草稿时本地值优先并提示。 */
  function init(items: PreSubmitItem[]): boolean {
    const draft = readDraft()
    let hasLocal = false
    for (const item of items) {
      const local = draft[item.id]
      if (local) {
        hasLocal = true
        entries.set(item.id, {
          value: local.value,
          marked: local.marked,
          state: 'idle',
          savedAt: null,
          dirty: true,
          timer: null,
          inFlight: false,
          localOnly: true,
        })
        continue
      }
      entries.set(item.id, {
        value: item.savedAnswer,
        marked: item.markedForReview,
        state: 'idle',
        savedAt: item.savedAt,
        dirty: false,
        timer: null,
        inFlight: false,
        localOnly: false,
      })
    }
    return hasLocal
  }

  function entryOf(itemID: string): AnswerEntry {
    let entry = entries.get(itemID)
    if (!entry) {
      entry = { value: null, marked: false, state: 'idle', savedAt: null, dirty: false, timer: null, inFlight: false, localOnly: false }
      entries.set(itemID, entry)
    }
    return entry
  }

  function setAnswer(itemID: string, value: AnswerValue): void {
    const entry = entryOf(itemID)
    entry.value = value
    schedule(itemID)
  }

  function setMarked(itemID: string, marked: boolean): void {
    const entry = entryOf(itemID)
    entry.marked = marked
    schedule(itemID)
  }

  function schedule(itemID: string): void {
    const entry = entryOf(itemID)
    entry.dirty = true
    if (entry.inFlight) {
      return // 当前请求完成后立即补一次保存
    }
    if (entry.timer) {
      clearTimeout(entry.timer)
    }
    entry.timer = setTimeout(() => {
      entry.timer = null
      void save(itemID)
    }, DEBOUNCE_MS)
  }

  async function save(itemID: string): Promise<void> {
    const entry = entryOf(itemID)
    if (entry.inFlight || !entry.dirty) {
      return
    }
    entry.inFlight = true
    entry.state = 'saving'
    entry.dirty = false
    const payload = { value: entry.value, markedForReview: entry.marked }
    try {
      const res = await request<{ savedAt: string }>(
        `/practice-sessions/${sessionId.value}/answers/${itemID}`,
        { method: 'PUT', body: payload },
      )
      entry.state = 'saved'
      entry.savedAt = res.savedAt
      entry.localOnly = false
      clearDraftItem(itemID)
      anyError.value = [...entries.values()].some((e) => e.state === 'error')
    } catch {
      entry.dirty = true
      entry.state = 'error'
      anyError.value = true
      writeDraft(itemID, entry.value, entry.marked)
    } finally {
      entry.inFlight = false
    }
    // 保存期间答案又被修改：在 inFlight 复位后立即补存最新值
    if (entry.dirty && entry.state === 'saved') {
      void save(itemID)
    }
  }

  async function retryFailed(): Promise<void> {
    const failed = [...entries.entries()].filter(([, e]) => e.state === 'error').map(([id]) => id)
    await Promise.all(failed.map((id) => save(id)))
  }

  /** 提交前读取界面中的全部最终答案。 */
  function finalAnswers(items: PreSubmitItem[]): Array<{ itemId: string; value: AnswerValue; markedForReview: boolean }> {
    return items.map((item) => {
      const entry = entries.get(item.id)
      return {
        itemId: item.id,
        value: entry?.value ?? null,
        markedForReview: entry?.marked ?? false,
      }
    })
  }

  /** 提交前尽力冲刷尚未保存的修改（不阻塞提交：提交请求自带全部答案）。 */
  async function flushPending(items: PreSubmitItem[]): Promise<void> {
    for (const entry of entries.values()) {
      if (entry.timer) {
        clearTimeout(entry.timer)
        entry.timer = null
      }
    }
    const dirty = items.filter((i) => entries.get(i.id)?.dirty)
    await Promise.all(dirty.map((i) => save(i.id)))
  }

  function cleanup(): void {
    try {
      localStorage.removeItem(`practice-draft:${sessionId.value}`)
    } catch {
      // 忽略
    }
  }

  return {
    entries,
    anyError,
    init,
    setAnswer,
    setMarked,
    retryFailed,
    finalAnswers,
    flushPending,
    cleanup,
    entryOf,
  }
}
