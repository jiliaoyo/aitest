import { computed, onMounted, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import { request, ApiError } from '@/api/client';
import AppShell from '@/components/AppShell.vue';
import AppStatus from '@/components/AppStatus.vue';
import { sessionUser } from '@/app/session';
const router = useRouter();
const exams = ref([]);
const loadState = ref('loading');
const loadError = ref('');
const levelId = ref('');
const subjectId = ref('');
const sourceId = ref('');
const mode = ref('comprehensive');
const selectionOrder = ref('source_order');
const knowledgePointIds = ref([]);
const count = ref(20);
const availability = ref(null);
const availabilityLoading = ref(false);
const availabilityError = ref('');
const creating = ref(false);
const createError = ref('');
const kps = ref([]);
const kpsLoading = ref(false);
const sources = ref([]);
const sourcesLoading = ref(false);
const sourcesError = ref('');
onMounted(async () => {
    try {
        const res = await request('/catalog');
        exams.value = res.exams;
        levelId.value = sessionUser()?.defaultLevelId ?? res.exams[0]?.levels[0]?.id ?? '';
        loadState.value = 'ready';
    }
    catch (err) {
        loadError.value = err instanceof ApiError ? err.message : '加载失败';
        loadState.value = 'error';
    }
});
const levels = computed(() => exams.value.flatMap((e) => e.levels));
const subjects = computed(() => exams.value.flatMap((e) => e.subjects));
watch([levelId, subjectId, mode, selectionOrder, sourceId, knowledgePointIds], async () => {
    await refreshAvailability();
    if (mode.value === 'knowledge') {
        await loadKnowledgePoints();
    }
}, { deep: true });
watch([levelId, subjectId], async () => {
    if (!levelId.value)
        return;
    sourcesLoading.value = true;
    sourcesError.value = '';
    try {
        const params = new URLSearchParams({ levelId: levelId.value });
        if (subjectId.value)
            params.set('subjectId', subjectId.value);
        const res = await request(`/practice/sources?${params}`);
        sources.value = res.sources;
        if (!sources.value.some((source) => source.id === sourceId.value)) {
            sourceId.value = '';
        }
    }
    catch (err) {
        sources.value = [];
        sourceId.value = '';
        sourcesError.value = err instanceof ApiError ? err.message : '数据来源加载失败';
    }
    finally {
        sourcesLoading.value = false;
    }
}, { immediate: true });
let availabilityTimer = null;
function refreshAvailability() {
    if (!levelId.value)
        return;
    if (availabilityTimer)
        clearTimeout(availabilityTimer);
    availabilityTimer = setTimeout(async () => {
        availabilityLoading.value = true;
        availabilityError.value = '';
        try {
            const params = new URLSearchParams({
                levelId: levelId.value,
                mode: mode.value,
                selectionOrder: selectionOrder.value,
                count: '10',
            });
            if (subjectId.value)
                params.set('subjectId', subjectId.value);
            if (sourceId.value)
                params.set('sourceId', sourceId.value);
            if (mode.value === 'knowledge' && knowledgePointIds.value.length) {
                params.set('knowledgePointIds', knowledgePointIds.value.join(','));
            }
            const res = await request(`/practice/availability?${params}`);
            availability.value = res.available;
        }
        catch (err) {
            availabilityError.value = err instanceof ApiError ? err.message : '可用题量查询失败';
            availability.value = null;
        }
        finally {
            availabilityLoading.value = false;
        }
    }, 250);
}
async function loadKnowledgePoints() {
    if (!levelId.value)
        return;
    kpsLoading.value = true;
    try {
        const params = new URLSearchParams({ levelId: levelId.value });
        if (subjectId.value)
            params.set('subjectId', subjectId.value);
        const res = await request(`/knowledge-points?${params}`);
        kps.value = res.knowledgePoints;
    }
    finally {
        kpsLoading.value = false;
    }
}
const insufficient = computed(() => availability.value !== null && availability.value < count.value);
const canCreate = computed(() => !!levelId.value &&
    (mode.value !== 'knowledge' || knowledgePointIds.value.length > 0) &&
    availability.value !== null &&
    !insufficient.value &&
    !availabilityLoading.value);
async function create() {
    creating.value = true;
    createError.value = '';
    try {
        const session = await request('/practice-sessions', {
            method: 'POST',
            body: {
                levelId: levelId.value,
                subjectId: subjectId.value,
                sourceId: sourceId.value,
                mode: mode.value,
                selectionOrder: selectionOrder.value,
                knowledgePointIds: knowledgePointIds.value,
                count: count.value,
            },
        });
        await router.push(`/practice/${session.id}`);
    }
    catch (err) {
        if (err instanceof ApiError && err.code === 'insufficient_questions') {
            availability.value = Number(err.details?.available ?? 0);
        }
        createError.value = err instanceof ApiError ? err.message : '创建失败，请重试';
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
if (__VLS_ctx.loadState === 'loading') {
    /** @type {[typeof AppStatus, ]} */ ;
    // @ts-ignore
    const __VLS_4 = __VLS_asFunctionalComponent(AppStatus, new AppStatus({
        state: "loading",
    }));
    const __VLS_5 = __VLS_4({
        state: "loading",
    }, ...__VLS_functionalComponentArgsRest(__VLS_4));
}
else if (__VLS_ctx.loadState === 'error') {
    /** @type {[typeof AppStatus, ]} */ ;
    // @ts-ignore
    const __VLS_7 = __VLS_asFunctionalComponent(AppStatus, new AppStatus({
        ...{ 'onAction': {} },
        state: "error",
        message: (__VLS_ctx.loadError),
    }));
    const __VLS_8 = __VLS_7({
        ...{ 'onAction': {} },
        state: "error",
        message: (__VLS_ctx.loadError),
    }, ...__VLS_functionalComponentArgsRest(__VLS_7));
    let __VLS_10;
    let __VLS_11;
    let __VLS_12;
    const __VLS_13 = {
        onAction: (...[$event]) => {
            if (!!(__VLS_ctx.loadState === 'loading'))
                return;
            if (!(__VLS_ctx.loadState === 'error'))
                return;
            __VLS_ctx.loadState = 'ready';
        }
    };
    var __VLS_9;
}
else {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.h1, __VLS_intrinsicElements.h1)({});
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "muted" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.form, __VLS_intrinsicElements.form)({
        ...{ onSubmit: (__VLS_ctx.create) },
        ...{ class: "card" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.fieldset, __VLS_intrinsicElements.fieldset)({
        ...{ class: "field" },
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.legend, __VLS_intrinsicElements.legend)({
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ style: {} },
    });
    for (const [l] of __VLS_getVForSourceType((__VLS_ctx.levels))) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
            key: (l.id),
            ...{ class: "option-row" },
            ...{ style: {} },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
            type: "radio",
            name: "level",
            value: (l.id),
        });
        (__VLS_ctx.levelId);
        __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({});
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
    for (const [s] of __VLS_getVForSourceType((__VLS_ctx.subjects))) {
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
        for: "source",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.select, __VLS_intrinsicElements.select)({
        id: "source",
        value: (__VLS_ctx.sourceId),
        disabled: (__VLS_ctx.sourcesLoading),
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
        value: "",
    });
    for (const [source] of __VLS_getVForSourceType((__VLS_ctx.sources))) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
            key: (source.id),
            value: (source.id),
        });
        (source.name);
        (source.questionCount);
    }
    if (__VLS_ctx.sourcesLoading) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "muted" },
        });
    }
    else if (__VLS_ctx.sourcesError) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "error" },
        });
        (__VLS_ctx.sourcesError);
    }
    else if (__VLS_ctx.sources.length === 0) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "muted" },
        });
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.fieldset, __VLS_intrinsicElements.fieldset)({
        ...{ class: "field" },
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.legend, __VLS_intrinsicElements.legend)({
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        ...{ class: "option-row" },
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
        type: "radio",
        name: "mode",
        value: "comprehensive",
    });
    (__VLS_ctx.mode);
    __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({});
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        ...{ class: "option-row" },
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
        type: "radio",
        name: "mode",
        value: "knowledge",
    });
    (__VLS_ctx.mode);
    __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({});
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        ...{ class: "option-row" },
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
        type: "radio",
        name: "mode",
        value: "wrong_items",
    });
    (__VLS_ctx.mode);
    __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({});
    __VLS_asFunctionalElement(__VLS_intrinsicElements.fieldset, __VLS_intrinsicElements.fieldset)({
        ...{ class: "field" },
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.legend, __VLS_intrinsicElements.legend)({
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        ...{ class: "option-row" },
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
        type: "radio",
        name: "selectionOrder",
        value: "source_order",
    });
    (__VLS_ctx.selectionOrder);
    __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({});
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        ...{ class: "option-row" },
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
        type: "radio",
        name: "selectionOrder",
        value: "random",
    });
    (__VLS_ctx.selectionOrder);
    __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({});
    if (__VLS_ctx.mode === 'knowledge') {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "field" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
            ...{ style: {} },
        });
        if (__VLS_ctx.kpsLoading) {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
                ...{ class: "muted" },
            });
        }
        else {
            for (const [k] of __VLS_getVForSourceType((__VLS_ctx.kps))) {
                __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
                    key: (k.id),
                    ...{ class: "option-row" },
                });
                __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
                    type: "checkbox",
                    value: (k.id),
                });
                (__VLS_ctx.knowledgePointIds);
                __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({});
                (k.name);
                __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
                    ...{ class: "muted" },
                    ...{ style: {} },
                });
                (k.subjectName);
                (k.questionCount);
            }
            if (__VLS_ctx.kps.length === 0) {
                __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
                    ...{ class: "muted" },
                });
            }
        }
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.fieldset, __VLS_intrinsicElements.fieldset)({
        ...{ class: "field" },
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.legend, __VLS_intrinsicElements.legend)({
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ style: {} },
    });
    for (const [c] of __VLS_getVForSourceType(([10, 20, 30]))) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
            key: (c),
            ...{ class: "option-row" },
            ...{ style: {} },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
            type: "radio",
            name: "count",
            value: (c),
        });
        (__VLS_ctx.count);
        __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
            ...{ class: "mono" },
        });
        (c);
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        role: "status",
        ...{ class: "mono muted" },
        ...{ style: {} },
    });
    if (__VLS_ctx.availabilityLoading) {
    }
    else if (__VLS_ctx.availabilityError) {
        (__VLS_ctx.availabilityError);
    }
    else if (__VLS_ctx.availability !== null) {
        (__VLS_ctx.availability);
    }
    if (__VLS_ctx.insufficient) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "error-summary" },
            role: "alert",
        });
        (__VLS_ctx.availability);
        (__VLS_ctx.count);
    }
    if (__VLS_ctx.createError) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "error-summary" },
            role: "alert",
        });
        (__VLS_ctx.createError);
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
        ...{ class: "primary" },
        type: "submit",
        disabled: (!__VLS_ctx.canCreate || __VLS_ctx.creating),
    });
    (__VLS_ctx.creating ? '创建中…' : '开始练习');
}
var __VLS_2;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['option-row']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['error']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['option-row']} */ ;
/** @type {__VLS_StyleScopedClasses['option-row']} */ ;
/** @type {__VLS_StyleScopedClasses['option-row']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['option-row']} */ ;
/** @type {__VLS_StyleScopedClasses['option-row']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['option-row']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['option-row']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['error-summary']} */ ;
/** @type {__VLS_StyleScopedClasses['error-summary']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            AppShell: AppShell,
            AppStatus: AppStatus,
            loadState: loadState,
            loadError: loadError,
            levelId: levelId,
            subjectId: subjectId,
            sourceId: sourceId,
            mode: mode,
            selectionOrder: selectionOrder,
            knowledgePointIds: knowledgePointIds,
            count: count,
            availability: availability,
            availabilityLoading: availabilityLoading,
            availabilityError: availabilityError,
            creating: creating,
            createError: createError,
            kps: kps,
            kpsLoading: kpsLoading,
            sources: sources,
            sourcesLoading: sourcesLoading,
            sourcesError: sourcesError,
            levels: levels,
            subjects: subjects,
            insufficient: insufficient,
            canCreate: canCreate,
            create: create,
        };
    },
});
export default (await import('vue')).defineComponent({
    setup() {
        return {};
    },
});
; /* PartiallyEnd: #4569/main.vue */
