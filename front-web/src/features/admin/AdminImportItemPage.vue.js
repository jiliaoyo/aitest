import { computed, onMounted, reactive, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import { request, ApiError, fieldErrors } from '@/api/client';
import AppShell from '@/components/AppShell.vue';
import AppStatus from '@/components/AppStatus.vue';
import ConfirmDialog from '@/components/ConfirmDialog.vue';
import StatusBadge from '@/components/StatusBadge.vue';
import { questionTypeText } from '@/app/format';
const route = useRoute();
const itemID = route.params.importItemId;
const item = ref(null);
const exams = ref([]);
const sources = ref([]);
const kps = ref([]);
const state = ref('loading');
const errorMessage = ref('');
const requestID = ref('');
const topError = ref('');
const info = ref('');
const saving = ref(false);
const approving = ref(false);
const publishing = ref(false);
const confirmPublish = ref(false);
const fieldErr = reactive({});
const form = reactive({
    materialKey: '',
    type: 'single_choice',
    stem: '',
    options: [],
    materialTitle: '',
    materialContent: '',
    levelId: '',
    subjectId: '',
    sourceSectionId: '',
    difficulty: 3,
    knowledgePointIds: [],
    hasAnswer: false,
    correctOptionIds: [],
    acceptableText: '',
    referenceText: '',
    authority: 'official',
    explanation: '',
});
const sourceAnswer = ref();
const aiSuggestedAnswer = ref();
const isChoice = computed(() => form.type === 'single_choice' || form.type === 'multiple_choice');
const sections = computed(() => sources.value.flatMap((source) => source.sections.map((section) => ({ ...section, sourceName: source.name }))));
const suggestionText = computed(() => aiSuggestedAnswer.value ? JSON.stringify(aiSuggestedAnswer.value.value) : '');
function stringList(value) {
    return Array.isArray(value) ? value.filter((v) => typeof v === 'string') : [];
}
function optionIds(value) {
    return stringList(value?.optionIds);
}
function acceptable(value) {
    return stringList(value?.acceptable);
}
function applyDraft(draft) {
    form.materialKey = draft.materialKey ?? '';
    form.type = draft.type;
    form.stem = draft.stem;
    form.options = draft.options ? JSON.parse(JSON.stringify(draft.options)) : [];
    form.materialTitle = draft.materialTitle ?? '';
    form.materialContent = draft.materialContent ?? '';
    form.levelId = draft.levelId;
    form.subjectId = draft.subjectId;
    form.sourceSectionId = draft.sourceSectionId ?? '';
    form.difficulty = draft.difficulty;
    form.knowledgePointIds = [...draft.knowledgePointIds];
    sourceAnswer.value = draft.sourceAnswer;
    aiSuggestedAnswer.value = draft.aiSuggestedAnswer;
    form.hasAnswer = draft.answer !== undefined;
    form.authority = draft.answer?.authority ?? 'official';
    form.explanation = draft.answer?.explanation ?? '';
    form.correctOptionIds = optionIds(draft.answer?.value);
    form.acceptableText = acceptable(draft.answer?.value).join('\n');
    form.referenceText = typeof draft.answer?.value.reference === 'string' ? draft.answer.value.reference : '';
}
async function load() {
    state.value = 'loading';
    try {
        const [itemRes, catalogRes, sourceRes] = await Promise.all([
            request(`/admin/import-items/${itemID}`),
            request('/catalog'),
            request('/admin/sources'),
        ]);
        item.value = itemRes.item;
        exams.value = catalogRes.exams;
        sources.value = sourceRes.sources;
        if (item.value.draft)
            applyDraft(item.value.draft);
        await loadKPs();
        state.value = 'ready';
    }
    catch (err) {
        errorMessage.value = err instanceof ApiError ? err.message : '加载失败';
        requestID.value = err instanceof ApiError ? err.requestId ?? '' : '';
        state.value = 'error';
    }
}
async function loadKPs() {
    if (!form.levelId)
        return;
    const params = new URLSearchParams({ levelId: form.levelId });
    if (form.subjectId)
        params.set('subjectId', form.subjectId);
    const res = await request(`/admin/knowledge-points?${params}`);
    kps.value = res.knowledgePoints;
}
watch(() => [form.levelId, form.subjectId], () => void loadKPs());
function ensureOptionCount() {
    while (form.options.length < 2) {
        const index = form.options.length;
        form.options.push({ id: String.fromCharCode(97 + index), label: String.fromCharCode(65 + index), text: '' });
    }
}
function addOption() {
    const index = form.options.length;
    form.options.push({ id: `option-${index + 1}`, label: String.fromCharCode(65 + index), text: '' });
}
function removeOption(index) {
    if (form.options.length <= 2)
        return;
    form.options.splice(index, 1);
}
function answerValue() {
    if (isChoice.value)
        return form.correctOptionIds.length ? { optionIds: [...form.correctOptionIds] } : null;
    if (form.type === 'fill_blank') {
        const values = form.acceptableText.split('\n').map((value) => value.trim()).filter(Boolean);
        return values.length ? { acceptable: values } : null;
    }
    return form.referenceText.trim() ? { reference: form.referenceText.trim() } : null;
}
function buildDraft() {
    const draft = {
        materialKey: form.materialKey || undefined,
        type: form.type,
        stem: form.stem,
        options: isChoice.value ? form.options : [],
        materialTitle: form.materialTitle,
        materialContent: form.materialContent,
        levelId: form.levelId,
        subjectId: form.subjectId,
        sourceSectionId: form.sourceSectionId || null,
        difficulty: form.difficulty,
        knowledgePointIds: form.knowledgePointIds,
        sourceAnswer: sourceAnswer.value,
        aiSuggestedAnswer: aiSuggestedAnswer.value,
    };
    const value = form.hasAnswer ? answerValue() : null;
    if (value)
        draft.answer = { value, authority: form.authority, explanation: form.explanation };
    return draft;
}
async function save() {
    saving.value = true;
    topError.value = '';
    info.value = '';
    for (const key of Object.keys(fieldErr))
        delete fieldErr[key];
    try {
        const res = await request(`/admin/import-items/${itemID}`, { method: 'PATCH', body: { draft: buildDraft() } });
        item.value = res.item;
        info.value = '草稿已保存；保存后需要重新审核。';
    }
    catch (err) {
        const fields = fieldErrors(err);
        for (const [key, value] of Object.entries(fields))
            fieldErr[key] = value;
        topError.value = Object.keys(fields).length ? '请修正表单错误' : err instanceof ApiError ? err.message : '保存失败，请重试';
    }
    finally {
        saving.value = false;
    }
}
async function approve() {
    approving.value = true;
    topError.value = '';
    try {
        const res = await request(`/admin/import-items/${itemID}/approve`, { method: 'POST' });
        item.value = res.item;
        info.value = '已审核，可以发布。';
    }
    catch (err) {
        topError.value = err instanceof ApiError ? err.message : '审核失败';
    }
    finally {
        approving.value = false;
    }
}
function askPublish() {
    confirmPublish.value = true;
}
async function publish() {
    publishing.value = true;
    topError.value = '';
    try {
        const res = await request(`/admin/import-items/${itemID}/publish`, { method: 'POST' });
        item.value = res.item;
        info.value = '已发布到题库；题目后续编辑仍会创建新版本。';
    }
    catch (err) {
        topError.value = err instanceof ApiError ? err.message : '发布失败';
    }
    finally {
        publishing.value = false;
        confirmPublish.value = false;
    }
}
onMounted(() => void load());
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
else if (__VLS_ctx.item) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "page-header" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({});
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "muted" },
        ...{ style: {} },
    });
    const __VLS_14 = {}.RouterLink;
    /** @type {[typeof __VLS_components.RouterLink, typeof __VLS_components.RouterLink, ]} */ ;
    // @ts-ignore
    const __VLS_15 = __VLS_asFunctionalComponent(__VLS_14, new __VLS_14({
        to: (`/admin/imports/${__VLS_ctx.item.jobId}`),
    }));
    const __VLS_16 = __VLS_15({
        to: (`/admin/imports/${__VLS_ctx.item.jobId}`),
    }, ...__VLS_functionalComponentArgsRest(__VLS_15));
    __VLS_17.slots.default;
    var __VLS_17;
    (__VLS_ctx.item.position);
    __VLS_asFunctionalElement(__VLS_intrinsicElements.h1, __VLS_intrinsicElements.h1)({
        ...{ style: {} },
    });
    /** @type {[typeof StatusBadge, ]} */ ;
    // @ts-ignore
    const __VLS_18 = __VLS_asFunctionalComponent(StatusBadge, new StatusBadge({
        value: (__VLS_ctx.item.reviewStatus),
    }));
    const __VLS_19 = __VLS_18({
        value: (__VLS_ctx.item.reviewStatus),
    }, ...__VLS_functionalComponentArgsRest(__VLS_18));
    if (__VLS_ctx.info) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "tag" },
            'data-tone': "success",
            role: "status",
        });
        (__VLS_ctx.info);
    }
    if (__VLS_ctx.topError) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "error-summary" },
            role: "alert",
        });
        (__VLS_ctx.topError);
    }
    if (__VLS_ctx.item.anomalies.length) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "error-summary" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.strong, __VLS_intrinsicElements.strong)({});
        __VLS_asFunctionalElement(__VLS_intrinsicElements.ul, __VLS_intrinsicElements.ul)({});
        for (const [anomaly] of __VLS_getVForSourceType((__VLS_ctx.item.anomalies))) {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.li, __VLS_intrinsicElements.li)({
                key: (anomaly),
            });
            (anomaly);
        }
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "import-review-grid" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.section, __VLS_intrinsicElements.section)({
        ...{ class: "card" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.h2, __VLS_intrinsicElements.h2)({
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.pre, __VLS_intrinsicElements.pre)({
        ...{ class: "import-source" },
        lang: "ja",
    });
    (__VLS_ctx.item.rawExcerpt);
    if (__VLS_ctx.aiSuggestedAnswer) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "import-note" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.strong, __VLS_intrinsicElements.strong)({});
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "mono" },
        });
        (__VLS_ctx.suggestionText);
        if (__VLS_ctx.aiSuggestedAnswer.explanation) {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({});
            (__VLS_ctx.aiSuggestedAnswer.explanation);
        }
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.form, __VLS_intrinsicElements.form)({
        ...{ onSubmit: (__VLS_ctx.save) },
        ...{ class: "card" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.h2, __VLS_intrinsicElements.h2)({
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "grid-2" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "field" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        for: "import-type",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.select, __VLS_intrinsicElements.select)({
        ...{ onChange: (__VLS_ctx.ensureOptionCount) },
        id: "import-type",
        value: (__VLS_ctx.form.type),
    });
    for (const [label, value] of __VLS_getVForSourceType((__VLS_ctx.questionTypeText))) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
            key: (value),
            value: (value),
        });
        (label);
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "field" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        for: "import-difficulty",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
        id: "import-difficulty",
        type: "number",
        min: "1",
        max: "5",
    });
    (__VLS_ctx.form.difficulty);
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "field" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        for: "import-stem",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.textarea)({
        id: "import-stem",
        value: (__VLS_ctx.form.stem),
        rows: "4",
        lang: "ja",
        'aria-describedby': (__VLS_ctx.fieldErr.stem ? 'import-stem-error' : undefined),
    });
    if (__VLS_ctx.fieldErr.stem) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            id: "import-stem-error",
            ...{ class: "error" },
        });
        (__VLS_ctx.fieldErr.stem);
    }
    if (__VLS_ctx.isChoice) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "field" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({});
        for (const [option, index] of __VLS_getVForSourceType((__VLS_ctx.form.options))) {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
                key: (option.id),
                ...{ class: "import-option-row" },
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
                value: (option.label),
                type: "text",
                'aria-label': "选项标号",
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
                value: (option.text),
                type: "text",
                lang: "ja",
                'aria-label': "选项内容",
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
                ...{ onClick: (...[$event]) => {
                        if (!!(__VLS_ctx.state === 'loading'))
                            return;
                        if (!!(__VLS_ctx.state === 'error'))
                            return;
                        if (!(__VLS_ctx.item))
                            return;
                        if (!(__VLS_ctx.isChoice))
                            return;
                        __VLS_ctx.removeOption(index);
                    } },
                type: "button",
                ...{ class: "ghost danger" },
                disabled: (__VLS_ctx.form.options.length <= 2),
            });
        }
        __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
            ...{ onClick: (__VLS_ctx.addOption) },
            type: "button",
        });
        if (__VLS_ctx.fieldErr.options) {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
                ...{ class: "error" },
            });
            (__VLS_ctx.fieldErr.options);
        }
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "field" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        for: "import-material-key",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
        id: "import-material-key",
        value: (__VLS_ctx.form.materialKey),
        type: "text",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        for: "import-material-title",
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
        id: "import-material-title",
        value: (__VLS_ctx.form.materialTitle),
        type: "text",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        for: "import-material-content",
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.textarea)({
        id: "import-material-content",
        value: (__VLS_ctx.form.materialContent),
        rows: "5",
        lang: "ja",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "grid-2" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "field" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        for: "import-level",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.select, __VLS_intrinsicElements.select)({
        id: "import-level",
        value: (__VLS_ctx.form.levelId),
    });
    for (const [level] of __VLS_getVForSourceType((__VLS_ctx.exams.flatMap((exam) => exam.levels)))) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
            key: (level.id),
            value: (level.id),
        });
        (level.name);
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
        for: "import-subject",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.select, __VLS_intrinsicElements.select)({
        id: "import-subject",
        value: (__VLS_ctx.form.subjectId),
    });
    for (const [subject] of __VLS_getVForSourceType((__VLS_ctx.exams.flatMap((exam) => exam.subjects)))) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
            key: (subject.id),
            value: (subject.id),
        });
        (subject.name);
    }
    if (__VLS_ctx.fieldErr.subjectId) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "error" },
        });
        (__VLS_ctx.fieldErr.subjectId);
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "field" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        for: "import-section",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.select, __VLS_intrinsicElements.select)({
        id: "import-section",
        value: (__VLS_ctx.form.sourceSectionId),
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
        value: "",
    });
    for (const [section] of __VLS_getVForSourceType((__VLS_ctx.sections))) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
            key: (section.id),
            value: (section.id),
        });
        (section.sourceName);
        (section.name);
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "field" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({});
    for (const [kp] of __VLS_getVForSourceType((__VLS_ctx.kps))) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
            key: (kp.id),
            ...{ class: "option-row" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
            type: "checkbox",
            value: (kp.id),
        });
        (__VLS_ctx.form.knowledgePointIds);
        (kp.name);
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.fieldset, __VLS_intrinsicElements.fieldset)({
        ...{ class: "import-answer-fieldset" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.legend, __VLS_intrinsicElements.legend)({});
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        ...{ class: "option-row" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
        type: "checkbox",
    });
    (__VLS_ctx.form.hasAnswer);
    if (__VLS_ctx.form.hasAnswer) {
        if (__VLS_ctx.isChoice) {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
                ...{ class: "field" },
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({});
            for (const [option] of __VLS_getVForSourceType((__VLS_ctx.form.options))) {
                __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
                    key: (option.id),
                    ...{ class: "option-row" },
                });
                __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
                    type: (__VLS_ctx.form.type === 'multiple_choice' ? 'checkbox' : 'radio'),
                    name: "import-correct-option",
                    value: (option.id),
                });
                (__VLS_ctx.form.correctOptionIds);
                (option.label);
                (option.text);
            }
        }
        else if (__VLS_ctx.form.type === 'fill_blank') {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
                ...{ class: "field" },
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
                for: "import-acceptable",
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.textarea)({
                id: "import-acceptable",
                value: (__VLS_ctx.form.acceptableText),
                rows: "3",
                lang: "ja",
            });
        }
        else {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
                ...{ class: "field" },
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
                for: "import-reference",
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.textarea)({
                id: "import-reference",
                value: (__VLS_ctx.form.referenceText),
                rows: "2",
                lang: "ja",
            });
        }
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "grid-2" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "field" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
            for: "import-authority",
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.select, __VLS_intrinsicElements.select)({
            id: "import-authority",
            value: (__VLS_ctx.form.authority),
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
            value: "official",
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
            value: "human_verified",
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "field" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
            for: "import-explanation",
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.textarea)({
            id: "import-explanation",
            value: (__VLS_ctx.form.explanation),
            rows: "2",
        });
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "muted" },
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "import-actions" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
        ...{ class: "primary" },
        type: "submit",
        disabled: (__VLS_ctx.saving || __VLS_ctx.item.reviewStatus === 'published'),
    });
    (__VLS_ctx.saving ? '保存中…' : '保存草稿');
    if (__VLS_ctx.item.reviewStatus === 'pending') {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
            ...{ onClick: (__VLS_ctx.approve) },
            type: "button",
            disabled: (__VLS_ctx.saving || __VLS_ctx.approving),
        });
        (__VLS_ctx.approving ? '审核中…' : '确认审核');
    }
    if (__VLS_ctx.item.reviewStatus === 'approved') {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
            ...{ onClick: (__VLS_ctx.askPublish) },
            type: "button",
            ...{ class: "primary" },
            disabled: (__VLS_ctx.publishing),
        });
    }
    const __VLS_21 = {}.RouterLink;
    /** @type {[typeof __VLS_components.RouterLink, typeof __VLS_components.RouterLink, ]} */ ;
    // @ts-ignore
    const __VLS_22 = __VLS_asFunctionalComponent(__VLS_21, new __VLS_21({
        ...{ class: "tag" },
        to: (`/admin/imports/${__VLS_ctx.item.jobId}`),
    }));
    const __VLS_23 = __VLS_22({
        ...{ class: "tag" },
        to: (`/admin/imports/${__VLS_ctx.item.jobId}`),
    }, ...__VLS_functionalComponentArgsRest(__VLS_22));
    __VLS_24.slots.default;
    var __VLS_24;
    /** @type {[typeof ConfirmDialog, typeof ConfirmDialog, ]} */ ;
    // @ts-ignore
    const __VLS_25 = __VLS_asFunctionalComponent(ConfirmDialog, new ConfirmDialog({
        ...{ 'onConfirm': {} },
        ...{ 'onCancel': {} },
        open: (__VLS_ctx.confirmPublish),
        title: "确认发布题目？",
        confirmLabel: "确认发布",
        cancelLabel: "再检查一下",
    }));
    const __VLS_26 = __VLS_25({
        ...{ 'onConfirm': {} },
        ...{ 'onCancel': {} },
        open: (__VLS_ctx.confirmPublish),
        title: "确认发布题目？",
        confirmLabel: "确认发布",
        cancelLabel: "再检查一下",
    }, ...__VLS_functionalComponentArgsRest(__VLS_25));
    let __VLS_28;
    let __VLS_29;
    let __VLS_30;
    const __VLS_31 = {
        onConfirm: (__VLS_ctx.publish)
    };
    const __VLS_32 = {
        onCancel: (...[$event]) => {
            if (!!(__VLS_ctx.state === 'loading'))
                return;
            if (!!(__VLS_ctx.state === 'error'))
                return;
            if (!(__VLS_ctx.item))
                return;
            __VLS_ctx.confirmPublish = false;
        }
    };
    __VLS_27.slots.default;
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({});
    var __VLS_27;
}
var __VLS_2;
/** @type {__VLS_StyleScopedClasses['page-header']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['tag']} */ ;
/** @type {__VLS_StyleScopedClasses['error-summary']} */ ;
/** @type {__VLS_StyleScopedClasses['error-summary']} */ ;
/** @type {__VLS_StyleScopedClasses['import-review-grid']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['import-source']} */ ;
/** @type {__VLS_StyleScopedClasses['import-note']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['grid-2']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['error']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['import-option-row']} */ ;
/** @type {__VLS_StyleScopedClasses['ghost']} */ ;
/** @type {__VLS_StyleScopedClasses['danger']} */ ;
/** @type {__VLS_StyleScopedClasses['error']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['grid-2']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['error']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['error']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['option-row']} */ ;
/** @type {__VLS_StyleScopedClasses['import-answer-fieldset']} */ ;
/** @type {__VLS_StyleScopedClasses['option-row']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['option-row']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['grid-2']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['import-actions']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
/** @type {__VLS_StyleScopedClasses['tag']} */ ;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            AppShell: AppShell,
            AppStatus: AppStatus,
            ConfirmDialog: ConfirmDialog,
            StatusBadge: StatusBadge,
            questionTypeText: questionTypeText,
            item: item,
            exams: exams,
            kps: kps,
            state: state,
            errorMessage: errorMessage,
            requestID: requestID,
            topError: topError,
            info: info,
            saving: saving,
            approving: approving,
            publishing: publishing,
            confirmPublish: confirmPublish,
            fieldErr: fieldErr,
            form: form,
            aiSuggestedAnswer: aiSuggestedAnswer,
            isChoice: isChoice,
            sections: sections,
            suggestionText: suggestionText,
            load: load,
            ensureOptionCount: ensureOptionCount,
            addOption: addOption,
            removeOption: removeOption,
            save: save,
            approve: approve,
            askPublish: askPublish,
            publish: publish,
        };
    },
});
export default (await import('vue')).defineComponent({
    setup() {
        return {};
    },
});
; /* PartiallyEnd: #4569/main.vue */
