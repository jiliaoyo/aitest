import { computed } from 'vue';
import { authorityText, explanationSourceText, gradingStatusText, questionTypeText } from '@/app/format';
import ReportDialog from '@/features/issues/ReportDialog.vue';
const props = defineProps();
function answerText(answer, options) {
    if (!answer)
        return '—';
    if ('optionIds' in answer && answer.optionIds) {
        return answer.optionIds
            .map((id) => options.find((o) => o.id === id)?.label ?? id)
            .join('、');
    }
    if ('text' in answer && answer.text)
        return answer.text;
    return '—';
}
const userText = computed(() => answerText(props.item.userAnswer, props.item.options));
const correctText = computed(() => props.item.gradingStatus === 'pending' || props.item.gradingStatus === 'failed'
    ? '待 AI 判定'
    : answerText(props.item.correctAnswer, props.item.options));
const statusTone = computed(() => props.item.gradingStatus === 'correct'
    ? 'success'
    : props.item.gradingStatus === 'pending'
        ? 'accent'
        : props.item.gradingStatus === 'failed'
            ? 'danger'
            : props.item.gradingStatus === 'unanswered'
                ? 'warning'
                : 'danger');
const isAI = computed(() => props.item.gradingSource === 'ai');
const reportItemID = computed(() => props.item.id);
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
if (__VLS_ctx.item.sourceSectionName) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "muted" },
        ...{ style: {} },
    });
    (__VLS_ctx.item.sourceSectionName);
}
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ style: {} },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
    ...{ class: "tag" },
    'data-tone': (__VLS_ctx.statusTone),
});
(__VLS_ctx.gradingStatusText[__VLS_ctx.item.gradingStatus] ?? __VLS_ctx.item.gradingStatus);
if (__VLS_ctx.isAI) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
        ...{ class: "tag" },
        'data-tone': "accent",
    });
}
else if (__VLS_ctx.item.answerAuthority) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
        ...{ class: "tag" },
        'data-tone': "success",
    });
    (__VLS_ctx.authorityText[__VLS_ctx.item.answerAuthority]);
}
if (__VLS_ctx.item.material) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.section, __VLS_intrinsicElements.section)({
        ...{ class: "card" },
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "muted" },
        ...{ style: {} },
    });
    (__VLS_ctx.item.material.title ? ` · ${__VLS_ctx.item.material.title}` : '');
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "material-text" },
        ...{ style: {} },
        lang: "ja",
    });
    (__VLS_ctx.item.material.content);
}
__VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
    ...{ style: {} },
    lang: "ja",
});
(__VLS_ctx.item.stem);
__VLS_asFunctionalElement(__VLS_intrinsicElements.dl, __VLS_intrinsicElements.dl)({
    ...{ style: {} },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.dt, __VLS_intrinsicElements.dt)({
    ...{ class: "muted" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.dd, __VLS_intrinsicElements.dd)({
    ...{ class: "mono" },
    ...{ style: {} },
});
(__VLS_ctx.userText);
__VLS_asFunctionalElement(__VLS_intrinsicElements.dt, __VLS_intrinsicElements.dt)({
    ...{ class: "muted" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.dd, __VLS_intrinsicElements.dd)({
    ...{ class: "mono" },
    ...{ style: {} },
});
(__VLS_ctx.correctText);
if (__VLS_ctx.item.knowledgePoints.length) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.dt, __VLS_intrinsicElements.dt)({
        ...{ class: "muted" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.dd, __VLS_intrinsicElements.dd)({
        ...{ style: {} },
    });
    (__VLS_ctx.item.knowledgePoints.map((k) => k.name).join('、'));
}
if (__VLS_ctx.item.explanation) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "tag" },
        'data-tone': "neutral",
        ...{ style: {} },
    });
    (__VLS_ctx.explanationSourceText[__VLS_ctx.item.explanation.source] ?? __VLS_ctx.item.explanation.source);
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ style: {} },
    });
    (__VLS_ctx.item.explanation.text);
}
else if (__VLS_ctx.item.gradingStatus === 'failed') {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "muted" },
        ...{ style: {} },
    });
}
__VLS_asFunctionalElement(__VLS_intrinsicElements.footer, __VLS_intrinsicElements.footer)({
    ...{ style: {} },
});
/** @type {[typeof ReportDialog, ]} */ ;
// @ts-ignore
const __VLS_0 = __VLS_asFunctionalComponent(ReportDialog, new ReportDialog({
    practiceItemId: (__VLS_ctx.reportItemID),
}));
const __VLS_1 = __VLS_0({
    practiceItemId: (__VLS_ctx.reportItemID),
}, ...__VLS_functionalComponentArgsRest(__VLS_0));
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['tag']} */ ;
/** @type {__VLS_StyleScopedClasses['tag']} */ ;
/** @type {__VLS_StyleScopedClasses['tag']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['material-text']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['tag']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            authorityText: authorityText,
            explanationSourceText: explanationSourceText,
            gradingStatusText: gradingStatusText,
            questionTypeText: questionTypeText,
            ReportDialog: ReportDialog,
            userText: userText,
            correctText: correctText,
            statusTone: statusTone,
            isAI: isAI,
            reportItemID: reportItemID,
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
