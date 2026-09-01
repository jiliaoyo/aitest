import { computed, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { request, ApiError } from '@/api/client';
import AppShell from '@/components/AppShell.vue';
import AppStatus from '@/components/AppStatus.vue';
import { authorityText, gradingStatusText } from '@/app/format';
const router = useRouter();
const items = ref([]);
const kps = ref([]);
const kpFilter = ref('');
const state = ref('loading');
const errorMessage = ref('');
const requestID = ref('');
const creating = ref(false);
async function load() {
    state.value = 'loading';
    try {
        const params = new URLSearchParams();
        if (kpFilter.value)
            params.set('knowledgePointId', kpFilter.value);
        const res = await request(`/wrong-items?${params}`);
        items.value = res.wrongItems;
        state.value = 'ready';
    }
    catch (err) {
        errorMessage.value = err instanceof ApiError ? err.message : '加载失败';
        requestID.value = err instanceof ApiError ? err.requestId ?? '' : '';
        state.value = 'error';
    }
}
onMounted(async () => {
    await load();
    try {
        const res = await request('/knowledge-points');
        kps.value = res.knowledgePoints.filter((k) => (k.stats?.confirmedAnswered ?? 0) > 0);
    }
    catch {
        // 筛选列表加载失败不阻塞主列表
    }
});
function optionText(item, ids) {
    if (!ids)
        return '—';
    return ids.map((id) => item.options?.find((o) => o.id === id)?.label ?? id).join('、');
}
function answerText(item, answer) {
    if (!answer)
        return '未作答';
    if ('optionIds' in answer && answer.optionIds)
        return optionText(item, answer.optionIds);
    if ('text' in answer && answer.text)
        return answer.text;
    return '未作答';
}
function correctText(item) {
    const answer = item.correctAnswer;
    if (!answer)
        return '—';
    if ('optionIds' in answer && answer.optionIds)
        return optionText(item, answer.optionIds);
    if ('text' in answer && answer.text)
        return answer.text;
    return '—';
}
const canRetrain = computed(() => items.value.length > 0);
async function retrain() {
    creating.value = true;
    try {
        const me = await request('/me');
        if (!me.user.defaultLevelId) {
            await router.push('/practice/new');
            return;
        }
        const session = await request('/practice-sessions', {
            method: 'POST',
            body: { levelId: me.user.defaultLevelId, mode: 'wrong_items', count: 10 },
        });
        await router.push(`/practice/${session.id}`);
    }
    catch (err) {
        errorMessage.value = err instanceof ApiError ? err.message : '创建错题练习失败';
        state.value = 'error';
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
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "page-header" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.h1, __VLS_intrinsicElements.h1)({
    ...{ style: {} },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
    ...{ onClick: (__VLS_ctx.retrain) },
    ...{ class: "primary" },
    disabled: (!__VLS_ctx.canRetrain || __VLS_ctx.creating),
});
(__VLS_ctx.creating ? '创建中…' : '错题重练 10 题');
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "field" },
    ...{ style: {} },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
    for: "kp-filter",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.select, __VLS_intrinsicElements.select)({
    ...{ onChange: (__VLS_ctx.load) },
    id: "kp-filter",
    value: (__VLS_ctx.kpFilter),
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
    value: "",
});
for (const [k] of __VLS_getVForSourceType((__VLS_ctx.kps))) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
        key: (k.id),
        value: (k.id),
    });
    (k.name);
}
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
        ...{ 'onAction': {} },
        state: "empty",
        message: "没有待复习的错题，继续保持。",
        actionLabel: "创建新练习",
    }));
    const __VLS_15 = __VLS_14({
        ...{ 'onAction': {} },
        state: "empty",
        message: "没有待复习的错题，继续保持。",
        actionLabel: "创建新练习",
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
            if (!(__VLS_ctx.items.length === 0))
                return;
            __VLS_ctx.router.push('/practice/new');
        }
    };
    var __VLS_16;
}
else {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ style: {} },
    });
    for (const [item] of __VLS_getVForSourceType((__VLS_ctx.items))) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.article, __VLS_intrinsicElements.article)({
            key: (item.itemId),
            ...{ class: "card" },
            lang: "ja",
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.header, __VLS_intrinsicElements.header)({
            ...{ style: {} },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
            ...{ class: "tag" },
            'data-tone': "danger",
        });
        (__VLS_ctx.gradingStatusText[item.gradingStatus] ?? item.gradingStatus);
        if (item.answerAuthority) {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
                ...{ class: "tag" },
                'data-tone': "success",
            });
            (__VLS_ctx.authorityText[item.answerAuthority]);
        }
        if (item.knowledgePoints.length) {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
                ...{ class: "muted" },
            });
            (item.knowledgePoints.map((k) => k.name).join('、'));
        }
        if (item.material) {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.section, __VLS_intrinsicElements.section)({
                ...{ class: "card" },
                ...{ style: {} },
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
                ...{ class: "material-text" },
                ...{ style: {} },
            });
            (item.material.content);
        }
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ style: {} },
        });
        (item.stem);
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "mono" },
            ...{ style: {} },
        });
        (__VLS_ctx.answerText(item, item.userAnswer));
        (__VLS_ctx.correctText(item));
        if (item.explanation) {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
                ...{ style: {} },
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
                ...{ class: "tag" },
                ...{ style: {} },
            });
            (item.explanation.source === 'ai' ? 'AI 解析（可能有误）' : item.explanation.source === 'official' ? '官方解析' : '人工解析');
            __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
                ...{ style: {} },
            });
            (item.explanation.text);
        }
    }
}
var __VLS_2;
/** @type {__VLS_StyleScopedClasses['page-header']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['tag']} */ ;
/** @type {__VLS_StyleScopedClasses['tag']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['material-text']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
/** @type {__VLS_StyleScopedClasses['tag']} */ ;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            AppShell: AppShell,
            AppStatus: AppStatus,
            authorityText: authorityText,
            gradingStatusText: gradingStatusText,
            router: router,
            items: items,
            kps: kps,
            kpFilter: kpFilter,
            state: state,
            errorMessage: errorMessage,
            requestID: requestID,
            creating: creating,
            load: load,
            answerText: answerText,
            correctText: correctText,
            canRetrain: canRetrain,
            retrain: retrain,
        };
    },
});
export default (await import('vue')).defineComponent({
    setup() {
        return {};
    },
});
; /* PartiallyEnd: #4569/main.vue */
