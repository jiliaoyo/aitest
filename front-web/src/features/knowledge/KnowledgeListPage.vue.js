import { onMounted, ref, watch } from 'vue';
import { request, ApiError } from '@/api/client';
import AppShell from '@/components/AppShell.vue';
import AppStatus from '@/components/AppStatus.vue';
import { formatPercent, formatDateTime } from '@/app/format';
const exams = ref([]);
const items = ref([]);
const levelId = ref('');
const subjectId = ref('');
const search = ref('');
const state = ref('loading');
const errorMessage = ref('');
const requestID = ref('');
let timer = null;
async function load() {
    state.value = 'loading';
    try {
        const params = new URLSearchParams();
        if (levelId.value)
            params.set('levelId', levelId.value);
        if (subjectId.value)
            params.set('subjectId', subjectId.value);
        if (search.value)
            params.set('q', search.value);
        const res = await request(`/knowledge-points?${params}`);
        items.value = res.knowledgePoints;
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
        // 分类筛选加载失败不阻塞列表
    }
    await load();
});
watch([levelId, subjectId], () => void load());
watch(search, () => {
    if (timer)
        clearTimeout(timer);
    timer = setTimeout(() => void load(), 300);
});
function mastery(k) {
    const stats = k.stats;
    if (!stats || stats.confirmedAnswered === 0)
        return '未练习';
    if (stats.consecutiveWrong >= 3)
        return '连续出错';
    const acc = stats.confirmedCorrect / stats.confirmedAnswered;
    if (acc >= 0.8)
        return '掌握较好';
    return '需要巩固';
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
__VLS_asFunctionalElement(__VLS_intrinsicElements.h1, __VLS_intrinsicElements.h1)({
    ...{ style: {} },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "grid-3" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "field" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
    for: "level",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.select, __VLS_intrinsicElements.select)({
    id: "level",
    value: (__VLS_ctx.levelId),
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
    for: "subject",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.select, __VLS_intrinsicElements.select)({
    id: "subject",
    value: (__VLS_ctx.subjectId),
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
    value: "",
});
for (const [s] of __VLS_getVForSourceType((__VLS_ctx.exams.flatMap((e) => e.subjects)))) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
        key: (s.id),
        value: (s.id),
    });
    (s.name);
}
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "field" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
    for: "search",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
    id: "search",
    value: (__VLS_ctx.search),
    type: "text",
    placeholder: "知识点名称",
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
else if (__VLS_ctx.items.length === 0) {
    /** @type {[typeof AppStatus, ]} */ ;
    // @ts-ignore
    const __VLS_14 = __VLS_asFunctionalComponent(AppStatus, new AppStatus({
        state: "empty",
        message: "没有符合条件且已发布的知识点。",
    }));
    const __VLS_15 = __VLS_14({
        state: "empty",
        message: "没有符合条件且已发布的知识点。",
    }, ...__VLS_functionalComponentArgsRest(__VLS_14));
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
    __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({
        ...{ class: "num" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({});
    __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({});
    __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({});
    __VLS_asFunctionalElement(__VLS_intrinsicElements.tbody, __VLS_intrinsicElements.tbody)({});
    for (const [k] of __VLS_getVForSourceType((__VLS_ctx.items))) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.tr, __VLS_intrinsicElements.tr)({
            key: (k.id),
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({});
        const __VLS_17 = {}.RouterLink;
        /** @type {[typeof __VLS_components.RouterLink, typeof __VLS_components.RouterLink, ]} */ ;
        // @ts-ignore
        const __VLS_18 = __VLS_asFunctionalComponent(__VLS_17, new __VLS_17({
            to: (`/knowledge/${k.id}`),
        }));
        const __VLS_19 = __VLS_18({
            to: (`/knowledge/${k.id}`),
        }, ...__VLS_functionalComponentArgsRest(__VLS_18));
        __VLS_20.slots.default;
        (k.name);
        var __VLS_20;
        __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({});
        (k.levelCode);
        __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({});
        (k.subjectName);
        __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({
            ...{ class: "num" },
        });
        (k.questionCount);
        __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({
            ...{ class: "num" },
        });
        (k.stats && k.stats.confirmedAnswered > 0
            ? __VLS_ctx.formatPercent(k.stats.confirmedCorrect / k.stats.confirmedAnswered) : '—');
        __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({});
        (__VLS_ctx.mastery(k));
        __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({
            ...{ class: "mono" },
        });
        (k.stats?.lastPracticedAt ? __VLS_ctx.formatDateTime(k.stats.lastPracticedAt) : '—');
        __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({});
        const __VLS_21 = {}.RouterLink;
        /** @type {[typeof __VLS_components.RouterLink, typeof __VLS_components.RouterLink, ]} */ ;
        // @ts-ignore
        const __VLS_22 = __VLS_asFunctionalComponent(__VLS_21, new __VLS_21({
            to: (`/knowledge/${k.id}`),
        }));
        const __VLS_23 = __VLS_22({
            to: (`/knowledge/${k.id}`),
        }, ...__VLS_functionalComponentArgsRest(__VLS_22));
        __VLS_24.slots.default;
        var __VLS_24;
    }
}
var __VLS_2;
/** @type {__VLS_StyleScopedClasses['grid-3']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['data']} */ ;
/** @type {__VLS_StyleScopedClasses['num']} */ ;
/** @type {__VLS_StyleScopedClasses['num']} */ ;
/** @type {__VLS_StyleScopedClasses['num']} */ ;
/** @type {__VLS_StyleScopedClasses['num']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            AppShell: AppShell,
            AppStatus: AppStatus,
            formatPercent: formatPercent,
            formatDateTime: formatDateTime,
            exams: exams,
            items: items,
            levelId: levelId,
            subjectId: subjectId,
            search: search,
            state: state,
            errorMessage: errorMessage,
            requestID: requestID,
            load: load,
            mastery: mastery,
        };
    },
});
export default (await import('vue')).defineComponent({
    setup() {
        return {};
    },
});
; /* PartiallyEnd: #4569/main.vue */
