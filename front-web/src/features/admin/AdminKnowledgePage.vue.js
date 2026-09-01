import { onMounted, reactive, ref } from 'vue';
import { request, ApiError, fieldErrors } from '@/api/client';
import AppShell from '@/components/AppShell.vue';
import AppStatus from '@/components/AppStatus.vue';
import StatusBadge from '@/components/StatusBadge.vue';
const kps = ref([]);
const exams = ref([]);
const levelFilter = ref('');
const state = ref('loading');
const errorMessage = ref('');
const requestID = ref('');
const form = reactive({
    name: '',
    levelId: '',
    subjectId: '',
    parentId: '',
    description: '',
    commonMistakes: '',
    examples: '',
});
const fieldErr = reactive({});
const topError = ref('');
const saving = ref(false);
const subjects = ref([]);
async function load() {
    state.value = 'loading';
    try {
        const res = await request(`/admin/knowledge-points?levelId=${levelFilter.value}`);
        kps.value = res.knowledgePoints;
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
        // 忽略
    }
    await load();
});
function onLevelChange() {
    const level = exams.value.flatMap((e) => e.levels).find((l) => l.id === form.levelId);
    const exam = exams.value.find((e) => e.levels.some((l) => l.id === form.levelId));
    form.subjectId = '';
    subjects.value = exam?.subjects ?? [];
    void level;
}
async function create() {
    saving.value = true;
    topError.value = '';
    for (const k of Object.keys(fieldErr))
        delete fieldErr[k];
    try {
        await request('/admin/knowledge-points', {
            method: 'POST',
            body: {
                name: form.name,
                levelId: form.levelId,
                subjectId: form.subjectId,
                parentId: form.parentId || null,
                description: form.description,
                commonMistakes: form.commonMistakes,
                examples: form.examples,
            },
        });
        form.name = '';
        form.parentId = '';
        form.description = '';
        form.commonMistakes = '';
        form.examples = '';
        await load();
    }
    catch (err) {
        const fields = fieldErrors(err);
        for (const [k, v] of Object.entries(fields))
            fieldErr[k] = v;
        topError.value = Object.keys(fields).length ? '请检查表单' : err instanceof ApiError ? err.message : '创建失败';
    }
    finally {
        saving.value = false;
    }
}
async function publish(k) {
    try {
        await request(`/admin/knowledge-points/${k.id}`, {
            method: 'PATCH',
            body: { status: 'published' },
        });
        await load();
    }
    catch (err) {
        errorMessage.value = err instanceof ApiError ? err.message : '操作失败';
    }
}
async function unpublish(k) {
    try {
        await request(`/admin/knowledge-points/${k.id}`, {
            method: 'PATCH',
            body: { status: 'draft' },
        });
        await load();
    }
    catch (err) {
        errorMessage.value = err instanceof ApiError ? err.message : '操作失败';
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
__VLS_asFunctionalElement(__VLS_intrinsicElements.h1, __VLS_intrinsicElements.h1)({
    ...{ style: {} },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.form, __VLS_intrinsicElements.form)({
    ...{ onSubmit: (__VLS_ctx.create) },
    ...{ class: "card" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.h2, __VLS_intrinsicElements.h2)({
    ...{ style: {} },
});
if (__VLS_ctx.topError) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "error-summary" },
        role: "alert",
    });
    (__VLS_ctx.topError);
}
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "grid-2" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "field" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
    for: "k-level",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.select, __VLS_intrinsicElements.select)({
    ...{ onChange: (__VLS_ctx.onLevelChange) },
    id: "k-level",
    value: (__VLS_ctx.form.levelId),
    required: true,
});
for (const [l] of __VLS_getVForSourceType((__VLS_ctx.exams.flatMap((e) => e.levels)))) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
        key: (l.id),
        value: (l.id),
    });
    (l.name);
}
if (__VLS_ctx.fieldErr.levelId) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "error" },
    });
    (__VLS_ctx.fieldErr.levelId);
}
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "field" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
    for: "k-subject",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.select, __VLS_intrinsicElements.select)({
    id: "k-subject",
    value: (__VLS_ctx.form.subjectId),
    required: true,
});
for (const [s] of __VLS_getVForSourceType((__VLS_ctx.subjects))) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
        key: (s.id),
        value: (s.id),
    });
    (s.name);
}
if (__VLS_ctx.fieldErr.subjectId) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "error" },
    });
    (__VLS_ctx.fieldErr.subjectId);
}
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "grid-2" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "field" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
    for: "k-name",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
    id: "k-name",
    value: (__VLS_ctx.form.name),
    type: "text",
});
if (__VLS_ctx.fieldErr.name) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "error" },
    });
    (__VLS_ctx.fieldErr.name);
}
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "field" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
    for: "k-parent",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.select, __VLS_intrinsicElements.select)({
    id: "k-parent",
    value: (__VLS_ctx.form.parentId),
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
    value: "",
});
for (const [k] of __VLS_getVForSourceType((__VLS_ctx.kps.filter((x) => x.levelId === __VLS_ctx.form.levelId && x.subjectId === __VLS_ctx.form.subjectId)))) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
        key: (k.id),
        value: (k.id),
    });
    (k.name);
}
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "field" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
    for: "k-desc",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.textarea)({
    id: "k-desc",
    value: (__VLS_ctx.form.description),
    rows: "3",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "grid-2" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "field" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
    for: "k-mistakes",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.textarea)({
    id: "k-mistakes",
    value: (__VLS_ctx.form.commonMistakes),
    rows: "2",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "field" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
    for: "k-examples",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.textarea)({
    id: "k-examples",
    value: (__VLS_ctx.form.examples),
    rows: "2",
    lang: "ja",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
    ...{ class: "primary" },
    type: "submit",
    disabled: (__VLS_ctx.saving),
});
(__VLS_ctx.saving ? '创建中…' : '创建（草稿）');
__VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
    ...{ class: "muted" },
    ...{ style: {} },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "field" },
    ...{ style: {} },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
    for: "k-filter",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.select, __VLS_intrinsicElements.select)({
    ...{ onChange: (__VLS_ctx.load) },
    id: "k-filter",
    value: (__VLS_ctx.levelFilter),
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
else if (__VLS_ctx.kps.length === 0) {
    /** @type {[typeof AppStatus, ]} */ ;
    // @ts-ignore
    const __VLS_14 = __VLS_asFunctionalComponent(AppStatus, new AppStatus({
        state: "empty",
        message: "还没有知识点。",
    }));
    const __VLS_15 = __VLS_14({
        state: "empty",
        message: "还没有知识点。",
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
    __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({});
    __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({
        ...{ class: "num" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({});
    __VLS_asFunctionalElement(__VLS_intrinsicElements.tbody, __VLS_intrinsicElements.tbody)({});
    for (const [k] of __VLS_getVForSourceType((__VLS_ctx.kps))) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.tr, __VLS_intrinsicElements.tr)({
            key: (k.id),
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({});
        (k.name);
        __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({
            ...{ class: "mono" },
        });
        (k.levelId.slice(0, 6));
        __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({
            ...{ class: "mono" },
        });
        (k.subjectId.slice(0, 6));
        __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({});
        /** @type {[typeof StatusBadge, ]} */ ;
        // @ts-ignore
        const __VLS_17 = __VLS_asFunctionalComponent(StatusBadge, new StatusBadge({
            value: (k.status),
        }));
        const __VLS_18 = __VLS_17({
            value: (k.status),
        }, ...__VLS_functionalComponentArgsRest(__VLS_17));
        __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({
            ...{ class: "num" },
        });
        (k.questionCount);
        __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({});
        if (k.status === 'draft') {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
                ...{ onClick: (...[$event]) => {
                        if (!!(__VLS_ctx.state === 'loading'))
                            return;
                        if (!!(__VLS_ctx.state === 'error'))
                            return;
                        if (!!(__VLS_ctx.kps.length === 0))
                            return;
                        if (!(k.status === 'draft'))
                            return;
                        __VLS_ctx.publish(k);
                    } },
                ...{ style: {} },
            });
        }
        else if (k.status === 'published') {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
                ...{ onClick: (...[$event]) => {
                        if (!!(__VLS_ctx.state === 'loading'))
                            return;
                        if (!!(__VLS_ctx.state === 'error'))
                            return;
                        if (!!(__VLS_ctx.kps.length === 0))
                            return;
                        if (!!(k.status === 'draft'))
                            return;
                        if (!(k.status === 'published'))
                            return;
                        __VLS_ctx.unpublish(k);
                    } },
                ...{ style: {} },
            });
        }
    }
}
var __VLS_2;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['error-summary']} */ ;
/** @type {__VLS_StyleScopedClasses['grid-2']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['error']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['error']} */ ;
/** @type {__VLS_StyleScopedClasses['grid-2']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['error']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['grid-2']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['data']} */ ;
/** @type {__VLS_StyleScopedClasses['num']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
/** @type {__VLS_StyleScopedClasses['num']} */ ;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            AppShell: AppShell,
            AppStatus: AppStatus,
            StatusBadge: StatusBadge,
            kps: kps,
            exams: exams,
            levelFilter: levelFilter,
            state: state,
            errorMessage: errorMessage,
            requestID: requestID,
            form: form,
            fieldErr: fieldErr,
            topError: topError,
            saving: saving,
            subjects: subjects,
            load: load,
            onLevelChange: onLevelChange,
            create: create,
            publish: publish,
            unpublish: unpublish,
        };
    },
});
export default (await import('vue')).defineComponent({
    setup() {
        return {};
    },
});
; /* PartiallyEnd: #4569/main.vue */
