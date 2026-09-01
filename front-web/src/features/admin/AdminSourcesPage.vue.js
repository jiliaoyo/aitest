import { onMounted, reactive, ref } from 'vue';
import { request, ApiError, fieldErrors } from '@/api/client';
import AppShell from '@/components/AppShell.vue';
import AppStatus from '@/components/AppStatus.vue';
import { sourceKindText } from '@/app/format';
const sources = ref([]);
const state = ref('loading');
const errorMessage = ref('');
const requestID = ref('');
const form = reactive({
    name: '',
    kind: 'self_made',
    author: '',
    publisher: '',
    year: '',
    licenseNote: '',
    internalNote: '',
});
const sectionNames = reactive(new Map());
const fieldErr = reactive({});
const topError = ref('');
const saving = ref(false);
async function load() {
    state.value = 'loading';
    try {
        const res = await request('/admin/sources');
        sources.value = res.sources;
        state.value = 'ready';
    }
    catch (err) {
        errorMessage.value = err instanceof ApiError ? err.message : '加载失败';
        requestID.value = err instanceof ApiError ? err.requestId ?? '' : '';
        state.value = 'error';
    }
}
onMounted(load);
async function create() {
    saving.value = true;
    topError.value = '';
    for (const k of Object.keys(fieldErr))
        delete fieldErr[k];
    try {
        await request('/admin/sources', {
            method: 'POST',
            body: {
                name: form.name,
                kind: form.kind,
                author: form.author,
                publisher: form.publisher,
                year: form.year === '' ? null : Number(form.year),
                licenseNote: form.licenseNote,
                internalNote: form.internalNote,
            },
        });
        form.name = '';
        form.author = '';
        form.publisher = '';
        form.year = '';
        form.licenseNote = '';
        form.internalNote = '';
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
async function addSection(source) {
    const name = sectionNames.get(source.id)?.trim();
    if (!name)
        return;
    try {
        await request(`/admin/sources/${source.id}/sections`, { method: 'POST', body: { name } });
        sectionNames.set(source.id, '');
        await load();
    }
    catch (err) {
        errorMessage.value = err instanceof ApiError ? err.message : '创建章节失败';
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
__VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
    ...{ class: "muted" },
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
    for: "s-name",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
    id: "s-name",
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
    for: "s-kind",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.select, __VLS_intrinsicElements.select)({
    id: "s-kind",
    value: (__VLS_ctx.form.kind),
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
    value: "book",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
    value: "past_exam",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
    value: "self_made",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
    value: "ai_generated",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "grid-3" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "field" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
    for: "s-author",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
    id: "s-author",
    value: (__VLS_ctx.form.author),
    type: "text",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "field" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
    for: "s-publisher",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
    id: "s-publisher",
    value: (__VLS_ctx.form.publisher),
    type: "text",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "field" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
    for: "s-year",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
    id: "s-year",
    type: "number",
    min: "1900",
    max: "2100",
});
(__VLS_ctx.form.year);
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "field" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
    for: "s-license",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.textarea)({
    id: "s-license",
    value: (__VLS_ctx.form.licenseNote),
    rows: "2",
});
if (__VLS_ctx.fieldErr.licenseNote) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "error" },
    });
    (__VLS_ctx.fieldErr.licenseNote);
}
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "field" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
    for: "s-note",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.textarea)({
    id: "s-note",
    value: (__VLS_ctx.form.internalNote),
    rows: "2",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
    ...{ class: "primary" },
    type: "submit",
    disabled: (__VLS_ctx.saving),
});
(__VLS_ctx.saving ? '创建中…' : '创建来源');
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
else if (__VLS_ctx.sources.length === 0) {
    /** @type {[typeof AppStatus, ]} */ ;
    // @ts-ignore
    const __VLS_14 = __VLS_asFunctionalComponent(AppStatus, new AppStatus({
        state: "empty",
        message: "还没有来源。",
    }));
    const __VLS_15 = __VLS_14({
        state: "empty",
        message: "还没有来源。",
    }, ...__VLS_functionalComponentArgsRest(__VLS_14));
}
else {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ style: {} },
    });
    for (const [s] of __VLS_getVForSourceType((__VLS_ctx.sources))) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.article, __VLS_intrinsicElements.article)({
            key: (s.id),
            ...{ class: "card" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "page-header" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({});
        __VLS_asFunctionalElement(__VLS_intrinsicElements.h2, __VLS_intrinsicElements.h2)({
            ...{ style: {} },
        });
        (s.name);
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "muted" },
            ...{ style: {} },
        });
        (__VLS_ctx.sourceKindText[s.kind] ?? s.kind);
        if (s.author) {
            (s.author);
        }
        if (s.publisher) {
            (s.publisher);
        }
        if (s.year) {
            (s.year);
        }
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "muted" },
            ...{ style: {} },
        });
        (s.licenseNote || '（未填写）');
        __VLS_asFunctionalElement(__VLS_intrinsicElements.ul, __VLS_intrinsicElements.ul)({
            ...{ style: {} },
        });
        for (const [sec] of __VLS_getVForSourceType((s.sections))) {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.li, __VLS_intrinsicElements.li)({
                key: (sec.id),
                ...{ class: "mono" },
                ...{ style: {} },
            });
            (sec.name);
        }
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ style: {} },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
            ...{ onInput: (...[$event]) => {
                    if (!!(__VLS_ctx.state === 'loading'))
                        return;
                    if (!!(__VLS_ctx.state === 'error'))
                        return;
                    if (!!(__VLS_ctx.sources.length === 0))
                        return;
                    __VLS_ctx.sectionNames.set(s.id, $event.target.value);
                } },
            value: (__VLS_ctx.sectionNames.get(s.id) ?? ''),
            type: "text",
            placeholder: "新章节名称",
            ...{ style: {} },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
            ...{ onClick: (...[$event]) => {
                    if (!!(__VLS_ctx.state === 'loading'))
                        return;
                    if (!!(__VLS_ctx.state === 'error'))
                        return;
                    if (!!(__VLS_ctx.sources.length === 0))
                        return;
                    __VLS_ctx.addSection(s);
                } },
        });
    }
}
var __VLS_2;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['error-summary']} */ ;
/** @type {__VLS_StyleScopedClasses['grid-2']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['error']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['grid-3']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['error']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['page-header']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            AppShell: AppShell,
            AppStatus: AppStatus,
            sourceKindText: sourceKindText,
            sources: sources,
            state: state,
            errorMessage: errorMessage,
            requestID: requestID,
            form: form,
            sectionNames: sectionNames,
            fieldErr: fieldErr,
            topError: topError,
            saving: saving,
            load: load,
            create: create,
            addSection: addSection,
        };
    },
});
export default (await import('vue')).defineComponent({
    setup() {
        return {};
    },
});
; /* PartiallyEnd: #4569/main.vue */
