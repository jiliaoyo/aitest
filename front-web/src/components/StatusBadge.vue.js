import { computed } from 'vue';
import { authorityText, explanationSourceText, gradingStatusText, sessionStatusText, statusText } from '@/app/format';
const props = defineProps();
const text = computed(() => {
    switch (props.kind) {
        case 'session':
            return sessionStatusText[props.value] ?? props.value;
        case 'grading':
            return gradingStatusText[props.value] ?? props.value;
        case 'authority':
            return authorityText[props.value] ?? props.value;
        case 'explanation':
            return explanationSourceText[props.value] ?? props.value;
        case 'issue':
            return statusText[props.value] ?? props.value;
        default:
            return statusText[props.value] ?? props.value;
    }
});
const tone = computed(() => {
    switch (props.value) {
        case 'correct':
        case 'completed':
        case 'published':
        case 'official':
        case 'human_verified':
        case 'resolved':
        case 'approved':
            return 'success';
        case 'pending':
        case 'grading':
        case 'in_review':
        case 'draft':
        case 'unanswered':
        case 'uploaded':
        case 'extracting':
        case 'structuring':
        case 'review_ready':
            return 'warning';
        case 'incorrect':
        case 'failed':
        case 'analysis_failed':
        case 'retired':
        case 'rejected':
            return 'danger';
        case 'active':
        case 'ai':
            return 'accent';
        default:
            return 'neutral';
    }
});
debugger; /* PartiallyEnd: #3632/scriptSetup.vue */
const __VLS_ctx = {};
let __VLS_components;
let __VLS_directives;
__VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
    ...{ class: "tag" },
    'data-tone': (__VLS_ctx.tone),
});
(__VLS_ctx.text);
/** @type {__VLS_StyleScopedClasses['tag']} */ ;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            text: text,
            tone: tone,
        };
    },
    __typeProps: {},
});
export default (await import('vue')).defineComponent({
    setup() {
        return {};
    },
    __typeProps: {},
});
; /* PartiallyEnd: #4569/main.vue */
