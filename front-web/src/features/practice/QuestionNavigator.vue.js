const props = defineProps();
const emit = defineEmits();
function stateOf(item, index) {
    const parts = [];
    if (props.isAnswered(item))
        parts.push('answered');
    if (props.isMarked(item))
        parts.push('marked');
    if (index === props.currentIndex)
        parts.push('current');
    return parts.join(' ') || 'unanswered';
}
debugger; /* PartiallyEnd: #3632/scriptSetup.vue */
const __VLS_ctx = {};
let __VLS_components;
let __VLS_directives;
__VLS_asFunctionalElement(__VLS_intrinsicElements.nav, __VLS_intrinsicElements.nav)({
    'aria-label': "题目导航",
    ...{ class: "navigator" },
});
for (const [item, index] of __VLS_getVForSourceType((__VLS_ctx.items))) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
        ...{ onClick: (...[$event]) => {
                __VLS_ctx.emit('select', index);
            } },
        key: (item.id),
        type: "button",
        'data-state': (__VLS_ctx.stateOf(item, index)),
        'aria-label': (`第 ${item.position} 题${props.isAnswered(item) ? '，已答' : '，未答'}${props.isMarked(item) ? '，已标记' : ''}`),
        'aria-current': (index === __VLS_ctx.currentIndex ? 'true' : undefined),
    });
    (item.position);
}
/** @type {__VLS_StyleScopedClasses['navigator']} */ ;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            emit: emit,
            stateOf: stateOf,
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
