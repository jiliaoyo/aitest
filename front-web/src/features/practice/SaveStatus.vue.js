import { computed } from 'vue';
import { formatTime } from '@/app/format';
const props = defineProps();
const emit = defineEmits();
const text = computed(() => {
    if (props.state === 'saving')
        return '保存中…';
    if (props.state === 'error')
        return '保存失败';
    if (props.localOnly)
        return '尚未同步';
    if (props.state === 'saved')
        return `已保存 ${formatTime(props.savedAt)}`;
    return '';
});
debugger; /* PartiallyEnd: #3632/scriptSetup.vue */
const __VLS_ctx = {};
let __VLS_components;
let __VLS_directives;
__VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
    ...{ class: "muted" },
    ...{ style: {} },
    role: "status",
    'aria-live': "polite",
});
if (__VLS_ctx.anyError && __VLS_ctx.state === 'error') {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
        ...{ onClick: (...[$event]) => {
                if (!(__VLS_ctx.anyError && __VLS_ctx.state === 'error'))
                    return;
                __VLS_ctx.emit('retry');
            } },
        type: "button",
        ...{ class: "ghost" },
        ...{ style: {} },
    });
}
else {
    (__VLS_ctx.text);
}
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['ghost']} */ ;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            emit: emit,
            text: text,
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
