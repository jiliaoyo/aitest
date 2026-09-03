import { onMounted, ref } from 'vue';
import { request, upload, ApiError } from '@/api/client';
import AppShell from '@/components/AppShell.vue';
import AppStatus from '@/components/AppStatus.vue';
import StatusBadge from '@/components/StatusBadge.vue';
import { formatDateTime } from '@/app/format';
const jobs = ref([]);
const state = ref('loading');
const file = ref(null);
const uploading = ref(false);
const message = ref('');
const errorMessage = ref('');
const requestID = ref('');
async function load() {
    state.value = 'loading';
    try {
        const res = await request('/admin/import-jobs?limit=50');
        jobs.value = res.jobs;
        state.value = 'ready';
    }
    catch (err) {
        errorMessage.value = err instanceof ApiError ? err.message : '加载失败';
        requestID.value = err instanceof ApiError ? err.requestId ?? '' : '';
        state.value = 'error';
    }
}
function chooseFile(event) {
    file.value = event.target.files?.[0] ?? null;
    message.value = '';
}
async function createJob() {
    if (!file.value) {
        message.value = '请先选择文件。';
        return;
    }
    uploading.value = true;
    message.value = '';
    try {
        const res = await upload('/admin/import-jobs', file.value);
        file.value = null;
        const input = document.getElementById('import-file');
        if (input)
            input.value = '';
        message.value = '已创建导入任务，正在处理。';
        await load();
        void res;
    }
    catch (err) {
        message.value = err instanceof ApiError ? err.message : '上传失败，请重试';
    }
    finally {
        uploading.value = false;
    }
}
onMounted(load);
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
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "page-header" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({});
__VLS_asFunctionalElement(__VLS_intrinsicElements.h1, __VLS_intrinsicElements.h1)({
    ...{ style: {} },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
    ...{ class: "muted" },
    ...{ style: {} },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.form, __VLS_intrinsicElements.form)({
    ...{ onSubmit: (__VLS_ctx.createJob) },
    ...{ class: "card" },
    ...{ style: {} },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "field" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
    for: "import-file",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
    ...{ onChange: (__VLS_ctx.chooseFile) },
    id: "import-file",
    type: "file",
    accept: ".txt,.md,.csv,.docx,.pdf",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
    ...{ class: "muted" },
    ...{ style: {} },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
    ...{ class: "primary" },
    type: "submit",
    disabled: (__VLS_ctx.uploading),
});
(__VLS_ctx.uploading ? '上传中…' : '上传并开始整理');
if (__VLS_ctx.message) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "tag" },
        'data-tone': "success",
        role: "status",
        ...{ style: {} },
    });
    (__VLS_ctx.message);
}
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
else if (__VLS_ctx.jobs.length === 0) {
    /** @type {[typeof AppStatus, ]} */ ;
    // @ts-ignore
    const __VLS_14 = __VLS_asFunctionalComponent(AppStatus, new AppStatus({
        state: "empty",
        message: "还没有导入任务。",
    }));
    const __VLS_15 = __VLS_14({
        state: "empty",
        message: "还没有导入任务。",
    }, ...__VLS_functionalComponentArgsRest(__VLS_14));
}
else {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "card" },
        ...{ style: {} },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.table, __VLS_intrinsicElements.table)({
        ...{ class: "data" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.thead, __VLS_intrinsicElements.thead)({});
    __VLS_asFunctionalElement(__VLS_intrinsicElements.tr, __VLS_intrinsicElements.tr)({});
    __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({});
    __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({});
    __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({
        ...{ class: "num" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.th, __VLS_intrinsicElements.th)({});
    __VLS_asFunctionalElement(__VLS_intrinsicElements.tbody, __VLS_intrinsicElements.tbody)({});
    for (const [job] of __VLS_getVForSourceType((__VLS_ctx.jobs))) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.tr, __VLS_intrinsicElements.tr)({
            key: (job.id),
        });
        __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({});
        const __VLS_17 = {}.RouterLink;
        /** @type {[typeof __VLS_components.RouterLink, typeof __VLS_components.RouterLink, ]} */ ;
        // @ts-ignore
        const __VLS_18 = __VLS_asFunctionalComponent(__VLS_17, new __VLS_17({
            to: (`/admin/imports/${job.id}`),
        }));
        const __VLS_19 = __VLS_18({
            to: (`/admin/imports/${job.id}`),
        }, ...__VLS_functionalComponentArgsRest(__VLS_18));
        __VLS_20.slots.default;
        (job.fileName);
        var __VLS_20;
        __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({});
        /** @type {[typeof StatusBadge, ]} */ ;
        // @ts-ignore
        const __VLS_21 = __VLS_asFunctionalComponent(StatusBadge, new StatusBadge({
            value: (job.status),
        }));
        const __VLS_22 = __VLS_21({
            value: (job.status),
        }, ...__VLS_functionalComponentArgsRest(__VLS_21));
        __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({
            ...{ class: "num" },
        });
        (job.itemCount);
        __VLS_asFunctionalElement(__VLS_intrinsicElements.td, __VLS_intrinsicElements.td)({
            ...{ class: "mono" },
        });
        (__VLS_ctx.formatDateTime(job.updatedAt));
    }
}
var __VLS_2;
/** @type {__VLS_StyleScopedClasses['page-header']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
/** @type {__VLS_StyleScopedClasses['tag']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['data']} */ ;
/** @type {__VLS_StyleScopedClasses['num']} */ ;
/** @type {__VLS_StyleScopedClasses['num']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            AppShell: AppShell,
            AppStatus: AppStatus,
            StatusBadge: StatusBadge,
            formatDateTime: formatDateTime,
            jobs: jobs,
            state: state,
            uploading: uploading,
            message: message,
            errorMessage: errorMessage,
            requestID: requestID,
            load: load,
            chooseFile: chooseFile,
            createJob: createJob,
        };
    },
});
export default (await import('vue')).defineComponent({
    setup() {
        return {};
    },
});
; /* PartiallyEnd: #4569/main.vue */
