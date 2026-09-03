import { onBeforeUnmount, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { request, ApiError } from '@/api/client';
import AppShell from '@/components/AppShell.vue';
import AppStatus from '@/components/AppStatus.vue';
import StatusBadge from '@/components/StatusBadge.vue';
import { formatDateTime } from '@/app/format';
const route = useRoute();
const router = useRouter();
const jobID = route.params.importJobId;
const job = ref(null);
const items = ref([]);
const state = ref('loading');
const errorMessage = ref('');
const requestID = ref('');
const retrying = ref(false);
let timer = null;
function shouldPoll() {
    return job.value?.status === 'uploaded' || job.value?.status === 'extracting' || job.value?.status === 'structuring';
}
function schedule() {
    if (timer)
        clearTimeout(timer);
    if (shouldPoll())
        timer = setTimeout(() => void load(false), 3000);
}
async function load(initial = true) {
    if (initial)
        state.value = 'loading';
    try {
        const res = await request(`/admin/import-jobs/${jobID}`);
        job.value = res.job;
        items.value = res.items;
        state.value = 'ready';
        schedule();
    }
    catch (err) {
        errorMessage.value = err instanceof ApiError ? err.message : '加载失败';
        requestID.value = err instanceof ApiError ? err.requestId ?? '' : '';
        state.value = 'error';
    }
}
async function retry() {
    retrying.value = true;
    try {
        await request(`/admin/import-jobs/${jobID}/retry`, { method: 'POST' });
        await load();
    }
    catch (err) {
        errorMessage.value = err instanceof ApiError ? err.message : '重试失败';
    }
    finally {
        retrying.value = false;
    }
}
onMounted(() => void load());
onBeforeUnmount(() => { if (timer)
    clearTimeout(timer); });
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
else if (__VLS_ctx.job) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "page-header" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({});
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "muted" },
        ...{ style: {} },
    });
    const __VLS_14 = {}.RouterLink;
    /** @type {[typeof __VLS_components.RouterLink, typeof __VLS_components.RouterLink, ]} */ ;
    // @ts-ignore
    const __VLS_15 = __VLS_asFunctionalComponent(__VLS_14, new __VLS_14({
        to: "/admin/imports",
    }));
    const __VLS_16 = __VLS_15({
        to: "/admin/imports",
    }, ...__VLS_functionalComponentArgsRest(__VLS_15));
    __VLS_17.slots.default;
    var __VLS_17;
    __VLS_asFunctionalElement(__VLS_intrinsicElements.h1, __VLS_intrinsicElements.h1)({
        ...{ style: {} },
    });
    (__VLS_ctx.job.fileName);
    /** @type {[typeof StatusBadge, ]} */ ;
    // @ts-ignore
    const __VLS_18 = __VLS_asFunctionalComponent(StatusBadge, new StatusBadge({
        value: (__VLS_ctx.job.status),
    }));
    const __VLS_19 = __VLS_18({
        value: (__VLS_ctx.job.status),
    }, ...__VLS_functionalComponentArgsRest(__VLS_18));
    if (__VLS_ctx.job.stageError) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "error-summary" },
            role: "alert",
        });
        (__VLS_ctx.job.stageError);
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "card" },
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.strong, __VLS_intrinsicElements.strong)({});
    /** @type {[typeof StatusBadge, ]} */ ;
    // @ts-ignore
    const __VLS_21 = __VLS_asFunctionalComponent(StatusBadge, new StatusBadge({
        value: (__VLS_ctx.job.status),
    }));
    const __VLS_22 = __VLS_21({
        value: (__VLS_ctx.job.status),
    }, ...__VLS_functionalComponentArgsRest(__VLS_21));
    __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
        ...{ class: "muted" },
    });
    (__VLS_ctx.job.itemCount);
    (__VLS_ctx.formatDateTime(__VLS_ctx.job.updatedAt));
    if (__VLS_ctx.job.status === 'failed') {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
            ...{ onClick: (__VLS_ctx.retry) },
            ...{ class: "primary" },
            disabled: (__VLS_ctx.retrying),
        });
        (__VLS_ctx.retrying ? '重试中…' : '从安全阶段重试');
    }
    if (__VLS_ctx.job.extractedText) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "card" },
            ...{ style: {} },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.h2, __VLS_intrinsicElements.h2)({
            ...{ style: {} },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.pre, __VLS_intrinsicElements.pre)({
            ...{ class: "import-source" },
            lang: "ja",
        });
        (__VLS_ctx.job.extractedText);
    }
    if (__VLS_ctx.items.length) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "card" },
            ...{ style: {} },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.h2, __VLS_intrinsicElements.h2)({
            ...{ style: {} },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.table, __VLS_intrinsicElements.table)({
            ...{ class: "data" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.thead, __VLS_intrinsicElements.thead)({});
        __VLS_asFunctionalElement(__VLS_intrinsicElements.tr, __VLS_intrinsicElements.tr)({});
        __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({
            ...{ class: "num" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({});
        __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({});
        __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({});
        __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({});
        __VLS_asFunctionalElement(__VLS_intrinsicElements.tbody, __VLS_intrinsicElements.tbody)({});
        for (const [item] of __VLS_getVForSourceType((__VLS_ctx.items))) {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.tr, __VLS_intrinsicElements.tr)({
                key: (item.id),
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({
                ...{ class: "num" },
            });
            (item.position);
            __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({
                lang: "ja",
            });
            (item.draft?.stem ?? '—');
            __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({});
            if (item.anomalies.length) {
                __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
                    ...{ class: "tag" },
                    'data-tone': "warning",
                });
                (item.anomalies.length);
            }
            else {
                __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
                    ...{ class: "muted" },
                });
            }
            __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({});
            /** @type {[typeof StatusBadge, ]} */ ;
            // @ts-ignore
            const __VLS_24 = __VLS_asFunctionalComponent(StatusBadge, new StatusBadge({
                value: (item.reviewStatus),
            }));
            const __VLS_25 = __VLS_24({
                value: (item.reviewStatus),
            }, ...__VLS_functionalComponentArgsRest(__VLS_24));
            __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({});
            const __VLS_27 = {}.RouterLink;
            /** @type {[typeof __VLS_components.RouterLink, typeof __VLS_components.RouterLink, ]} */ ;
            // @ts-ignore
            const __VLS_28 = __VLS_asFunctionalComponent(__VLS_27, new __VLS_27({
                to: (`/admin/import-items/${item.id}`),
            }));
            const __VLS_29 = __VLS_28({
                to: (`/admin/import-items/${item.id}`),
            }, ...__VLS_functionalComponentArgsRest(__VLS_28));
            __VLS_30.slots.default;
            var __VLS_30;
        }
    }
    else if (__VLS_ctx.job.status === 'review_ready') {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "muted" },
        });
    }
    if (__VLS_ctx.job.status === 'uploaded' || __VLS_ctx.job.status === 'extracting' || __VLS_ctx.job.status === 'structuring') {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "muted" },
            role: "status",
        });
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
        ...{ onClick: (...[$event]) => {
                if (!!(__VLS_ctx.state === 'loading'))
                    return;
                if (!!(__VLS_ctx.state === 'error'))
                    return;
                if (!(__VLS_ctx.job))
                    return;
                __VLS_ctx.router.push('/admin/imports');
            } },
        ...{ class: "ghost" },
        ...{ style: {} },
    });
}
var __VLS_2;
/** @type {__VLS_StyleScopedClasses['page-header']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['error-summary']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['import-source']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['data']} */ ;
/** @type {__VLS_StyleScopedClasses['num']} */ ;
/** @type {__VLS_StyleScopedClasses['num']} */ ;
/** @type {__VLS_StyleScopedClasses['tag']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['ghost']} */ ;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            AppShell: AppShell,
            AppStatus: AppStatus,
            StatusBadge: StatusBadge,
            formatDateTime: formatDateTime,
            router: router,
            job: job,
            items: items,
            state: state,
            errorMessage: errorMessage,
            requestID: requestID,
            retrying: retrying,
            load: load,
            retry: retry,
        };
    },
});
export default (await import('vue')).defineComponent({
    setup() {
        return {};
    },
});
; /* PartiallyEnd: #4569/main.vue */
