import { onMounted, reactive, ref } from 'vue';
import { request, ApiError } from '@/api/client';
import AppShell from '@/components/AppShell.vue';
import AppStatus from '@/components/AppStatus.vue';
import StatusBadge from '@/components/StatusBadge.vue';
import { formatDateTime } from '@/app/format';
const reports = ref([]);
const state = ref('loading');
const errorMessage = ref('');
const requestID = ref('');
const statusFilter = ref('open');
const notes = reactive(new Map());
const targetTypeText = {
    stem: '题干',
    answer: '答案',
    explanation: '解析',
    classification: '分类',
    ai_grading: 'AI 判定',
};
async function load() {
    state.value = 'loading';
    try {
        const res = await request(`/admin/issue-reports?status=${statusFilter.value}`);
        reports.value = res.issueReports;
        state.value = 'ready';
    }
    catch (err) {
        errorMessage.value = err instanceof ApiError ? err.message : '加载失败';
        requestID.value = err instanceof ApiError ? err.requestId ?? '' : '';
        state.value = 'error';
    }
}
onMounted(load);
async function handle(report, status) {
    try {
        await request(`/admin/issue-reports/${report.id}`, {
            method: 'PATCH',
            body: { status, resolutionNote: notes.get(report.id) ?? '' },
        });
        await load();
    }
    catch (err) {
        errorMessage.value = err instanceof ApiError ? err.message : '处理失败';
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
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "page-header" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.h1, __VLS_intrinsicElements.h1)({
    ...{ style: {} },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
    ...{ style: {} },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
    ...{ class: "muted" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.select, __VLS_intrinsicElements.select)({
    ...{ onChange: (__VLS_ctx.load) },
    value: (__VLS_ctx.statusFilter),
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
    value: "open",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
    value: "resolved",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
    value: "dismissed",
});
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
else if (__VLS_ctx.reports.length === 0) {
    /** @type {[typeof AppStatus, ]} */ ;
    // @ts-ignore
    const __VLS_14 = __VLS_asFunctionalComponent(AppStatus, new AppStatus({
        state: "empty",
        message: "没有待处理的举报。",
    }));
    const __VLS_15 = __VLS_14({
        state: "empty",
        message: "没有待处理的举报。",
    }, ...__VLS_functionalComponentArgsRest(__VLS_14));
}
else {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ style: {} },
    });
    for (const [r] of __VLS_getVForSourceType((__VLS_ctx.reports))) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.article, __VLS_intrinsicElements.article)({
            key: (r.id),
            ...{ class: "card" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "page-header" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({});
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ style: {} },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
            ...{ class: "tag" },
            'data-tone': "accent",
        });
        (__VLS_ctx.targetTypeText[r.targetType] ?? r.targetType);
        /** @type {[typeof StatusBadge, ]} */ ;
        // @ts-ignore
        const __VLS_17 = __VLS_asFunctionalComponent(StatusBadge, new StatusBadge({
            value: (r.status),
            kind: "issue",
            ...{ style: {} },
        }));
        const __VLS_18 = __VLS_17({
            value: (r.status),
            kind: "issue",
            ...{ style: {} },
        }, ...__VLS_functionalComponentArgsRest(__VLS_17));
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "muted mono" },
            ...{ style: {} },
        });
        (r.questionId.slice(0, 8));
        (r.userEmail);
        (__VLS_ctx.formatDateTime(r.createdAt));
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "material-text" },
            lang: "ja",
            ...{ style: {} },
        });
        (r.stem);
        if (r.description) {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
                ...{ style: {} },
            });
            (r.description);
        }
        if (r.status === 'open') {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
                ...{ style: {} },
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
                ...{ class: "field" },
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
                for: (`note-${r.id}`),
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
                ...{ onInput: (...[$event]) => {
                        if (!!(__VLS_ctx.state === 'loading'))
                            return;
                        if (!!(__VLS_ctx.state === 'error'))
                            return;
                        if (!!(__VLS_ctx.reports.length === 0))
                            return;
                        if (!(r.status === 'open'))
                            return;
                        __VLS_ctx.notes.set(r.id, $event.target.value);
                    } },
                id: (`note-${r.id}`),
                value: (__VLS_ctx.notes.get(r.id) ?? ''),
                type: "text",
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
                ...{ style: {} },
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
                ...{ onClick: (...[$event]) => {
                        if (!!(__VLS_ctx.state === 'loading'))
                            return;
                        if (!!(__VLS_ctx.state === 'error'))
                            return;
                        if (!!(__VLS_ctx.reports.length === 0))
                            return;
                        if (!(r.status === 'open'))
                            return;
                        __VLS_ctx.handle(r, 'resolved');
                    } },
                ...{ class: "primary" },
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
                ...{ onClick: (...[$event]) => {
                        if (!!(__VLS_ctx.state === 'loading'))
                            return;
                        if (!!(__VLS_ctx.state === 'error'))
                            return;
                        if (!!(__VLS_ctx.reports.length === 0))
                            return;
                        if (!(r.status === 'open'))
                            return;
                        __VLS_ctx.handle(r, 'dismissed');
                    } },
                ...{ class: "danger" },
            });
        }
        else if (r.resolutionNote) {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
                ...{ class: "muted" },
                ...{ style: {} },
            });
            (r.resolutionNote);
        }
    }
}
var __VLS_2;
/** @type {__VLS_StyleScopedClasses['page-header']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['page-header']} */ ;
/** @type {__VLS_StyleScopedClasses['tag']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
/** @type {__VLS_StyleScopedClasses['material-text']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
/** @type {__VLS_StyleScopedClasses['danger']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            AppShell: AppShell,
            AppStatus: AppStatus,
            StatusBadge: StatusBadge,
            formatDateTime: formatDateTime,
            reports: reports,
            state: state,
            errorMessage: errorMessage,
            requestID: requestID,
            statusFilter: statusFilter,
            notes: notes,
            targetTypeText: targetTypeText,
            load: load,
            handle: handle,
        };
    },
});
export default (await import('vue')).defineComponent({
    setup() {
        return {};
    },
});
; /* PartiallyEnd: #4569/main.vue */
