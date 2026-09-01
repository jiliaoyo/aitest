import { reactive, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { request } from '@/api/client';
import { fieldErrors } from '@/api/client';
import { refreshSession, safeRedirect } from '@/app/session';
const router = useRouter();
const route = useRoute();
const form = reactive({ email: '', password: '' });
const fieldErr = reactive({});
const topError = ref('');
const submitting = ref(false);
async function submit() {
    submitting.value = true;
    topError.value = '';
    for (const k of Object.keys(fieldErr))
        delete fieldErr[k];
    try {
        await request('/auth/login', { method: 'POST', body: form });
        await refreshSession();
        router.replace(safeRedirect(route.query.redirect));
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
__VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
    ...{ class: "muted" },
});
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
    for: "email",
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.input)({
    id: "email",
    type: "email",
    autocomplete: "email",
    'aria-describedby': (__VLS_ctx.fieldErr.email ? 'email-err' : undefined),
});
(__VLS_ctx.form.email);
if (__VLS_ctx.fieldErr.email) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        id: "email-err",
        ...{ class: "error" },
    });
    (__VLS_ctx.fieldErr.email);
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
    autocomplete: "current-password",
    'aria-describedby': (__VLS_ctx.fieldErr.password ? 'password-err' : undefined),
});
(__VLS_ctx.form.password);
if (__VLS_ctx.fieldErr.password) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
        id: "password-err",
        ...{ class: "error" },
    });
    (__VLS_ctx.fieldErr.password);
}
__VLS_asFunctionalElement(__VLS_intrinsicElements.button, __VLS_intrinsicElements.button)({
    ...{ class: "primary" },
    type: "submit",
    disabled: (__VLS_ctx.submitting),
    ...{ style: {} },
});
(__VLS_ctx.submitting ? '登录中' : '登录');
__VLS_asFunctionalElement(__VLS_intrinsicElements.p, __VLS_intrinsicElements.p)({
    ...{ style: {} },
});
const __VLS_0 = {}.RouterLink;
/** @type {[typeof __VLS_components.RouterLink, typeof __VLS_components.RouterLink, ]} */ ;
// @ts-ignore
const __VLS_1 = __VLS_asFunctionalComponent(__VLS_0, new __VLS_0({
    to: "/forgot-password",
}));
const __VLS_2 = __VLS_1({
    to: "/forgot-password",
}, ...__VLS_functionalComponentArgsRest(__VLS_1));
__VLS_3.slots.default;
var __VLS_3;
const __VLS_4 = {}.RouterLink;
/** @type {[typeof __VLS_components.RouterLink, typeof __VLS_components.RouterLink, ]} */ ;
// @ts-ignore
const __VLS_5 = __VLS_asFunctionalComponent(__VLS_4, new __VLS_4({
    to: "/register",
}));
const __VLS_6 = __VLS_5({
    to: "/register",
}, ...__VLS_functionalComponentArgsRest(__VLS_5));
__VLS_7.slots.default;
var __VLS_7;
/** @type {__VLS_StyleScopedClasses['layout-shell']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['card']} */ ;
/** @type {__VLS_StyleScopedClasses['error-summary']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['error']} */ ;
/** @type {__VLS_StyleScopedClasses['field']} */ ;
/** @type {__VLS_StyleScopedClasses['error']} */ ;
/** @type {__VLS_StyleScopedClasses['primary']} */ ;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            form: form,
            fieldErr: fieldErr,
            topError: topError,
            submitting: submitting,
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
