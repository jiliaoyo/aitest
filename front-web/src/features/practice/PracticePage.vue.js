import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { request, ApiError } from '@/api/client';
import AppShell from '@/components/AppShell.vue';
import AppStatus from '@/components/AppStatus.vue';
import ConfirmDialog from '@/components/ConfirmDialog.vue';
import QuestionCard from './QuestionCard.vue';
import QuestionNavigator from './QuestionNavigator.vue';
import SaveStatus from './SaveStatus.vue';
import { useAnswerAutosave } from './useAnswerAutosave';
const route = useRoute();
const router = useRouter();
const sessionID = computed(() => route.params.sessionId);
const session = ref(null);
const pageState = ref('loading');
const errorMessage = ref('');
const requestID = ref('');
const currentIndex = ref(0);
const collapsedMaterials = ref(new Set());
const showLocalDraftNote = ref(false);
const confirmOpen = ref(false);
const submitting = ref(false);
const submitError = ref('');
const autosave = useAnswerAutosave(sessionID);
const currentEntry = computed(() => autosave.entryOf(currentItem.value?.id ?? ''));
const currentItem = computed(() => {
    const item = session.value?.items[currentIndex.value];
    return item ?? null;
});
const answeredCount = computed(() => session.value?.items.filter((i) => autosave.entries.get(i.id)?.value != null).length ?? 0);
const markedCount = computed(() => session.value?.items.filter((i) => autosave.entries.get(i.id)?.marked).length ?? 0);
const unansweredCount = computed(() => (session.value?.totalCount ?? 0) - answeredCount.value);
const progressPercent = computed(() => session.value ? (answeredCount.value / session.value.totalCount) : 0);
function isAnswered(item) {
    return autosave.entries.get(item.id)?.value != null;
}
function isMarked(item) {
    return autosave.entries.get(item.id)?.marked ?? false;
}
onMounted(load);
async function load() {
    pageState.value = 'loading';
    try {
        const data = await request(`/practice-sessions/${sessionID.value}`);
        session.value = data;
        showLocalDraftNote.value = autosave.init(data.items);
        pageState.value = 'ready';
    }
    catch (err) {
        if (err instanceof ApiError && err.status === 404) {
            pageState.value = 'notfound';
            return;
        }
        errorMessage.value = err instanceof ApiError ? err.message : '加载失败';
        requestID.value = err instanceof ApiError ? err.requestId ?? '' : '';
        pageState.value = 'error';
    }
}
function setAnswer(value) {
    if (!currentItem.value)
        return;
    autosave.setAnswer(currentItem.value.id, value);
}
function setMarked(marked) {
    if (!currentItem.value)
        return;
    autosave.setMarked(currentItem.value.id, marked);
}
function toggleMaterial() {
    const material = currentItem.value?.material;
    if (!material)
        return;
    const set = collapsedMaterials.value;
    if (set.has(material.id)) {
        set.delete(material.id);
    }
    else {
        set.add(material.id);
    }
}
function materialCollapsed() {
    const material = currentItem.value?.material;
    return material ? collapsedMaterials.value.has(material.id) : false;
}
// ---- 批次提交 ----
function submitKey() {
    const storageKey = `practice-submit-key:${sessionID.value}`;
    let key = localStorage.getItem(storageKey);
    if (!key) {
        key = crypto.randomUUID();
        localStorage.setItem(storageKey, key);
    }
    return key;
}
function clearSubmitKey() {
    localStorage.removeItem(`practice-submit-key:${sessionID.value}`);
}
async function openConfirm() {
    // 停止新的 debounce 并尽力冲刷未保存修改；交卷请求会携带全部最终答案
    await autosave.flushPending(session.value?.items ?? []);
    submitError.value = '';
    confirmOpen.value = true;
}
async function doSubmit() {
    if (!session.value || submitting.value)
        return;
    submitting.value = true;
    submitError.value = '';
    try {
        const result = await request(`/practice-sessions/${sessionID.value}/submit`, {
            method: 'POST',
            body: { answers: autosave.finalAnswers(session.value.items) },
            headers: { 'Idempotency-Key': submitKey() },
        });
        void result;
        autosave.cleanup();
        clearSubmitKey();
        await router.replace(`/practice/${sessionID.value}/result`);
    }
    catch (err) {
        if (err instanceof ApiError && (err.code === 'practice_not_active' || err.code === 'idempotency_conflict')) {
            // 批次已在其他会话提交：直接进入现有结果，不产生第二份成绩
            autosave.cleanup();
            clearSubmitKey();
            await router.replace(`/practice/${sessionID.value}/result`);
            return;
        }
        submitError.value = err instanceof ApiError ? `${err.message}（本批答案已保留，可重试）` : '提交失败，请重试';
        confirmOpen.value = false;
    }
    finally {
        submitting.value = false;
    }
}
onBeforeUnmount(() => {
    for (const entry of autosave.entries.values()) {
        if (entry.timer)
            clearTimeout(entry.timer);
    }
});
debugger; /* PartiallyEnd: #3632/scriptSetup.vue */
const __VLS_ctx = {};
let __VLS_components;
let __VLS_directives;
/** @type {[typeof AppShell, typeof AppShell, ]} */ ;
// @ts-ignore
const __VLS_0 = __VLS_asFunctionalComponent(AppShell, new AppShell({}));
const __VLS_1 = __VLS_0({}, ...__VLS_functionalComponentArgsRest(__VLS_0));
var __VLS_3 = {};
__VLS_2.slots.default;
if (__VLS_ctx.pageState === 'loading') {
    /** @type {[typeof AppStatus, ]} */ ;
    // @ts-ignore
    const __VLS_4 = __VLS_asFunctionalComponent(AppStatus, new AppStatus({
        state: "loading",
    }));
    const __VLS_5 = __VLS_4({
        state: "loading",
    }, ...__VLS_functionalComponentArgsRest(__VLS_4));
}
else if (__VLS_ctx.pageState === 'error') {
    /** @type {[typeof AppStatus, ]} */ ;
    // @ts-ignore
    const __VLS_7 = __VLS_asFunctionalComponent(AppStatus, new AppStatus({
        ...{ 'onAction': {} },
        state: "error",
        message: (__VLS_ctx.errorMessage),
        requestId: (__VLS_ctx.requestID),
    }));
    const __VLS_8 = __VLS_7({
        ...{ 'onAction': {} },
        state: "error",
        message: (__VLS_ctx.errorMessage),
        requestId: (__VLS_ctx.requestID),
    }, ...__VLS_functionalComponentArgsRest(__VLS_7));
    let __VLS_10;
    let __VLS_11;
    let __VLS_12;
    const __VLS_13 = {
        onAction: (__VLS_ctx.load)
    };
    var __VLS_9;
}
else if (__VLS_ctx.pageState === 'notfound') {
    /** @type {[typeof AppStatus, ]} */ ;
    // @ts-ignore
    const __VLS_14 = __VLS_asFunctionalComponent(AppStatus, new AppStatus({
        ...{ 'onAction': {} },
        state: "empty",
        message: "练习不存在，或刷新后已被提交。",
        actionLabel: "返回学习概览",
    }));
    const __VLS_15 = __VLS_14({
        ...{ 'onAction': {} },
        state: "empty",
        message: "练习不存在，或刷新后已被提交。",
        actionLabel: "返回学习概览",
    }, ...__VLS_functionalComponentArgsRest(__VLS_14));
    let __VLS_17;
    let __VLS_18;
    let __VLS_19;
    const __VLS_20 = {
        onAction: (...[$event]) => {
            if (!!(__VLS_ctx.pageState === 'loading'))
                return;
            if (!!(__VLS_ctx.pageState === 'error'))
                return;
            if (!(__VLS_ctx.pageState === 'notfound'))
                return;
            __VLS_ctx.router.push('/');
        }
    };
    var __VLS_16;
}
else if (__VLS_ctx.session) {
    if (__VLS_ctx.session.status !== 'active') {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "card" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.h2, __VLS_intrinsicElements.h2)({});
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "muted" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
            ...{ onClick: (...[$event]) => {
                    if (!!(__VLS_ctx.pageState === 'loading'))
                        return;
                    if (!!(__VLS_ctx.pageState === 'error'))
                        return;
                    if (!!(__VLS_ctx.pageState === 'notfound'))
                        return;
                    if (!(__VLS_ctx.session))
                        return;
                    if (!(__VLS_ctx.session.status !== 'active'))
                        return;
                    __VLS_ctx.router.push(`/practice/${__VLS_ctx.sessionID}/result`);
                } },
            ...{ class: "primary" },
        });
    }
    else {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "page-header" },
            ...{ style: {} },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.h1, __VLS_intrinsicElements.h1)({
            ...{ style: {} },
        });
        /** @type {[typeof SaveStatus, ]} */ ;
        // @ts-ignore
        const __VLS_21 = __VLS_asFunctionalComponent(SaveStatus, new SaveStatus({
            ...{ 'onRetry': {} },
            state: (__VLS_ctx.currentEntry.state),
            savedAt: (__VLS_ctx.currentEntry.savedAt),
            localOnly: (__VLS_ctx.currentEntry.localOnly),
            anyError: (__VLS_ctx.autosave.anyError.value),
        }));
        const __VLS_22 = __VLS_21({
            ...{ 'onRetry': {} },
            state: (__VLS_ctx.currentEntry.state),
            savedAt: (__VLS_ctx.currentEntry.savedAt),
            localOnly: (__VLS_ctx.currentEntry.localOnly),
            anyError: (__VLS_ctx.autosave.anyError.value),
        }, ...__VLS_functionalComponentArgsRest(__VLS_21));
        let __VLS_24;
        let __VLS_25;
        let __VLS_26;
        const __VLS_27 = {
            onRetry: (__VLS_ctx.autosave.retryFailed)
        };
        var __VLS_23;
        if (__VLS_ctx.showLocalDraftNote) {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
                ...{ class: "error-summary" },
                role: "status",
            });
        }
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "progress-bar" },
            role: "img",
            'aria-label': (`已答 ${__VLS_ctx.answeredCount} / ${__VLS_ctx.session.totalCount} 题`),
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.span)({
            ...{ style: ({ width: `${Math.round(__VLS_ctx.progressPercent * 100)}%` }) },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "mono muted" },
            ...{ style: {} },
        });
        (__VLS_ctx.answeredCount);
        (__VLS_ctx.session.totalCount);
        (__VLS_ctx.markedCount);
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "practice-layout" },
            ...{ style: {} },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({});
        if (__VLS_ctx.currentItem) {
            /** @type {[typeof QuestionCard, ]} */ ;
            // @ts-ignore
            const __VLS_28 = __VLS_asFunctionalComponent(QuestionCard, new QuestionCard({
                ...{ 'onUpdate:answer': {} },
                ...{ 'onUpdate:marked': {} },
                ...{ 'onToggleMaterial': {} },
                item: (__VLS_ctx.currentItem),
                answer: (__VLS_ctx.currentEntry.value),
                marked: (__VLS_ctx.currentEntry.marked),
                materialCollapsed: (__VLS_ctx.materialCollapsed()),
            }));
            const __VLS_29 = __VLS_28({
                ...{ 'onUpdate:answer': {} },
                ...{ 'onUpdate:marked': {} },
                ...{ 'onToggleMaterial': {} },
                item: (__VLS_ctx.currentItem),
                answer: (__VLS_ctx.currentEntry.value),
                marked: (__VLS_ctx.currentEntry.marked),
                materialCollapsed: (__VLS_ctx.materialCollapsed()),
            }, ...__VLS_functionalComponentArgsRest(__VLS_28));
            let __VLS_31;
            let __VLS_32;
            let __VLS_33;
            const __VLS_34 = {
                'onUpdate:answer': (__VLS_ctx.setAnswer)
            };
            const __VLS_35 = {
                'onUpdate:marked': (__VLS_ctx.setMarked)
            };
            const __VLS_36 = {
                onToggleMaterial: (__VLS_ctx.toggleMaterial)
            };
            var __VLS_30;
        }
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ style: {} },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
            ...{ onClick: (...[$event]) => {
                    if (!!(__VLS_ctx.pageState === 'loading'))
                        return;
                    if (!!(__VLS_ctx.pageState === 'error'))
                        return;
                    if (!!(__VLS_ctx.pageState === 'notfound'))
                        return;
                    if (!(__VLS_ctx.session))
                        return;
                    if (!!(__VLS_ctx.session.status !== 'active'))
                        return;
                    __VLS_ctx.currentIndex--;
                } },
            type: "button",
            disabled: (__VLS_ctx.currentIndex === 0),
        });
        if (__VLS_ctx.currentIndex < __VLS_ctx.session.items.length - 1) {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
                ...{ onClick: (...[$event]) => {
                        if (!!(__VLS_ctx.pageState === 'loading'))
                            return;
                        if (!!(__VLS_ctx.pageState === 'error'))
                            return;
                        if (!!(__VLS_ctx.pageState === 'notfound'))
                            return;
                        if (!(__VLS_ctx.session))
                            return;
                        if (!!(__VLS_ctx.session.status !== 'active'))
                            return;
                        if (!(__VLS_ctx.currentIndex < __VLS_ctx.session.items.length - 1))
                            return;
                        __VLS_ctx.currentIndex++;
                    } },
                type: "button",
                ...{ class: "primary" },
            });
        }
        __VLS_asFunctionalElement(__VLS_intrinsicElements.aside, __VLS_intrinsicElements.aside)({
            ...{ class: "practice-side" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "card" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.h2, __VLS_intrinsicElements.h2)({
            ...{ style: {} },
        });
        /** @type {[typeof QuestionNavigator, ]} */ ;
        // @ts-ignore
        const __VLS_37 = __VLS_asFunctionalComponent(QuestionNavigator, new QuestionNavigator({
            ...{ 'onSelect': {} },
            items: (__VLS_ctx.session.items),
            currentIndex: (__VLS_ctx.currentIndex),
            isAnswered: (__VLS_ctx.isAnswered),
            isMarked: (__VLS_ctx.isMarked),
        }));
        const __VLS_38 = __VLS_37({
            ...{ 'onSelect': {} },
            items: (__VLS_ctx.session.items),
            currentIndex: (__VLS_ctx.currentIndex),
            isAnswered: (__VLS_ctx.isAnswered),
            isMarked: (__VLS_ctx.isMarked),
        }, ...__VLS_functionalComponentArgsRest(__VLS_37));
        let __VLS_40;
        let __VLS_41;
        let __VLS_42;
        const __VLS_43 = {
            onSelect: (...[$event]) => {
                if (!!(__VLS_ctx.pageState === 'loading'))
                    return;
                if (!!(__VLS_ctx.pageState === 'error'))
                    return;
                if (!!(__VLS_ctx.pageState === 'notfound'))
                    return;
                if (!(__VLS_ctx.session))
                    return;
                if (!!(__VLS_ctx.session.status !== 'active'))
                    return;
                __VLS_ctx.currentIndex = $event;
            }
        };
        var __VLS_39;
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "card" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "muted" },
            ...{ style: {} },
        });
        (__VLS_ctx.unansweredCount);
        __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
            ...{ onClick: (__VLS_ctx.openConfirm) },
            ...{ class: "primary" },
            ...{ style: {} },
        });
        /** @type {[typeof ConfirmDialog, typeof ConfirmDialog, ]} */ ;
        // @ts-ignore
        const __VLS_44 = __VLS_asFunctionalComponent(ConfirmDialog, new ConfirmDialog({
            ...{ 'onConfirm': {} },
            ...{ 'onCancel': {} },
            open: (__VLS_ctx.confirmOpen),
            title: "确认提交本批练习？",
            confirmLabel: "确认提交",
            cancelLabel: "继续答题",
        }));
        const __VLS_45 = __VLS_44({
            ...{ 'onConfirm': {} },
            ...{ 'onCancel': {} },
            open: (__VLS_ctx.confirmOpen),
            title: "确认提交本批练习？",
            confirmLabel: "确认提交",
            cancelLabel: "继续答题",
        }, ...__VLS_functionalComponentArgsRest(__VLS_44));
        let __VLS_47;
        let __VLS_48;
        let __VLS_49;
        const __VLS_50 = {
            onConfirm: (__VLS_ctx.doSubmit)
        };
        const __VLS_51 = {
            onCancel: (...[$event]) => {
                if (!!(__VLS_ctx.pageState === 'loading'))
                    return;
                if (!!(__VLS_ctx.pageState === 'error'))
                    return;
                if (!!(__VLS_ctx.pageState === 'notfound'))
                    return;
                if (!(__VLS_ctx.session))
                    return;
                if (!!(__VLS_ctx.session.status !== 'active'))
                    return;
                __VLS_ctx.confirmOpen = false;
            }
        };
        __VLS_46.slots.default;
        if (__VLS_ctx.unansweredCount > 0) {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({});
            __VLS_asFunctionalElement(__VLS_intrinsicElements.strong, __VLS_intrinsicElements.strong)({});
            (__VLS_ctx.unansweredCount);
            (__VLS_ctx.markedCount > 0 ? `、${__VLS_ctx.markedCount} 题已标记待检查` : '');
        }
        else {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({});
        }
        if (__VLS_ctx.submitError) {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
                ...{ class: "error-summary" },
                role: "alert",
            });
            (__VLS_ctx.submitError);
        }
        var __VLS_46;
    }
}
var __VLS_2;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
/** @type {__VLS_StyleScopedClasses['page-header']} */ ;
/** @type {__VLS_StyleScopedClasses['error-summary']} */ ;
/** @type {__VLS_StyleScopedClasses['progress-bar']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['practice-layout']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
/** @type {__VLS_StyleScopedClasses['practice-side']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
/** @type {__VLS_StyleScopedClasses['error-summary']} */ ;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            AppShell: AppShell,
            AppStatus: AppStatus,
            ConfirmDialog: ConfirmDialog,
            QuestionCard: QuestionCard,
            QuestionNavigator: QuestionNavigator,
            SaveStatus: SaveStatus,
            router: router,
            sessionID: sessionID,
            session: session,
            pageState: pageState,
            errorMessage: errorMessage,
            requestID: requestID,
            currentIndex: currentIndex,
            showLocalDraftNote: showLocalDraftNote,
            confirmOpen: confirmOpen,
            submitError: submitError,
            autosave: autosave,
            currentEntry: currentEntry,
            currentItem: currentItem,
            answeredCount: answeredCount,
            markedCount: markedCount,
            unansweredCount: unansweredCount,
            progressPercent: progressPercent,
            isAnswered: isAnswered,
            isMarked: isMarked,
            load: load,
            setAnswer: setAnswer,
            setMarked: setMarked,
            toggleMaterial: toggleMaterial,
            materialCollapsed: materialCollapsed,
            openConfirm: openConfirm,
            doSubmit: doSubmit,
        };
    },
});
export default (await import('vue')).defineComponent({
    setup() {
        return {};
    },
});
; /* PartiallyEnd: #4569/main.vue */
