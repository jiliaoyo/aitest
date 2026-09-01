import { ref } from 'vue';
import { request, ApiError, fieldErrors } from '@/api/client';
const props = defineProps();
const emit = defineEmits();
const open = ref(false);
const targetType = ref('stem');
const description = ref('');
const submitting = ref(false);
const done = ref(false);
const error = ref('');
const typeLabels = [
    { value: 'stem', label: '题干有误' },
    { value: 'answer', label: '答案有误' },
    { value: 'explanation', label: '解析有误' },
    { value: 'classification', label: '分类不对' },
    { value: 'ai_grading', label: 'AI 判定不可信' },
];
async function submit() {
    submitting.value = true;
    error.value = '';
    try {
        await request('/issue-reports', {
            method: 'POST',
            body: { practiceItemId: props.practiceItemId, targetType: targetType.value, description: description.value },
        });
        done.value = true;
        open.value = false;
        emit('submitted');
    }
    catch (err) {
        const fields = fieldErrors(err);
        error.value = Object.values(fields)[0] ?? (err instanceof ApiError ? err.message : '提交失败，请重试');
    }
    finally {
        submitting.value = false;
    }
}
debugger; /* PartiallyEnd: #3632/scriptSetup.vue */
const __VLS_ctx = {};
let __VLS_components;
let __VLS_directives;
__VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({});
if (!__VLS_ctx.done) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
        ...{ onClick: (...[$event]) => {
                if (!(!__VLS_ctx.done))
                    return;
                __VLS_ctx.open = true;
            } },
        type: "button",
        ...{ class: "ghost" },
        ...{ style: {} },
    });
}
else {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
        ...{ class: "tag" },
    });
}
const __VLS_0 = {}.Teleport;
/** @type {[typeof __VLS_components.Teleport, typeof __VLS_components.Teleport, ]} */ ;
// @ts-ignore
const __VLS_1 = __VLS_asFunctionalComponent(__VLS_0, new __VLS_0({
    to: "body",
}));
const __VLS_2 = __VLS_1({
    to: "body",
}, ...__VLS_functionalComponentArgsRest(__VLS_1));
__VLS_3.slots.default;
if (__VLS_ctx.open) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ onMousedown: (...[$event]) => {
                if (!(__VLS_ctx.open))
                    return;
                __VLS_ctx.open = false;
            } },
        ...{ class: "dialog-backdrop" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "dialog" },
        role: "dialog",
        'aria-modal': "true",
        'aria-label': "举报题目问题",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.h2, __VLS_intrinsicElements.h2)({
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "field" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        for: "report-type",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.select, __VLS_intrinsicElements.select)({
        id: "report-type",
        value: (__VLS_ctx.targetType),
    });
    for (const [t] of __VLS_getVForSourceType((__VLS_ctx.typeLabels))) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
            key: (t.value),
            value: (t.value),
        });
        (t.label);
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "field" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        for: "report-desc",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.textarea)({
        id: "report-desc",
        value: (__VLS_ctx.description),
        rows: "3",
        maxlength: "2000",
    });
    if (__VLS_ctx.error) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "error-summary" },
            role: "alert",
        });
        (__VLS_ctx.error);
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "dialog-actions" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
        ...{ onClick: (...[$event]) => {
                if (!(__VLS_ctx.open))
                    return;
                __VLS_ctx.open = false;
            } },
        type: "button",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
        ...{ onClick: (__VLS_ctx.submit) },
        type: "button",
        ...{ class: "primary" },
        disabled: (__VLS_ctx.submitting),
    });
    (__VLS_ctx.submitting ? '提交中' : '提交反馈');
}
var __VLS_3;
/** @type {__VLS_StyleScopedClasses['ghost']} */ ;
/** @type {__VLS_StyleScopedClasses['tag']} */ ;
/** @type {__VLS_StyleScopedClasses['dialog-backdrop']} */ ;
/** @type {__VLS_StyleScopedClasses['dialog']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['error-summary']} */ ;
/** @type {__VLS_StyleScopedClasses['dialog-actions']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            open: open,
            targetType: targetType,
            description: description,
            submitting: submitting,
            done: done,
            error: error,
            typeLabels: typeLabels,
            submit: submit,
        };
    },
    __typeEmits: {},
    __typeProps: {},
});
export default (await import('vue')).defineComponent({
    setup() {
        return {};
    },
    __typeEmits: {},
    __typeProps: {},
});
; /* PartiallyEnd: #4569/main.vue */
