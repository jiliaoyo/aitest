import { onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { request, ApiError } from '@/api/client';
import AppShell from '@/components/AppShell.vue';
import AppStatus from '@/components/AppStatus.vue';
import StatusBadge from '@/components/StatusBadge.vue';
import { formatDateTime, formatPercent } from '@/app/format';
const router = useRouter();
const dashboard = ref(null);
const state = ref('loading');
const errorBody = ref(null);
async function load() {
    state.value = 'loading';
    try {
        dashboard.value = await request('/dashboard');
        state.value = 'ready';
    }
    catch (err) {
        errorBody.value = err instanceof ApiError ? err : null;
        state.value = 'error';
    }
}
onMounted(load);
async function goPractice(rec) {
    const me = await request('/me');
    if (!me.user.defaultLevelId) {
        await router.push('/practice/new');
        return;
    }
    try {
        const session = await request('/practice-sessions', {
            method: 'POST',
            body: {
                levelId: me.user.defaultLevelId,
                mode: 'knowledge',
                knowledgePointIds: rec.knowledgePointIds,
                count: rec.suggestedCount,
            },
        });
        await router.push(`/practice/${session.id}`);
    }
    catch (err) {
        // 题量不足等情况下进入创建页让用户确认
        await router.push('/practice/new');
        void err;
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
        message: (__VLS_ctx.errorBody?.message),
        requestId: (__VLS_ctx.errorBody?.requestId),
    }));
    const __VLS_8 = __VLS_7({
        ...{ 'onAction': {} },
        state: "error",
        message: (__VLS_ctx.errorBody?.message),
        requestId: (__VLS_ctx.errorBody?.requestId),
    }, ...__VLS_functionalComponentArgsRest(__VLS_7));
    let __VLS_10;
    let __VLS_11;
    let __VLS_12;
    const __VLS_13 = {
        onAction: (__VLS_ctx.load)
    };
    var __VLS_9;
}
else if (__VLS_ctx.dashboard) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "page-header" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({});
    __VLS_asFunctionalElement(__VLS_intrinsicElements.h1, __VLS_intrinsicElements.h1)({});
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "muted" },
    });
    const __VLS_14 = {}.RouterLink;
    /** @type {[typeof __VLS_components.RouterLink, typeof __VLS_components.RouterLink, ]} */ ;
    // @ts-ignore
    const __VLS_15 = __VLS_asFunctionalComponent(__VLS_14, new __VLS_14({
        ...{ class: "primary" },
        to: "/practice/new",
        custom: true,
    }));
    const __VLS_16 = __VLS_15({
        ...{ class: "primary" },
        to: "/practice/new",
        custom: true,
    }, ...__VLS_functionalComponentArgsRest(__VLS_15));
    {
        const { default: __VLS_thisSlot } = __VLS_17.slots;
        const [{ navigate }] = __VLS_getSlotParams(__VLS_thisSlot);
        __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
            ...{ onClick: (navigate) },
            ...{ class: "primary" },
        });
        __VLS_17.slots['' /* empty slot name completion */];
    }
    var __VLS_17;
    if (__VLS_ctx.dashboard.activeSession) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "card" },
            'data-tone': "accent",
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "page-header" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({});
        __VLS_asFunctionalElement(__VLS_intrinsicElements.h2, __VLS_intrinsicElements.h2)({
            ...{ style: {} },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "muted mono" },
        });
        (__VLS_ctx.dashboard.activeSession.answeredCount);
        (__VLS_ctx.dashboard.activeSession.totalCount);
        __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
            ...{ onClick: (...[$event]) => {
                    if (!!(__VLS_ctx.state === 'loading'))
                        return;
                    if (!!(__VLS_ctx.state === 'error'))
                        return;
                    if (!(__VLS_ctx.dashboard))
                        return;
                    if (!(__VLS_ctx.dashboard.activeSession))
                        return;
                    __VLS_ctx.router.push(`/practice/${__VLS_ctx.dashboard.activeSession.id}`);
                } },
            ...{ class: "primary" },
        });
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.section, __VLS_intrinsicElements.section)({
        'aria-labelledby': "rec-title",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.h2, __VLS_intrinsicElements.h2)({
        id: "rec-title",
        ...{ style: {} },
    });
    if (__VLS_ctx.dashboard.recommendations.length === 0 && __VLS_ctx.dashboard.comprehensive) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "card" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({});
        __VLS_asFunctionalElement(__VLS_intrinsicElements.strong, __VLS_intrinsicElements.strong)({});
        (__VLS_ctx.dashboard.comprehensive.name);
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "muted" },
        });
        (__VLS_ctx.dashboard.comprehensive.reason);
        __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
            ...{ onClick: (...[$event]) => {
                    if (!!(__VLS_ctx.state === 'loading'))
                        return;
                    if (!!(__VLS_ctx.state === 'error'))
                        return;
                    if (!(__VLS_ctx.dashboard))
                        return;
                    if (!(__VLS_ctx.dashboard.recommendations.length === 0 && __VLS_ctx.dashboard.comprehensive))
                        return;
                    __VLS_ctx.router.push('/practice/new');
                } },
            ...{ class: "primary" },
        });
    }
    for (const [rec] of __VLS_getVForSourceType((__VLS_ctx.dashboard.recommendations))) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            key: (rec.knowledgePointId ?? rec.name),
            ...{ class: "card" },
            ...{ style: {} },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "page-header" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({});
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({});
        __VLS_asFunctionalElement(__VLS_intrinsicElements.strong, __VLS_intrinsicElements.strong)({});
        (rec.name);
        __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
            ...{ class: "muted mono" },
            ...{ style: {} },
        });
        (__VLS_ctx.formatPercent(rec.accuracy));
        (rec.recentAnswered);
        (rec.consecutiveWrong);
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "muted" },
        });
        (rec.reason);
        __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
            ...{ onClick: (...[$event]) => {
                    if (!!(__VLS_ctx.state === 'loading'))
                        return;
                    if (!!(__VLS_ctx.state === 'error'))
                        return;
                    if (!(__VLS_ctx.dashboard))
                        return;
                    __VLS_ctx.goPractice(rec);
                } },
            ...{ class: "primary" },
        });
        (rec.suggestedCount);
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.section, __VLS_intrinsicElements.section)({
        'aria-labelledby': "recent-title",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.h2, __VLS_intrinsicElements.h2)({
        id: "recent-title",
        ...{ style: {} },
    });
    if (__VLS_ctx.dashboard.recentSessions.length === 0) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "card" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "muted" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
            ...{ onClick: (...[$event]) => {
                    if (!!(__VLS_ctx.state === 'loading'))
                        return;
                    if (!!(__VLS_ctx.state === 'error'))
                        return;
                    if (!(__VLS_ctx.dashboard))
                        return;
                    if (!(__VLS_ctx.dashboard.recentSessions.length === 0))
                        return;
                    __VLS_ctx.router.push('/practice/new');
                } },
            ...{ class: "primary" },
        });
    }
    else {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "card" },
            ...{ style: {} },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.table, __VLS_intrinsicElements.table)({
            ...{ class: "data" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.thead, __VLS_intrinsicElements.thead)({});
        __VLS_asFunctionalElement(__VLS_intrinsicElements.tr, __VLS_intrinsicElements.tr)({});
        __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({});
        __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({});
        __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({
            ...{ class: "num" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({});
        __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({});
        __VLS_asFunctionalElement(__VLS_intrinsicElements.tbody, __VLS_intrinsicElements.tbody)({});
        for (const [s] of __VLS_getVForSourceType((__VLS_ctx.dashboard.recentSessions))) {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.tr, __VLS_intrinsicElements.tr)({
                key: (s.id),
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({
                ...{ class: "mono" },
            });
            (s.id.slice(0, 8));
            __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({});
            /** @type {[typeof StatusBadge, ]} */ ;
            // @ts-ignore
            const __VLS_18 = __VLS_asFunctionalComponent(StatusBadge, new StatusBadge({
                value: (s.status),
                kind: "session",
            }));
            const __VLS_19 = __VLS_18({
                value: (s.status),
                kind: "session",
            }, ...__VLS_functionalComponentArgsRest(__VLS_18));
            __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({
                ...{ class: "num" },
            });
            (s.totalCount);
            __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({
                ...{ class: "mono" },
            });
            (__VLS_ctx.formatDateTime(s.createdAt));
            __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({});
            const __VLS_21 = {}.RouterLink;
            /** @type {[typeof __VLS_components.RouterLink, typeof __VLS_components.RouterLink, ]} */ ;
            // @ts-ignore
            const __VLS_22 = __VLS_asFunctionalComponent(__VLS_21, new __VLS_21({
                to: (`/practice/${s.id}/result`),
            }));
            const __VLS_23 = __VLS_22({
                to: (`/practice/${s.id}/result`),
            }, ...__VLS_functionalComponentArgsRest(__VLS_22));
            __VLS_24.slots.default;
            var __VLS_24;
        }
    }
}
var __VLS_2;
/** @type {__VLS_StyleScopedClasses['page-header']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['page-header']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['page-header']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['data']} */ ;
/** @type {__VLS_StyleScopedClasses['num']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
/** @type {__VLS_StyleScopedClasses['num']} */ ;
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
            router: router,
            dashboard: dashboard,
            state: state,
            errorBody: errorBody,
            load: load,
            goPractice: goPractice,
        };
    },
});
export default (await import('vue')).defineComponent({
    setup() {
        return {};
    },
});
; /* PartiallyEnd: #4569/main.vue */
