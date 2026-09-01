import { reactive, ref } from 'vue';
import { request } from '@/api/client';
const DEBOUNCE_MS = 500;
/**
 * 自动保存：同一题只允许一个请求进行中；请求期间再次修改标记 dirty，
 * 完成后立即保存最新值，避免旧请求覆盖新答案（规范 §11.3）。
 * 保存失败时把最新答案写入本地草稿，成功后清理。
 */
export function useAnswerAutosave(sessionId) {
    const entries = reactive(new Map());
    const anyError = ref(false);
    function draftKey() {
        return `practice-draft:${sessionId.value}`;
    }
    function readDraft() {
        try {
            const raw = localStorage.getItem(draftKey());
            return raw ? JSON.parse(raw) : {};
        }
        catch {
            return {};
        }
    }
    function writeDraft(itemID, value, marked) {
        try {
            const draft = readDraft();
            draft[itemID] = { value, marked };
            localStorage.setItem(draftKey(), JSON.stringify(draft));
        }
        catch {
            // 隐私模式下 localStorage 不可用：忽略，答案仍保留在内存中
        }
    }
    function clearDraftItem(itemID) {
        try {
            const draft = readDraft();
            delete draft[itemID];
            if (Object.keys(draft).length === 0) {
                localStorage.removeItem(draftKey());
            }
            else {
                localStorage.setItem(draftKey(), JSON.stringify(draft));
            }
        }
        catch {
            // 忽略
        }
    }
    /** 用服务端数据初始化；有本地未同步草稿时本地值优先并提示。 */
    function init(items) {
        const draft = readDraft();
        let hasLocal = false;
        for (const item of items) {
            const local = draft[item.id];
            if (local) {
                hasLocal = true;
                entries.set(item.id, {
                    value: local.value,
                    marked: local.marked,
                    state: 'idle',
                    savedAt: null,
                    dirty: true,
                    timer: null,
                    inFlight: false,
                    localOnly: true,
                });
                continue;
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
            });
        }
        return hasLocal;
    }
    function entryOf(itemID) {
        let entry = entries.get(itemID);
        if (!entry) {
            entry = { value: null, marked: false, state: 'idle', savedAt: null, dirty: false, timer: null, inFlight: false, localOnly: false };
            entries.set(itemID, entry);
        }
        return entry;
    }
    function setAnswer(itemID, value) {
        const entry = entryOf(itemID);
        entry.value = value;
        schedule(itemID);
    }
    function setMarked(itemID, marked) {
        const entry = entryOf(itemID);
        entry.marked = marked;
        schedule(itemID);
    }
    function schedule(itemID) {
        const entry = entryOf(itemID);
        entry.dirty = true;
        if (entry.inFlight) {
            return; // 当前请求完成后立即补一次保存
        }
        if (entry.timer) {
            clearTimeout(entry.timer);
        }
        entry.timer = setTimeout(() => {
            entry.timer = null;
            void save(itemID);
        }, DEBOUNCE_MS);
    }
    async function save(itemID) {
        const entry = entryOf(itemID);
        if (entry.inFlight || !entry.dirty) {
            return;
        }
        entry.inFlight = true;
        entry.state = 'saving';
        entry.dirty = false;
        const payload = { value: entry.value, markedForReview: entry.marked };
        try {
            const res = await request(`/practice-sessions/${sessionId.value}/answers/${itemID}`, { method: 'PUT', body: payload });
            entry.state = 'saved';
            entry.savedAt = res.savedAt;
            entry.localOnly = false;
            clearDraftItem(itemID);
            anyError.value = [...entries.values()].some((e) => e.state === 'error');
        }
        catch {
            entry.dirty = true;
            entry.state = 'error';
            anyError.value = true;
            writeDraft(itemID, entry.value, entry.marked);
        }
        finally {
            entry.inFlight = false;
        }
        // 保存期间答案又被修改：在 inFlight 复位后立即补存最新值
        if (entry.dirty && entry.state === 'saved') {
            void save(itemID);
        }
    }
    async function retryFailed() {
        const failed = [...entries.entries()].filter(([, e]) => e.state === 'error').map(([id]) => id);
        await Promise.all(failed.map((id) => save(id)));
    }
    /** 提交前读取界面中的全部最终答案。 */
    function finalAnswers(items) {
        return items.map((item) => {
            const entry = entries.get(item.id);
            return {
                itemId: item.id,
                value: entry?.value ?? null,
                markedForReview: entry?.marked ?? false,
            };
        });
    }
    /** 提交前尽力冲刷尚未保存的修改（不阻塞提交：提交请求自带全部答案）。 */
    async function flushPending(items) {
        for (const entry of entries.values()) {
            if (entry.timer) {
                clearTimeout(entry.timer);
                entry.timer = null;
            }
        }
        const dirty = items.filter((i) => entries.get(i.id)?.dirty);
        await Promise.all(dirty.map((i) => save(i.id)));
    }
    function cleanup() {
        try {
            localStorage.removeItem(`practice-draft:${sessionId.value}`);
        }
        catch {
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
    };
}
