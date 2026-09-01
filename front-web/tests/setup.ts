// Node 运行时的 localStorage 需要额外 flag；测试环境用内存实现替代。
// useAnswerAutosave 的所有 localStorage 访问都有 try/catch 兜底，不会因缺失而崩溃。
class MemoryStorage {
  private map = new Map<string, string>()

  get length(): number {
    return this.map.size
  }

  key(index: number): string | null {
    return [...this.map.keys()][index] ?? null
  }

  getItem(key: string): string | null {
    return this.map.get(key) ?? null
  }

  setItem(key: string, value: string): void {
    this.map.set(key, String(value))
  }

  removeItem(key: string): void {
    this.map.delete(key)
  }

  clear(): void {
    this.map.clear()
  }
}

const storage = new MemoryStorage()
Object.defineProperty(globalThis, 'localStorage', { value: storage, configurable: true })
Object.defineProperty(globalThis.window, 'localStorage', { value: storage, configurable: true })
