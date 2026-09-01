import { computed, reactive, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { request, fieldErrors } from '@/api/client';
const route = useRoute();
const router = useRouter();
const form = reactive({ password: '', confirm: '' });
const fieldErr = reactive({});
const topError = ref('');
const submitting = ref(false);
const token = computed(() => route.query.token ?? '');
async function submit() {
    for (const k of Object.keys(fieldErr))
        delete fieldErr[k];
    if (form.password !== form.confirm) {
        fieldErr.confirm = '两次输入的密码不一致';
        return;
    }
    submitting.value = true;
    topError.value = '';
    try {
        await request('/auth/password-reset/confirm', {
            method: 'POST',
            body: { token: token.value, password: form.password },
        });
        router.replace('/login');
    }
    catch (err) {
        const fields = fieldErrors(err);
        for (const [k, v] of Object.entries(fields))
            fieldErr[k] = v;
        if (err instanceof Error && !Object.keys(fields).length) {
            topError.value = err.message;
        }
    }
    finally {
        submitting.value = false;
    }
}
debugger; /* PartiallyEnd: #3632/scriptSetup.vue */
const __VLS_ctx = {};
let __VLS_components;
let __VLS_directives;
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "layout-shell" },
    ...{ style: {} },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.h1, __VLS_intrinsicElements.h1)({});
if (__VLS_ctx.token) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.form, __VLS_intrinsicElements.form)({
        ...{ onSubmit: (__VLS_ctx.submit) },
        ...{ class: "card" },
        novalidate: true,
    });
    if (__VLS_ctx.topError) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
            ...{ class: "error-summary" },
            role: "alert",
        });
        (__VLS_ctx.topError);
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "field" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        for: "password",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
        id: "password",
        type: "password",
        autocomplete: "new-password",
    });
    (__VLS_ctx.form.password);
    if (__VLS_ctx.fieldErr.password) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "error" },
        });
        (__VLS_ctx.fieldErr.password);
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "field" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.label, __VLS_intrinsicElements.label)({
        for: "confirm",
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
        id: "confirm",
        type: "password",
        autocomplete: "new-password",
    });
    (__VLS_ctx.form.confirm);
    if (__VLS_ctx.fieldErr.confirm) {
        __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
            ...{ class: "error" },
        });
        (__VLS_ctx.fieldErr.confirm);
    }
    __VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
        ...{ class: "primary" },
        type: "submit",
        disabled: (__VLS_ctx.submitting),
        ...{ style: {} },
    });
    (__VLS_ctx.submitting ? '提交中' : '重置密码');
}
else {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
        ...{ class: "card" },
    });
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({});
    const __VLS_0 = {}.RouterLink;
    /** @type {[typeof __VLS_components.RouterLink, typeof __VLS_components.RouterLink, ]} */ ;
    // @ts-ignore
    const __VLS_1 = __VLS_asFunctionalComponent(__VLS_0, new __VLS_0({
        ...{ class: "tag" },
        to: "/forgot-password",
    }));
    const __VLS_2 = __VLS_1({
        ...{ class: "tag" },
        to: "/forgot-password",
    }, ...__VLS_functionalComponentArgsRest(__VLS_1));
    __VLS_3.slots.default;
    var __VLS_3;
}
/** @type {__VLS_StyleScopedClasses['layout-shell']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['error-summary']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['error']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['error']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['tag']} */ ;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            form: form,
            fieldErr: fieldErr,
            topError: topError,
            submitting: submitting,
            token: token,
            submit: submit,
        };
    },
});
export default (await import('vue')).defineComponent({
    setup() {
        return {};
    },
});
; /* PartiallyEnd: #4569/main.vue */
