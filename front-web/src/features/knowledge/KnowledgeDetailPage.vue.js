import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { request, ApiError } from '@/api/client';
import AppShell from '@/components/AppShell.vue';
import AppStatus from '@/components/AppStatus.vue';
import { formatPercent, formatDateTime } from '@/app/format';
const route = useRoute();
const router = useRouter();
const detail = ref(null);
const state = ref('loading');
const errorMessage = ref('');
const requestID = ref('');
const creating = ref(false);
async function load() {
    state.value = 'loading';
    try {
        detail.value = await request(`/knowledge-points/${route.params.knowledgePointId}`);
        state.value = 'ready';
    }
    catch (err) {
        if (err instanceof ApiError && err.status === 404) {
            state.value = 'notfound';
            return;
        }
        errorMessage.value = err instanceof ApiError ? err.message : '加载失败';
        requestID.value = err instanceof ApiError ? err.requestId ?? '' : '';
        state.value = 'error';
    }
}
onMounted(load);
const accuracy = computed(() => {
    const s = detail.value?.stats;
    if (!s || s.confirmedAnswered === 0)
        return null;
    return s.confirmedCorrect / s.confirmedAnswered;
});
async function startPractice() {
    if (!detail.value)
        return;
    creating.value = true;
    try {
        const session = await request('/practice-sessions', {
            method: 'POST',
            body: {
                levelId: detail.value.levelId,
                subjectId: detail.value.subjectId,
                mode: 'knowledge',
                knowledgePointIds: [detail.value.id],
                count: 10,
            },
        });
        await router.push(`/practice/${session.id}`);
    }
    catch (err) {
        errorMessage.value = err instanceof ApiError ? err.message : '创建练习失败';
    }
    finally {
        creating.value = false;
    }
}
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
if (__VLS_ctx.state === 'loading') {
    /** @type {[typeof AppStatus, ]} */ ;
    // @ts-ignore
    const __VLS_4 = __VLS_asFunctionalComponent(AppStatus, new AppStatus({
        state: "loading",
    }));
    const __VLS_5 = __VLS_4({
        state: "loading",
    }, ...__VLS_functionalComponentArgsRest(__VLS_4));
}
else if (__VLS_ctx.state === 'error') {
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
else if (__VLS_ctx.state === 'notfound' || !__VLS_ctx.detail) {
    /** @type {[typeof AppStatus, ]} */ ;
    // @ts-ignore
    const __VLS_14 = __VLS_asFunctionalComponent(AppStatus, new AppStatus({
        ...{ 'onAction': {} },
        state: "empty",
        message: "知识点不存在或尚未发布。",
        actionLabel: "返回知识点列表",
    }));
    const __VLS_15 = __VLS_14({
        ...{ 'onAction': {} },
        state: "empty",
        message: "知识点不存在或尚未发布。",
        actionLabel: "返回知识点列表",
    }, ...__VLS_functionalComponentArgsRest(__VLS_14));
    let __VLS_17;
    let __VLS_18;
    let __VLS_19;
    const __VLS_20 = {
        onAction: (...[$event]) => {
            if (!!(__VLS_ctx.state === 'loading'))
                return;
            if (!!(__VLS_ctx.state === 'error'))
                return;
            if (!(__VLS_ctx.state === 'notfound' || !__VLS_ctx.detail))
                return;
            __VLS_ctx.router.push('/knowledge');
        }
    };
    var __VLS_16;
}
else {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "page-header" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({});
    __VLS_asFunctionalElement(__VLS_intrinsicElements.h1, __VLS_intrinsicElements.h1)({
        ...{ style: {} },
    });
    (__VLS_ctx.detail.name);
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "muted" },
    });
    (__VLS_ctx.detail.levelCode);
    (__VLS_ctx.detail.subjectName);
    (__VLS_ctx.detail.questionCount);
    __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
        ...{ onClick: (__VLS_ctx.startPractice) },
        ...{ class: "primary" },
        disabled: (__VLS_ctx.creating || __VLS_ctx.detail.questionCount === 0),
    });
    (__VLS_ctx.creating ? '创建中…' : '专项练习 10 题');
    if (__VLS_ctx.detail.questionCount === 0) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "card" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "muted" },
        });
    }
    if (__VLS_ctx.detail.description || __VLS_ctx.detail.commonMistakes || __VLS_ctx.detail.examples) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "card" },
        });
        if (__VLS_ctx.detail.description) {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.section, __VLS_intrinsicElements.section)({});
            __VLS_asFunctionalElement(__VLS_intrinsicElements.h2, __VLS_intrinsicElements.h2)({
                ...{ style: {} },
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
                ...{ style: {} },
            });
            (__VLS_ctx.detail.description);
        }
        if (__VLS_ctx.detail.commonMistakes) {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.section, __VLS_intrinsicElements.section)({
                ...{ style: {} },
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.h2, __VLS_intrinsicElements.h2)({
                ...{ style: {} },
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
                ...{ style: {} },
            });
            (__VLS_ctx.detail.commonMistakes);
        }
        if (__VLS_ctx.detail.examples) {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.section, __VLS_intrinsicElements.section)({
                ...{ style: {} },
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.h2, __VLS_intrinsicElements.h2)({
                ...{ style: {} },
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
                ...{ class: "material-text" },
                lang: "ja",
                ...{ style: {} },
            });
            (__VLS_ctx.detail.examples);
        }
    }
    else {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "card" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "muted" },
        });
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
    (__VLS_ctx.detail.stats?.confirmedAnswered ?? 0);
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "label" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "metric" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "value" },
    });
    (__VLS_ctx.formatPercent(__VLS_ctx.accuracy));
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "label" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "metric" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "value" },
    });
    (__VLS_ctx.detail.stats?.consecutiveWrong ?? 0);
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "label" },
    });
    if (__VLS_ctx.detail.stats) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "muted mono" },
        });
        (__VLS_ctx.detail.stats.recentAnswered);
        (__VLS_ctx.detail.stats.lastPracticedAt ? __VLS_ctx.formatDateTime(__VLS_ctx.detail.stats.lastPracticedAt) : '—');
        if (__VLS_ctx.detail.stats.aiAnswered > 0) {
            (__VLS_ctx.detail.stats.aiAnswered);
            (__VLS_ctx.detail.stats.aiCorrect);
        }
    }
    else {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "muted" },
        });
    }
}
var __VLS_2;
/** @type {__VLS_StyleScopedClasses['page-header']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['material-text']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['metrics']} */ ;
/** @type {__VLS_StyleScopedClasses['metric']} */ ;
/** @type {__VLS_StyleScopedClasses['value']} */ ;
/** @type {__VLS_StyleScopedClasses['label']} */ ;
/** @type {__VLS_StyleScopedClasses['metric']} */ ;
/** @type {__VLS_StyleScopedClasses['value']} */ ;
/** @type {__VLS_StyleScopedClasses['label']} */ ;
/** @type {__VLS_StyleScopedClasses['metric']} */ ;
/** @type {__VLS_StyleScopedClasses['value']} */ ;
/** @type {__VLS_StyleScopedClasses['label']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            AppShell: AppShell,
            AppStatus: AppStatus,
            formatPercent: formatPercent,
            formatDateTime: formatDateTime,
            router: router,
            detail: detail,
            state: state,
            errorMessage: errorMessage,
            requestID: requestID,
            creating: creating,
            load: load,
            accuracy: accuracy,
            startPractice: startPractice,
        };
    },
});
export default (await import('vue')).defineComponent({
    setup() {
        return {};
    },
});
; /* PartiallyEnd: #4569/main.vue */
