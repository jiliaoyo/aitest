import { nextTick, onMounted, ref } from 'vue';
const props = defineProps();
const emit = defineEmits();
const panel = ref(null);
const previouslyFocused = ref(null);
async function onKeydown(event) {
    if (event.key === 'Escape') {
        event.preventDefault();
        emit('cancel');
        return;
    }
    if (event.key === 'Tab' && panel.value) {
        const focusables = panel.value.querySelectorAll('button, a[href], input, textarea, select');
        if (focusables.length === 0)
            return;
        const first = focusables[0];
        const last = focusables[focusables.length - 1];
        if (event.shiftKey && document.activeElement === first) {
            event.preventDefault();
            last.focus();
        }
        else if (!event.shiftKey && document.activeElement === last) {
            event.preventDefault();
            first.focus();
        }
    }
}
onMounted(async () => {
    if (props.open) {
        previouslyFocused.value = document.activeElement;
        await nextTick();
        panel.value?.querySelector('button')?.focus();
    }
});
const __VLS_exposed = {
    focusPanel: async () => {
        previouslyFocused.value = document.activeElement;
        await nextTick();
        panel.value?.querySelector('button')?.focus();
    },
    restoreFocus: () => {
        previouslyFocused.value?.focus();
    },
};
defineExpose(__VLS_exposed);
debugger; /* PartiallyEnd: #3632/scriptSetup.vue */
const __VLS_ctx = {};
let __VLS_components;
let __VLS_directives;
if (__VLS_ctx.open) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ onKeydown: (__VLS_ctx.onKeydown) },
        ...{ onMousedown: (...[$event]) => {
                if (!(__VLS_ctx.open))
                    return;
                __VLS_ctx.emit('cancel');
            } },
        ...{ class: "dialog-backdrop" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ref: "panel",
        ...{ class: "dialog" },
        role: "dialog",
        'aria-modal': "true",
        'aria-label': (__VLS_ctx.title),
    });
    /** @type {typeof __VLS_ctx.panel} */ ;
    __VLS_asFunctionalElement(__VLS_intrinsicElements.h2, __VLS_intrinsicElements.h2)({
        ...{ style: {} },
    });
    (__VLS_ctx.title);
    var __VLS_0 = {};
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "dialog-actions" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
        ...{ onClick: (...[$event]) => {
                if (!(__VLS_ctx.open))
                    return;
                __VLS_ctx.emit('cancel');
            } },
        type: "button",
    });
    (__VLS_ctx.cancelLabel ?? '继续答题');
    __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
        ...{ onClick: (...[$event]) => {
                if (!(__VLS_ctx.open))
                    return;
                __VLS_ctx.emit('confirm');
            } },
        type: "button",
        ...{ class: (__VLS_ctx.tone === 'danger' ? 'danger' : 'primary') },
    });
    (__VLS_ctx.confirmLabel ?? '确认');
}
/** @type {__VLS_StyleScopedClasses['dialog-backdrop']} */ ;
/** @type {__VLS_StyleScopedClasses['dialog']} */ ;
/** @type {__VLS_StyleScopedClasses['dialog-actions']} */ ;
// @ts-ignore
var __VLS_1 = __VLS_0;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            emit: emit,
            panel: panel,
            onKeydown: onKeydown,
        };
    },
    __typeEmits: {},
    __typeProps: {},
});
const __VLS_component = (await import('vue')).defineComponent({
    setup() {
        return {
            ...__VLS_exposed,
        };
    },
    __typeEmits: {},
    __typeProps: {},
});
export default {};
; /* PartiallyEnd: #4569/main.vue */
