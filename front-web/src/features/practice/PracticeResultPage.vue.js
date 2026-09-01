import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';
import { request, ApiError } from '@/api/client';
import AppShell from '@/components/AppShell.vue';
import AppStatus from '@/components/AppStatus.vue';
import StatusBadge from '@/components/StatusBadge.vue';
import { formatDateTime, formatPercent } from '@/app/format';
import ResultItem from './ResultItem.vue';
const route = useRoute();
const sessionID = computed(() => route.params.sessionId);
const result = ref(null);
const pageState = ref('loading');
const errorMessage = ref('');
const requestID = ref('');
let timer = null;
async function load(silent = false) {
    if (!silent)
        pageState.value = 'loading';
    try {
        result.value = await request(`/practice-sessions/${sessionID.value}/result`);
        pageState.value = 'ready';
        schedulePolling();
    }
    catch (err) {
        if (err instanceof ApiError && err.status === 404) {
            pageState.value = 'notfound';
            return;
        }
        if (err instanceof ApiError && err.code === 'practice_not_submitted') {
            await routeReplacePractice();
            return;
        }
        errorMessage.value = err instanceof ApiError ? err.message : '加载失败';
        requestID.value = err instanceof ApiError ? err.requestId ?? '' : '';
        pageState.value = 'error';
    }
}
async function routeReplacePractice() {
    window.location.assign(`/practice/${sessionID.value}`);
}
// 批次为 grading 时每 3 秒轮询；页面不可见时暂停，恢复可见后立即请求一次。
function schedulePolling() {
    const grading = result.value?.status === 'grading';
    if (grading && timer === null) {
        timer = setInterval(() => {
            if (!document.hidden) {
                void load(true);
            }
        }, 3000);
    }
    if (!grading && timer !== null) {
        clearInterval(timer);
        timer = null;
    }
}
function onVisibility() {
    if (!document.hidden && result.value?.status === 'grading') {
        void load(true);
    }
}
onMounted(() => {
    void load();
    document.addEventListener('visibilitychange', onVisibility);
});
onBeforeUnmount(() => {
    if (timer)
        clearInterval(timer);
    document.removeEventListener('visibilitychange', onVisibility);
});
const summary = computed(() => result.value?.summary);
const confirmedAccuracy = computed(() => summary.value?.confirmed.accuracy ?? null);
const aiDone = computed(() => (summary.value?.ai.completed ?? 0) + (summary.value?.ai.pending ?? 0) + (summary.value?.ai.failed ?? 0));
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
        onAction: (...[$event]) => {
            if (!!(__VLS_ctx.pageState === 'loading'))
                return;
            if (!(__VLS_ctx.pageState === 'error'))
                return;
            __VLS_ctx.load();
        }
    };
    var __VLS_9;
}
else if (__VLS_ctx.pageState === 'notfound') {
    /** @type {[typeof AppStatus, ]} */ ;
    // @ts-ignore
    const __VLS_14 = __VLS_asFunctionalComponent(AppStatus, new AppStatus({
        ...{ 'onAction': {} },
        state: "empty",
        message: "练习不存在，或它不属于当前账号。",
        actionLabel: "返回学习概览",
    }));
    const __VLS_15 = __VLS_14({
        ...{ 'onAction': {} },
        state: "empty",
        message: "练习不存在，或它不属于当前账号。",
        actionLabel: "返回学习概览",
    }, ...__VLS_functionalComponentArgsRest(__VLS_14));
    let __VLS_17;
    let __VLS_18;
    let __VLS_19;
    const __VLS_20 = {
        onAction: (() => __VLS_ctx.$router.push('/'))
    };
    var __VLS_16;
}
else if (__VLS_ctx.result) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "page-header" },
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.h1, __VLS_intrinsicElements.h1)({
        ...{ style: {} },
    });
    /** @type {[typeof StatusBadge, ]} */ ;
    // @ts-ignore
    const __VLS_21 = __VLS_asFunctionalComponent(StatusBadge, new StatusBadge({
        value: (__VLS_ctx.result.status),
        kind: "session",
    }));
    const __VLS_22 = __VLS_21({
        value: (__VLS_ctx.result.status),
        kind: "session",
    }, ...__VLS_functionalComponentArgsRest(__VLS_21));
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "muted mono" },
    });
    (__VLS_ctx.formatDateTime(__VLS_ctx.result.submittedAt));
    if (__VLS_ctx.result.status === 'grading') {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "card" },
            role: "status",
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({});
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "metrics" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "metric" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "value" },
    });
    (__VLS_ctx.summary?.confirmed.correct ?? 0);
    (__VLS_ctx.summary?.confirmed.total ?? 0);
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "label" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "muted mono" },
    });
    (__VLS_ctx.formatPercent(__VLS_ctx.confirmedAccuracy));
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "metric" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "value" },
    });
    (__VLS_ctx.summary?.ai.completed ?? 0);
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "label" },
    });
    if (__VLS_ctx.aiDone > 0) {
        (__VLS_ctx.aiDone);
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "muted mono" },
    });
    (__VLS_ctx.summary?.ai.correct ?? 0);
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "metric" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "value" },
    });
    ((__VLS_ctx.summary?.ai.pending ?? 0) + (__VLS_ctx.summary?.ai.failed ?? 0));
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "label" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "muted mono" },
    });
    (__VLS_ctx.summary?.ai.pending ?? 0);
    (__VLS_ctx.summary?.ai.failed ?? 0);
    __VLS_asFunctionalElement(__VLS_intrinsicElements.section, __VLS_intrinsicElements.section)({
        'aria-label': "逐题解析",
        ...{ style: {} },
    });
    for (const [item] of __VLS_getVForSourceType((__VLS_ctx.result.items))) {
        /** @type {[typeof ResultItem, ]} */ ;
        // @ts-ignore
        const __VLS_24 = __VLS_asFunctionalComponent(ResultItem, new ResultItem({
            key: (item.id),
            item: (item),
        }));
        const __VLS_25 = __VLS_24({
            key: (item.id),
            item: (item),
        }, ...__VLS_functionalComponentArgsRest(__VLS_24));
    }
}
var __VLS_2;
/** @type {__VLS_StyleScopedClasses['page-header']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['metrics']} */ ;
/** @type {__VLS_StyleScopedClasses['metric']} */ ;
/** @type {__VLS_StyleScopedClasses['value']} */ ;
/** @type {__VLS_StyleScopedClasses['label']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
/** @type {__VLS_StyleScopedClasses['metric']} */ ;
/** @type {__VLS_StyleScopedClasses['value']} */ ;
/** @type {__VLS_StyleScopedClasses['label']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
/** @type {__VLS_StyleScopedClasses['metric']} */ ;
/** @type {__VLS_StyleScopedClasses['value']} */ ;
/** @type {__VLS_StyleScopedClasses['label']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            AppShell: AppShell,
            AppStatus: AppStatus,
            StatusBadge: StatusBadge,
            formatDateTime: formatDateTime,
            formatPercent: formatPercent,
            ResultItem: ResultItem,
            result: result,
            pageState: pageState,
            errorMessage: errorMessage,
            requestID: requestID,
            load: load,
            summary: summary,
            confirmedAccuracy: confirmedAccuracy,
            aiDone: aiDone,
        };
    },
});
export default (await import('vue')).defineComponent({
    setup() {
        return {};
    },
});
; /* PartiallyEnd: #4569/main.vue */
