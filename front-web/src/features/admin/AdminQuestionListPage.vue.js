import { onMounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { request, ApiError } from '@/api/client';
import AppShell from '@/components/AppShell.vue';
import AppStatus from '@/components/AppStatus.vue';
import StatusBadge from '@/components/StatusBadge.vue';
import { formatDateTime, questionTypeText } from '@/app/format';
const route = useRoute();
const router = useRouter();
const questions = ref([]);
const nextCursor = ref('');
const exams = ref([]);
const state = ref('loading');
const errorMessage = ref('');
const requestID = ref('');
const filters = ref({
    status: route.query.status ?? '',
    levelId: route.query.levelId ?? '',
    subjectId: route.query.subjectId ?? '',
    q: route.query.q ?? '',
});
const statusOptions = [
    { value: '', label: '全部状态' },
    { value: 'draft', label: '草稿' },
    { value: 'in_review', label: '待审核' },
    { value: 'published', label: '已发布' },
    { value: 'retired', label: '已下架' },
];
let timer = null;
async function load(append = false) {
    if (!append)
        state.value = 'loading';
    try {
        const params = new URLSearchParams({ limit: '20' });
        for (const [k, v] of Object.entries(filters.value)) {
            if (v)
                params.set(k, v);
        }
        if (append && nextCursor.value)
            params.set('cursor', nextCursor.value);
        const res = await request(`/admin/questions?${params}`);
        questions.value = append ? [...questions.value, ...res.questions] : res.questions;
        nextCursor.value = res.nextCursor;
        state.value = 'ready';
    }
    catch (err) {
        errorMessage.value = err instanceof ApiError ? err.message : '加载失败';
        requestID.value = err instanceof ApiError ? err.requestId ?? '' : '';
        state.value = 'error';
    }
}
onMounted(async () => {
    try {
        const res = await request('/catalog');
        exams.value = res.exams;
    }
    catch {
        // 筛选下拉为空不阻塞列表
    }
    await load();
});
watch(filters, () => {
    router.replace({ query: { ...route.query, ...Object.fromEntries(Object.entries(filters.value).filter(([, v]) => v)) } });
    if (timer)
        clearTimeout(timer);
    timer = setTimeout(() => void load(), 250);
}, { deep: true });
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
const __VLS_4 = {}.RouterLink;
/** @type {[typeof __VLS_components.RouterLink, typeof __VLS_components.RouterLink, ]} */ ;
// @ts-ignore
const __VLS_5 = __VLS_asFunctionalComponent(__VLS_4, new __VLS_4({
    to: "/admin/questions/new",
}));
const __VLS_6 = __VLS_5({
    to: "/admin/questions/new",
}, ...__VLS_functionalComponentArgsRest(__VLS_5));
__VLS_7.slots.default;
__VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
    ...{ class: "primary" },
});
var __VLS_7;
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "grid-3" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "field" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
    for: "f-status",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.select, __VLS_intrinsicElements.select)({
    id: "f-status",
    value: (__VLS_ctx.filters.status),
});
for (const [o] of __VLS_getVForSourceType((__VLS_ctx.statusOptions))) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
        key: (o.value),
        value: (o.value),
    });
    (o.label);
}
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "field" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
    for: "f-level",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.select, __VLS_intrinsicElements.select)({
    id: "f-level",
    value: (__VLS_ctx.filters.levelId),
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
    value: "",
});
for (const [l] of __VLS_getVForSourceType((__VLS_ctx.exams.flatMap((e) => e.levels)))) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
        key: (l.id),
        value: (l.id),
    });
    (l.name);
}
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "field" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
    for: "f-q",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
    id: "f-q",
    value: (__VLS_ctx.filters.q),
    type: "text",
    placeholder: "输入关键词",
});
if (__VLS_ctx.state === 'loading') {
    /** @type {[typeof AppStatus, ]} */ ;
    // @ts-ignore
    const __VLS_8 = __VLS_asFunctionalComponent(AppStatus, new AppStatus({
        state: "loading",
    }));
    const __VLS_9 = __VLS_8({
        state: "loading",
    }, ...__VLS_functionalComponentArgsRest(__VLS_8));
}
else if (__VLS_ctx.state === 'error') {
    /** @type {[typeof AppStatus, ]} */ ;
    // @ts-ignore
    const __VLS_11 = __VLS_asFunctionalComponent(AppStatus, new AppStatus({
        ...{ 'onAction': {} },
        state: "error",
        message: (__VLS_ctx.errorMessage),
        requestId: (__VLS_ctx.requestID),
    }));
    const __VLS_12 = __VLS_11({
        ...{ 'onAction': {} },
        state: "error",
        message: (__VLS_ctx.errorMessage),
        requestId: (__VLS_ctx.requestID),
    }, ...__VLS_functionalComponentArgsRest(__VLS_11));
    let __VLS_14;
    let __VLS_15;
    let __VLS_16;
    const __VLS_17 = {
        onAction: (...[$event]) => {
            if (!!(__VLS_ctx.state === 'loading'))
                return;
            if (!(__VLS_ctx.state === 'error'))
                return;
            __VLS_ctx.load();
        }
    };
    var __VLS_13;
}
else if (__VLS_ctx.questions.length === 0) {
    /** @type {[typeof AppStatus, ]} */ ;
    // @ts-ignore
    const __VLS_18 = __VLS_asFunctionalComponent(AppStatus, new AppStatus({
        ...{ 'onAction': {} },
        state: "empty",
        message: "没有符合条件的题目。",
        actionLabel: "新建题目",
    }));
    const __VLS_19 = __VLS_18({
        ...{ 'onAction': {} },
        state: "empty",
        message: "没有符合条件的题目。",
        actionLabel: "新建题目",
    }, ...__VLS_functionalComponentArgsRest(__VLS_18));
    let __VLS_21;
    let __VLS_22;
    let __VLS_23;
    const __VLS_24 = {
        onAction: (...[$event]) => {
            if (!!(__VLS_ctx.state === 'loading'))
                return;
            if (!!(__VLS_ctx.state === 'error'))
                return;
            if (!(__VLS_ctx.questions.length === 0))
                return;
            __VLS_ctx.router.push('/admin/questions/new');
        }
    };
    var __VLS_20;
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
    __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({});
    __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({
        ...{ class: "num" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({});
    __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({});
    __VLS_asFunctionalElement(__VLS_intrinsicElements.tbody, __VLS_intrinsicElements.tbody)({});
    for (const [q] of __VLS_getVForSourceType((__VLS_ctx.questions))) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.tr, __VLS_intrinsicElements.tr)({
            key: (q.id),
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({
            ...{ style: {} },
        });
        const __VLS_25 = {}.RouterLink;
        /** @type {[typeof __VLS_components.RouterLink, typeof __VLS_components.RouterLink, ]} */ ;
        // @ts-ignore
        const __VLS_26 = __VLS_asFunctionalComponent(__VLS_25, new __VLS_25({
            to: (`/admin/questions/${q.id}`),
            ...{ class: "mono" },
            lang: "ja",
        }));
        const __VLS_27 = __VLS_26({
            to: (`/admin/questions/${q.id}`),
            ...{ class: "mono" },
            lang: "ja",
        }, ...__VLS_functionalComponentArgsRest(__VLS_26));
        __VLS_28.slots.default;
        (q.currentVersion?.stem.slice(0, 40) ?? '');
        ((q.currentVersion?.stem.length ?? 0) > 40 ? '…' : '');
        var __VLS_28;
        __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({});
        (__VLS_ctx.questionTypeText[q.currentVersion?.type ?? ''] ?? '—');
        __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({});
        /** @type {[typeof StatusBadge, ]} */ ;
        // @ts-ignore
        const __VLS_29 = __VLS_asFunctionalComponent(StatusBadge, new StatusBadge({
            value: (q.status),
        }));
        const __VLS_30 = __VLS_29({
            value: (q.status),
        }, ...__VLS_functionalComponentArgsRest(__VLS_29));
        __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({
            ...{ class: "num" },
        });
        (q.currentVersion?.versionNo ?? '-');
        __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({});
        (q.hasAnswer ? '✓' : '—');
        __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({
            ...{ class: "mono" },
        });
        (__VLS_ctx.formatDateTime(q.updatedAt));
    }
    if (__VLS_ctx.nextCursor) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ style: {} },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
            ...{ onClick: (...[$event]) => {
                    if (!!(__VLS_ctx.state === 'loading'))
                        return;
                    if (!!(__VLS_ctx.state === 'error'))
                        return;
                    if (!!(__VLS_ctx.questions.length === 0))
                        return;
                    if (!(__VLS_ctx.nextCursor))
                        return;
                    __VLS_ctx.load(true);
                } },
        });
    }
}
var __VLS_2;
/** @type {__VLS_StyleScopedClasses['page-header']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
/** @type {__VLS_StyleScopedClasses['grid-3']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
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
            questionTypeText: questionTypeText,
            router: router,
            questions: questions,
            nextCursor: nextCursor,
            exams: exams,
            state: state,
            errorMessage: errorMessage,
            requestID: requestID,
            filters: filters,
            statusOptions: statusOptions,
            load: load,
        };
    },
});
export default (await import('vue')).defineComponent({
    setup() {
        return {};
    },
});
; /* PartiallyEnd: #4569/main.vue */
