import { onMounted, reactive, ref } from 'vue';
import { useRouter } from 'vue-router';
import { request, ApiError, fieldErrors } from '@/api/client';
import AppShell from '@/components/AppShell.vue';
import { clearSession, sessionUser } from '@/app/session';
const router = useRouter();
const exams = ref([]);
const form = reactive({ defaultLevelId: '' });
const saving = ref(false);
const saved = ref(false);
const errorMessage = ref('');
onMounted(async () => {
    const me = sessionUser();
    if (me?.defaultLevelId)
        form.defaultLevelId = me.defaultLevelId;
    try {
        const res = await request('/catalog');
        exams.value = res.exams;
    }
    catch {
        // 忽略：级别下拉为空也可保存其他设置
    }
});
async function save() {
    saving.value = true;
    saved.value = false;
    errorMessage.value = '';
    try {
        await request('/me', {
            method: 'PATCH',
            body: { defaultLevelId: form.defaultLevelId || null },
        });
        saved.value = true;
    }
    catch (err) {
        errorMessage.value = Object.values(fieldErrors(err))[0] ?? (err instanceof ApiError ? err.message : '保存失败');
    }
    finally {
        saving.value = false;
    }
}
async function logout() {
    try {
        await request('/auth/logout', { method: 'POST' });
    }
    finally {
        clearSession();
        await router.replace('/login');
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
__VLS_asFunctionalElement(__VLS_intrinsicElements.h1, __VLS_intrinsicElements.h1)({
    ...{ style: {} },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.form, __VLS_intrinsicElements.form)({
    ...{ onSubmit: (__VLS_ctx.save) },
    ...{ class: "card" },
    ...{ style: {} },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "field" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
    for: "level",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.select, __VLS_intrinsicElements.select)({
    id: "level",
    value: (__VLS_ctx.form.defaultLevelId),
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
    value: "",
});
for (const [l] of __VLS_getVForSourceType((__VLS_ctx.exams.flatMap((e) => e.levels)))) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.option, __VLS_intrinsicElements.option)({
        key: (l.id),
        value: (l.id),
    });
    (l.name);
}
__VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
    ...{ class: "muted" },
    ...{ style: {} },
});
if (__VLS_ctx.saved) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "tag" },
        'data-tone': "success",
        role: "status",
        ...{ style: {} },
    });
}
if (__VLS_ctx.errorMessage) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        ...{ class: "error-summary" },
        role: "alert",
    });
    (__VLS_ctx.errorMessage);
}
__VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
    ...{ class: "primary" },
    type: "submit",
    disabled: (__VLS_ctx.saving),
});
(__VLS_ctx.saving ? '保存中…' : '保存');
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "card" },
    ...{ style: {} },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.h2, __VLS_intrinsicElements.h2)({
    ...{ style: {} },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
    ...{ class: "muted" },
});
(__VLS_ctx.sessionUser()?.email);
__VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
    ...{ onClick: (__VLS_ctx.logout) },
    ...{ class: "danger" },
});
var __VLS_2;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['tag']} */ ;
/** @type {__VLS_StyleScopedClasses['error-summary']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['danger']} */ ;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            AppShell: AppShell,
            sessionUser: sessionUser,
            exams: exams,
            form: form,
            saving: saving,
            saved: saved,
            errorMessage: errorMessage,
            save: save,
            logout: logout,
        };
    },
});
export default (await import('vue')).defineComponent({
    setup() {
        return {};
    },
});
; /* PartiallyEnd: #4569/main.vue */
