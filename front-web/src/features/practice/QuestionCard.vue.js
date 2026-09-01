import { computed } from 'vue';
import { questionTypeText } from '@/app/format';
import MaterialPanel from './MaterialPanel.vue';
const props = defineProps();
const emit = defineEmits();
const selectedIds = computed(() => props.answer && 'optionIds' in props.answer ? props.answer.optionIds : []);
const textValue = computed(() => (props.answer && 'text' in props.answer ? props.answer.text ?? '' : ''));
function selectSingle(optionID) {
    emit('update:answer', { optionIds: [optionID] });
}
function toggleMulti(optionID) {
    const current = new Set(selectedIds.value);
    if (current.has(optionID)) {
        current.delete(optionID);
    }
    else {
        current.add(optionID);
    }
    emit('update:answer', { optionIds: [...current] });
}
function inputText(event) {
    const value = event.target.value;
    emit('update:answer', value === '' ? null : { text: value });
}
debugger; /* PartiallyEnd: #3632/scriptSetup.vue */
const __VLS_ctx = {};
let __VLS_components;
let __VLS_directives;
__VLS_asFunctionalElement(__VLS_intrinsicElements.article, __VLS_intrinsicElements.article)({
    ...{ class: "card" },
    lang: "ja",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.header, __VLS_intrinsicElements.header)({
    ...{ style: {} },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
    ...{ class: "mono muted" },
    ...{ style: {} },
});
(__VLS_ctx.item.position);
(__VLS_ctx.questionTypeText[__VLS_ctx.item.type] ?? __VLS_ctx.item.type);
__VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
    ...{ class: "muted" },
    ...{ style: {} },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
    ...{ onChange: (...[$event]) => {
            __VLS_ctx.emit('update:marked', $event.target.checked);
        } },
    type: "checkbox",
    checked: (__VLS_ctx.marked),
});
if (__VLS_ctx.item.material) {
    /** @type {[typeof MaterialPanel, ]} */ ;
    // @ts-ignore
    const __VLS_0 = __VLS_asFunctionalComponent(MaterialPanel, new MaterialPanel({
        ...{ 'onToggle': {} },
        material: (__VLS_ctx.item.material),
        collapsed: (__VLS_ctx.materialCollapsed),
    }));
    const __VLS_1 = __VLS_0({
        ...{ 'onToggle': {} },
        material: (__VLS_ctx.item.material),
        collapsed: (__VLS_ctx.materialCollapsed),
    }, ...__VLS_functionalComponentArgsRest(__VLS_0));
    let __VLS_3;
    let __VLS_4;
    let __VLS_5;
    const __VLS_6 = {
        onToggle: (...[$event]) => {
            if (!(__VLS_ctx.item.material))
                return;
            __VLS_ctx.emit('toggle-material');
        }
    };
    var __VLS_2;
}
__VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
    ...{ class: "stem" },
    ...{ style: {} },
    lang: "ja",
});
(__VLS_ctx.item.stem);
__VLS_asFunctionalElement(__VLS_intrinsicElements.fieldset, __VLS_intrinsicElements.fieldset)({
    ...{ style: {} },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.legend, __VLS_intrinsicElements.legend)({
    ...{ class: "visually-hidden-ish" },
    ...{ style: {} },
});
(__VLS_ctx.questionTypeText[__VLS_ctx.item.type]);
if (__VLS_ctx.item.type === 'single_choice') {
    for (const [opt] of __VLS_getVForSourceType((__VLS_ctx.item.options))) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
            key: (opt.id),
            ...{ class: "option-row" },
            lang: "ja",
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
            ...{ onChange: (...[$event]) => {
                    if (!(__VLS_ctx.item.type === 'single_choice'))
                        return;
                    __VLS_ctx.selectSingle(opt.id);
                } },
            type: "radio",
            name: (`q-${__VLS_ctx.item.id}`),
            value: (opt.id),
            checked: (__VLS_ctx.selectedIds.includes(opt.id)),
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({});
        __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
            ...{ class: "mono" },
        });
        (opt.label);
        (opt.text);
    }
}
else if (__VLS_ctx.item.type === 'multiple_choice') {
    for (const [opt] of __VLS_getVForSourceType((__VLS_ctx.item.options))) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
            key: (opt.id),
            ...{ class: "option-row" },
            lang: "ja",
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
            ...{ onChange: (...[$event]) => {
                    if (!!(__VLS_ctx.item.type === 'single_choice'))
                        return;
                    if (!(__VLS_ctx.item.type === 'multiple_choice'))
                        return;
                    __VLS_ctx.toggleMulti(opt.id);
                } },
            type: "checkbox",
            checked: (__VLS_ctx.selectedIds.includes(opt.id)),
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({});
        __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
            ...{ class: "mono" },
        });
        (opt.label);
        (opt.text);
    }
}
else if (__VLS_ctx.item.type === 'fill_blank') {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "field" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        for: (`input-${__VLS_ctx.item.id}`),
        lang: "zh-CN",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
        ...{ onInput: (__VLS_ctx.inputText) },
        id: (`input-${__VLS_ctx.item.id}`),
        type: "text",
        lang: "ja",
        value: (__VLS_ctx.textValue),
        autocomplete: "off",
    });
}
else {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "field" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        for: (`input-${__VLS_ctx.item.id}`),
        lang: "zh-CN",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.textarea)({
        ...{ onInput: (__VLS_ctx.inputText) },
        id: (`input-${__VLS_ctx.item.id}`),
        rows: "3",
        lang: "ja",
        value: (__VLS_ctx.textValue),
    });
}
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['stem']} */ ;
/** @type {__VLS_StyleScopedClasses['visually-hidden-ish']} */ ;
/** @type {__VLS_StyleScopedClasses['option-row']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
/** @type {__VLS_StyleScopedClasses['option-row']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            questionTypeText: questionTypeText,
            MaterialPanel: MaterialPanel,
            emit: emit,
            selectedIds: selectedIds,
            textValue: textValue,
            selectSingle: selectSingle,
            toggleMulti: toggleMulti,
            inputText: inputText,
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
