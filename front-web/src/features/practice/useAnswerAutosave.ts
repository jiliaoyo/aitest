import { reactive, ref, type Ref } from 'vue'
import { request } from '@/api/client'
import type { AnswerValue, PreSubmitItem } from '@/api/types'

export type SaveState = 'idle' | 'saving' | 'saved' | 'error'

interface AnswerEntry {
  sessionID: string
  value: AnswerValue
  marked: boolean
  state: SaveState
  savedAt: string | null
  dirty: boolean
  timer: ReturnType<typeof setTimeout> | null
  inFlight: boolean
  localOnly: boolean
  revision: number
  active: boolean
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

  function draftKey(targetSessionID = sessionId.value): string {
    return `practice-draft:${targetSessionID}`
  }

  function readDraft(targetSessionID = sessionId.value): Record<string, { value: AnswerValue; marked: boolean }> {
    try {
      const raw = localStorage.getItem(draftKey(targetSessionID))
      return raw ? (JSON.parse(raw) as Record<string, { value: AnswerValue; marked: boolean }>) : {}
    } catch {
      return {}
    }
  }

  function writeDraft(targetSessionID: string, itemID: string, value: AnswerValue, marked: boolean): void {
    try {
      const draft = readDraft(targetSessionID)
      draft[itemID] = { value, marked }
      localStorage.setItem(draftKey(targetSessionID), JSON.stringify(draft))
    } catch {
      // 隐私模式下 localStorage 不可用：忽略，答案仍保留在内存中
    }
  }

  function clearDraftItem(targetSessionID: string, itemID: string, value: AnswerValue, marked: boolean): void {
    try {
      const draft = readDraft(targetSessionID)
      if (JSON.stringify(draft[itemID]) !== JSON.stringify({ value, marked })) return
      delete draft[itemID]
      if (Object.keys(draft).length === 0) {
        localStorage.removeItem(draftKey(targetSessionID))
      } else {
        localStorage.setItem(draftKey(targetSessionID), JSON.stringify(draft))
      }
    } catch {
      // 忽略
    }
  }

  /** 用服务端数据初始化；有本地未同步草稿时本地值优先并提示。 */
  function init(items: PreSubmitItem[], restoreDraft = true): boolean {
    disposeEntries()
    entries.clear()
    anyError.value = false
    const targetSessionID = sessionId.value
    if (!restoreDraft) {
      removeDraft(targetSessionID)
    }
    const draft = restoreDraft ? readDraft(targetSessionID) : {}
    let hasLocal = false
    for (const item of items) {
      const local = draft[item.id]
      if (local) {
        hasLocal = true
        entries.set(item.id, {
          sessionID: targetSessionID,
          value: local.value,
          marked: local.marked,
          state: 'idle',
          savedAt: null,
          dirty: true,
          timer: null,
          inFlight: false,
          localOnly: true,
          revision: 0,
          active: true,
        })
        continue
      }
      entries.set(item.id, {
        sessionID: targetSessionID,
        value: item.savedAnswer,
        marked: item.markedForReview,
        state: 'idle',
        savedAt: item.savedAt,
        dirty: false,
        timer: null,
        inFlight: false,
        localOnly: false,
        revision: 0,
        active: true,
      })
    }
    return hasLocal
  }

  function entryOf(itemID: string): AnswerEntry {
    let entry = entries.get(itemID)
    if (!entry) {
      entry = {
        sessionID: sessionId.value, value: null, marked: false, state: 'idle', savedAt: null,
        dirty: false, timer: null, inFlight: false, localOnly: false, revision: 0, active: true,
      }
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
    entry.revision++
    entry.dirty = true
    entry.localOnly = true
    writeDraft(entry.sessionID, itemID, entry.value, entry.marked)
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
    const revision = entry.revision
    const payload = { value: entry.value, markedForReview: entry.marked }
    try {
      const res = await request<{ savedAt: string }>(
        `/practice-sessions/${entry.sessionID}/answers/${itemID}`,
        { method: 'PUT', body: payload },
      )
      if (revision === entry.revision) {
        entry.state = 'saved'
        entry.savedAt = res.savedAt
        entry.localOnly = false
        clearDraftItem(entry.sessionID, itemID, payload.value, payload.markedForReview)
      }
    } catch {
      if (entry.active) {
        entry.dirty = true
        entry.state = 'error'
      }
    } finally {
      entry.inFlight = false
      anyError.value = [...entries.values()].some((e) => e.state === 'error')
    }
    // 保存期间又有修改时只补存最新值；没有新修改的失败等待用户重试。
    if (entry.active && entry.dirty && revision !== entry.revision) {
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

  /** 提交前取消 debounce 并发起保存；提交请求自带全部答案，因此调用方无需等待。 */
  function flushPending(items: PreSubmitItem[]): void {
    for (const entry of entries.values()) {
      if (entry.timer) {
        clearTimeout(entry.timer)
        entry.timer = null
      }
    }
    const dirty = items.filter((i) => entries.get(i.id)?.dirty)
    for (const item of dirty) void save(item.id)
  }

  function cleanup(): void {
    disposeEntries()
    removeDraft(sessionId.value)
  }

  function removeDraft(targetSessionID: string): void {
    try {
      localStorage.removeItem(draftKey(targetSessionID))
    } catch {
      // 忽略
    }
  }

  function disposeEntries(): void {
    for (const entry of entries.values()) {
      entry.active = false
      if (entry.timer) clearTimeout(entry.timer)
      entry.timer = null
    }
  }

  function dispose(): void {
    disposeEntries()
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
    dispose,
    entryOf,
  }
}
