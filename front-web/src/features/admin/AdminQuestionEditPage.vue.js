import { computed, onMounted, reactive, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { request, ApiError, fieldErrors } from '@/api/client';
import AppShell from '@/components/AppShell.vue';
import AppStatus from '@/components/AppStatus.vue';
import ConfirmDialog from '@/components/ConfirmDialog.vue';
import { questionTypeText } from '@/app/format';
const route = useRoute();
const router = useRouter();
const questionID = computed(() => route.params.questionId);
const exams = ref([]);
const sources = ref([]);
const kps = ref([]);
const pageState = ref('loading');
const loadError = ref('');
const requestID = ref('');
const form = reactive({
    type: 'single_choice',
    stem: '',
    options: [],
    useMaterial: false,
    materialTitle: '',
    materialContent: '',
    levelId: '',
    subjectId: '',
    sourceSectionId: '',
    difficulty: 3,
    knowledgePointIds: [],
    hasKey: true,
    correctOptionIds: [],
    acceptableText: '',
    referenceText: '',
    authority: 'official',
    explanation: '',
});
const status = ref('draft');
const hasPublishedVersion = ref(false);
const saving = ref(false);
const publishing = ref(false);
const fieldErr = reactive({});
const topError = ref('');
const info = ref('');
const confirmPublish = ref(false);
const isChoice = computed(() => form.type === 'single_choice' || form.type === 'multiple_choice');
const sections = computed(() => {
    const sourceID = (sources.value[0]?.id ?? '');
    return sources.value.find((s) => s.id === sourceID)?.sections ?? [];
});
onMounted(async () => {
    try {
        const [catalog, sourceRes] = await Promise.all([
            request('/catalog'),
            request('/admin/sources'),
        ]);
        exams.value = catalog.exams;
        sources.value = sourceRes.sources;
        if (questionID.value) {
            await loadQuestion();
        }
        else {
            form.levelId = exams.value[0]?.levels[0]?.id ?? '';
            form.subjectId = exams.value[0]?.subjects[0]?.id ?? '';
            ensureOptionCount();
        }
        await loadKPs();
        pageState.value = 'ready';
    }
    catch (err) {
        if (err instanceof ApiError && err.status === 404) {
            pageState.value = 'notfound';
            return;
        }
        loadError.value = err instanceof ApiError ? err.message : '加载失败';
        requestID.value = err instanceof ApiError ? err.requestId ?? '' : '';
        pageState.value = 'error';
    }
});
async function loadQuestion() {
    const res = await request(`/admin/questions/${questionID.value}`);
    const q = res.question;
    status.value = q.status;
    hasPublishedVersion.value = q.publishedVersionId !== null;
    const v = q.currentVersion;
    if (!v)
        return;
    form.type = v.type;
    form.stem = v.stem;
    form.options = v.options ? JSON.parse(JSON.stringify(v.options)) : [];
    if (v.materialContent) {
        form.useMaterial = true;
        form.materialTitle = v.materialTitle ?? '';
        form.materialContent = v.materialContent;
    }
    form.levelId = v.levelId;
    form.subjectId = v.subjectId;
    form.sourceSectionId = v.sourceSectionId ?? '';
    form.difficulty = v.difficulty;
    form.knowledgePointIds = [...v.knowledgePointIds];
    const key = v.answerKey;
    if (key) {
        form.hasKey = true;
        form.authority = key.authority;
        form.explanation = key.explanation;
        if (Array.isArray(key.value.optionIds)) {
            form.correctOptionIds = [...key.value.optionIds];
        }
        if (Array.isArray(key.value.acceptable)) {
            form.acceptableText = key.value.acceptable.join('\n');
        }
        if (typeof key.value.reference === 'string') {
            form.referenceText = key.value.reference;
        }
    }
    else {
        form.hasKey = false;
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
watch(() => form.type, () => {
    if (isChoice.value)
        ensureOptionCount();
});
function ensureOptionCount() {
    while (form.options.length < 4) {
        const letter = String.fromCharCode(65 + form.options.length);
        form.options.push({ id: letter.toLowerCase(), label: letter, text: '' });
    }
}
function addOption() {
    const n = form.options.length;
    const letter = String.fromCharCode(65 + n);
    form.options.push({ id: `${letter.toLowerCase()}${n}`, label: letter, text: '' });
}
function removeOption(index) {
    if (form.options.length <= 2)
        return;
    form.options.splice(index, 1);
}
function buildRequestBody() {
    const body = {
        type: form.type,
        stem: form.stem,
        options: isChoice.value ? form.options : [],
        levelId: form.levelId,
        subjectId: form.subjectId,
        difficulty: form.difficulty,
        knowledgePointIds: form.knowledgePointIds,
        sourceSectionId: form.sourceSectionId || null,
    };
    if (form.useMaterial && form.materialContent) {
        body.materialTitle = form.materialTitle;
        body.materialContent = form.materialContent;
    }
    if (form.hasKey) {
        let value = null;
        if (isChoice.value && form.correctOptionIds.length > 0) {
            value = { optionIds: [...form.correctOptionIds].sort() };
        }
        else if (form.type === 'fill_blank') {
            value = { acceptable: form.acceptableText.split('\n').map((s) => s.trim()).filter(Boolean) };
        }
        else if (form.type === 'short_answer' && form.referenceText.trim()) {
            value = { reference: form.referenceText.trim() };
        }
        if (value) {
            body.answer = { value, authority: form.authority, explanation: form.explanation };
        }
    }
    return body;
}
async function save() {
    saving.value = true;
    topError.value = '';
    info.value = '';
    for (const k of Object.keys(fieldErr))
        delete fieldErr[k];
    try {
        if (questionID.value) {
            await request(`/admin/questions/${questionID.value}`, {
                method: 'PATCH',
                body: buildRequestBody(),
            });
            await loadQuestion();
        }
        else {
            const res = await request('/admin/questions', {
                method: 'POST',
                body: buildRequestBody(),
            });
            await router.replace(`/admin/questions/${res.question.id}`);
        }
        info.value = '草稿已保存。发布是独立操作，需要再次确认。';
    }
    catch (err) {
        const fields = fieldErrors(err);
        for (const [k, v] of Object.entries(fields))
            fieldErr[k] = v;
        topError.value = Object.keys(fields).length ? '请修正表单错误' : err instanceof ApiError ? err.message : '保存失败，请重试';
    }
    finally {
        saving.value = false;
    }
}
async function submitReview() {
    await save();
    try {
        const res = await request(`/admin/questions/${questionID.value}/submit-review`, { method: 'POST' });
        status.value = res.question.status;
        info.value = '已提交审核。';
    }
    catch (err) {
        topError.value = err instanceof ApiError ? err.message : '操作失败';
    }
}
async function publish() {
    if (!form.hasKey) {
        topError.value = '客观题没有标准答案也可以发布，但请先确认：本题将走 AI 判定。再次点击“确认发布”继续。';
        confirmPublish.value = true;
        return;
    }
    confirmPublish.value = true;
}
async function doPublish() {
    publishing.value = true;
    topError.value = '';
    try {
        const res = await request(`/admin/questions/${questionID.value}/publish`, { method: 'POST' });
        status.value = res.question.status;
        hasPublishedVersion.value = res.question.publishedVersionId !== null;
        info.value = '已发布新版本。历史练习仍引用旧版本，不受影响。';
    }
    catch (err) {
        topError.value = err instanceof ApiError ? err.message : '发布失败';
    }
    finally {
        publishing.value = false;
        confirmPublish.value = false;
    }
}
async function retire() {
    if (!confirm('确认下架该题目？下架后不再进入新练习，历史记录保留。'))
        return;
    try {
        const res = await request(`/admin/questions/${questionID.value}/retire`, { method: 'POST' });
        status.value = res.question.status;
        info.value = '已下架。';
    }
    catch (err) {
        topError.value = err instanceof ApiError ? err.message : '操作失败';
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
if (__VLS_ctx.pageState === 'loading') {
    /** @type {[typeof AppStatus, ]} */ ;
    // @ts-ignore
    const __VLS_4 = __VLS_asFunctionalComponent(AppStatus, new AppStatus({
        state: "loading",
    }));
    const __VLS_5 = __VLS_4({
        state: "loading",
    }, ...__VLS_functionalComponentArgsRest(__VLS_4));
}
else if (__VLS_ctx.pageState === 'error') {
    /** @type {[typeof AppStatus, ]} */ ;
    // @ts-ignore
    const __VLS_7 = __VLS_asFunctionalComponent(AppStatus, new AppStatus({
        ...{ 'onAction': {} },
        state: "error",
        message: (__VLS_ctx.loadError),
        requestId: (__VLS_ctx.requestID),
    }));
    const __VLS_8 = __VLS_7({
        ...{ 'onAction': {} },
        state: "error",
        message: (__VLS_ctx.loadError),
        requestId: (__VLS_ctx.requestID),
    }, ...__VLS_functionalComponentArgsRest(__VLS_7));
    let __VLS_10;
    let __VLS_11;
    let __VLS_12;
    const __VLS_13 = {
        onAction: (...[$event]) => {
            if (!!(__VLS_ctx.pageState === 'loading'))
                return;
            if (!(__VLS_ctx.pageState === 'error'))
                return;
            __VLS_ctx.pageState = 'ready';
        }
    };
    var __VLS_9;
}
else if (__VLS_ctx.pageState === 'notfound') {
    /** @type {[typeof AppStatus, ]} */ ;
    // @ts-ignore
    const __VLS_14 = __VLS_asFunctionalComponent(AppStatus, new AppStatus({
        ...{ 'onAction': {} },
        state: "empty",
        message: "题目不存在。",
        actionLabel: "返回题目列表",
    }));
    const __VLS_15 = __VLS_14({
        ...{ 'onAction': {} },
        state: "empty",
        message: "题目不存在。",
        actionLabel: "返回题目列表",
    }, ...__VLS_functionalComponentArgsRest(__VLS_14));
    let __VLS_17;
    let __VLS_18;
    let __VLS_19;
    const __VLS_20 = {
        onAction: (...[$event]) => {
            if (!!(__VLS_ctx.pageState === 'loading'))
                return;
            if (!!(__VLS_ctx.pageState === 'error'))
                return;
            if (!(__VLS_ctx.pageState === 'notfound'))
                return;
            __VLS_ctx.router.push('/admin/questions');
        }
    };
    var __VLS_16;
}
else {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "page-header" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.h1, __VLS_intrinsicElements.h1)({
        ...{ style: {} },
    });
    (__VLS_ctx.questionID ? '编辑题目' : '新建题目');
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
        ...{ class: "tag" },
    });
    (__VLS_ctx.status === 'draft' ? '草稿' : __VLS_ctx.status === 'in_review' ? '待审核' : __VLS_ctx.status === 'published' ? '已发布' : '已下架');
    if (__VLS_ctx.hasPublishedVersion) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
            ...{ class: "tag" },
            'data-tone': "success",
        });
    }
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
    __VLS_asFunctionalElement(__VLS_intrinsicElements.form, __VLS_intrinsicElements.form)({
        ...{ onSubmit: (__VLS_ctx.save) },
        ...{ class: "card" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "grid-2" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "field" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        for: "q-type",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.select, __VLS_intrinsicElements.select)({
        id: "q-type",
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
        for: "q-difficulty",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
        id: "q-difficulty",
        type: "number",
        min: "1",
        max: "5",
    });
    (__VLS_ctx.form.difficulty);
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "field" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        for: "q-stem",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.textarea)({
        id: "q-stem",
        value: (__VLS_ctx.form.stem),
        rows: "3",
        lang: "ja",
        'aria-describedby': (__VLS_ctx.fieldErr.stem ? 'err-stem' : undefined),
    });
    if (__VLS_ctx.fieldErr.stem) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            id: "err-stem",
            ...{ class: "error" },
        });
        (__VLS_ctx.fieldErr.stem);
    }
    if (__VLS_ctx.isChoice) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "field" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({});
        for (const [opt, i] of __VLS_getVForSourceType((__VLS_ctx.form.options))) {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
                key: (i),
                ...{ style: {} },
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
                value: (opt.label),
                type: "text",
                ...{ style: {} },
                'aria-label': "选项标号",
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
                value: (opt.text),
                type: "text",
                lang: "ja",
                'aria-label': "选项内容",
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
                ...{ onClick: (...[$event]) => {
                        if (!!(__VLS_ctx.pageState === 'loading'))
                            return;
                        if (!!(__VLS_ctx.pageState === 'error'))
                            return;
                        if (!!(__VLS_ctx.pageState === 'notfound'))
                            return;
                        if (!(__VLS_ctx.isChoice))
                            return;
                        __VLS_ctx.removeOption(i);
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
    __VLS_asFunctionalElement(__VLS_intrinsicElements.fieldset, __VLS_intrinsicElements.fieldset)({
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.legend, __VLS_intrinsicElements.legend)({
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
        type: "checkbox",
    });
    (__VLS_ctx.form.useMaterial);
    __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({});
    if (__VLS_ctx.form.useMaterial) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "field" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
            for: "m-title",
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
            id: "m-title",
            value: (__VLS_ctx.form.materialTitle),
            type: "text",
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "field" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
            for: "m-content",
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.textarea)({
            id: "m-content",
            value: (__VLS_ctx.form.materialContent),
            rows: "5",
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
        for: "q-level",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.select, __VLS_intrinsicElements.select)({
        id: "q-level",
        value: (__VLS_ctx.form.levelId),
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
        for: "q-subject",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.select, __VLS_intrinsicElements.select)({
        id: "q-subject",
        value: (__VLS_ctx.form.subjectId),
    });
    for (const [s] of __VLS_getVForSourceType((__VLS_ctx.exams.flatMap((e) => e.subjects)))) {
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
        ...{ class: "field" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        for: "q-section",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.select, __VLS_intrinsicElements.select)({
        id: "q-section",
        value: (__VLS_ctx.form.sourceSectionId),
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
        value: "",
    });
    for (const [sec] of __VLS_getVForSourceType((__VLS_ctx.sections))) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
            key: (sec.id),
            value: (sec.id),
        });
        (sec.name);
    }
    if (__VLS_ctx.fieldErr.sourceSectionId) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "error" },
        });
        (__VLS_ctx.fieldErr.sourceSectionId);
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "field" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({});
    for (const [k] of __VLS_getVForSourceType((__VLS_ctx.kps))) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
            key: (k.id),
            ...{ class: "option-row" },
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
            type: "checkbox",
            value: (k.id),
        });
        (__VLS_ctx.form.knowledgePointIds);
        __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({});
        (k.name);
        __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
            ...{ class: "muted" },
        });
        (k.status === 'published' ? '已发布' : '草稿');
    }
    if (__VLS_ctx.fieldErr.knowledgePointIds) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "error" },
        });
        (__VLS_ctx.fieldErr.knowledgePointIds);
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.fieldset, __VLS_intrinsicElements.fieldset)({
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.legend, __VLS_intrinsicElements.legend)({
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
        type: "checkbox",
    });
    (__VLS_ctx.form.hasKey);
    __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({});
    if (__VLS_ctx.form.hasKey) {
        if (__VLS_ctx.isChoice) {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
                ...{ class: "field" },
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({});
            (__VLS_ctx.form.type === 'multiple_choice' ? '可多选' : '单选');
            for (const [opt] of __VLS_getVForSourceType((__VLS_ctx.form.options))) {
                __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
                    key: (opt.id),
                    ...{ class: "option-row" },
                    ...{ style: {} },
                });
                __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
                    type: (__VLS_ctx.form.type === 'multiple_choice' ? 'checkbox' : 'radio'),
                    name: "correct-option",
                    value: (opt.id),
                });
                (__VLS_ctx.form.correctOptionIds);
                __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
                    ...{ class: "mono" },
                });
                (opt.label);
                (opt.text);
            }
        }
        else if (__VLS_ctx.form.type === 'fill_blank') {
            __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
                ...{ class: "field" },
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
                for: "q-acceptable",
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.textarea)({
                id: "q-acceptable",
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
                for: "q-reference",
            });
            __VLS_asFunctionalElement(__VLS_intrinsicElements.textarea)({
                id: "q-reference",
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
            for: "q-authority",
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.select, __VLS_intrinsicElements.select)({
            id: "q-authority",
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
            for: "q-explanation",
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.textarea)({
            id: "q-explanation",
            value: (__VLS_ctx.form.explanation),
            rows: "2",
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "muted" },
            ...{ style: {} },
        });
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
        ...{ class: "primary" },
        type: "submit",
        disabled: (__VLS_ctx.saving),
    });
    (__VLS_ctx.saving ? '保存中…' : '保存草稿');
    if (__VLS_ctx.questionID && __VLS_ctx.status === 'draft') {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
            ...{ onClick: (__VLS_ctx.submitReview) },
            type: "button",
            disabled: (__VLS_ctx.saving),
        });
    }
    if (__VLS_ctx.questionID && __VLS_ctx.status !== 'retired') {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
            ...{ onClick: (__VLS_ctx.publish) },
            type: "button",
            disabled: (__VLS_ctx.saving || __VLS_ctx.publishing),
        });
    }
    if (__VLS_ctx.questionID && __VLS_ctx.status !== 'retired') {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
            ...{ onClick: (__VLS_ctx.retire) },
            type: "button",
            ...{ class: "danger" },
        });
    }
    /** @type {[typeof ConfirmDialog, typeof ConfirmDialog, ]} */ ;
    // @ts-ignore
    const __VLS_21 = __VLS_asFunctionalComponent(ConfirmDialog, new ConfirmDialog({
        ...{ 'onConfirm': {} },
        ...{ 'onCancel': {} },
        open: (__VLS_ctx.confirmPublish),
        title: "确认发布？",
        confirmLabel: "确认发布",
        cancelLabel: "再检查一下",
    }));
    const __VLS_22 = __VLS_21({
        ...{ 'onConfirm': {} },
        ...{ 'onCancel': {} },
        open: (__VLS_ctx.confirmPublish),
        title: "确认发布？",
        confirmLabel: "确认发布",
        cancelLabel: "再检查一下",
    }, ...__VLS_functionalComponentArgsRest(__VLS_21));
    let __VLS_24;
    let __VLS_25;
    let __VLS_26;
    const __VLS_27 = {
        onConfirm: (__VLS_ctx.doPublish)
    };
    const __VLS_28 = {
        onCancel: (...[$event]) => {
            if (!!(__VLS_ctx.pageState === 'loading'))
                return;
            if (!!(__VLS_ctx.pageState === 'error'))
                return;
            if (!!(__VLS_ctx.pageState === 'notfound'))
                return;
            __VLS_ctx.confirmPublish = false;
        }
    };
    __VLS_23.slots.default;
    if (!__VLS_ctx.form.hasKey) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({});
    }
    else {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({});
    }
    var __VLS_23;
}
var __VLS_2;
/** @type {__VLS_StyleScopedClasses['page-header']} */ ;
/** @type {__VLS_StyleScopedClasses['tag']} */ ;
/** @type {__VLS_StyleScopedClasses['tag']} */ ;
/** @type {__VLS_StyleScopedClasses['tag']} */ ;
/** @type {__VLS_StyleScopedClasses['error-summary']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['grid-2']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['error']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['ghost']} */ ;
/** @type {__VLS_StyleScopedClasses['danger']} */ ;
/** @type {__VLS_StyleScopedClasses['error']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['grid-2']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['error']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['error']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['error']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['option-row']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['error']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['option-row']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['grid-2']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
/** @type {__VLS_StyleScopedClasses['danger']} */ ;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            AppShell: AppShell,
            AppStatus: AppStatus,
            ConfirmDialog: ConfirmDialog,
            questionTypeText: questionTypeText,
            router: router,
            questionID: questionID,
            exams: exams,
            kps: kps,
            pageState: pageState,
            loadError: loadError,
            requestID: requestID,
            form: form,
            status: status,
            hasPublishedVersion: hasPublishedVersion,
            saving: saving,
            publishing: publishing,
            fieldErr: fieldErr,
            topError: topError,
            info: info,
            confirmPublish: confirmPublish,
            isChoice: isChoice,
            sections: sections,
            addOption: addOption,
            removeOption: removeOption,
            save: save,
            submitReview: submitReview,
            publish: publish,
            doPublish: doPublish,
            retire: retire,
        };
    },
});
export default (await import('vue')).defineComponent({
    setup() {
        return {};
    },
});
; /* PartiallyEnd: #4569/main.vue */
