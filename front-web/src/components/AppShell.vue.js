import { computed } from 'vue';
import { useRoute } from 'vue-router';
import { sessionUser, isAdmin } from '@/app/session';
const route = useRoute();
const learnerNav = [
    { to: '/', label: '学习概览' },
    { to: '/practice/new', label: '创建练习' },
    { to: '/history', label: '练习历史' },
    { to: '/wrong-items', label: '错题本' },
    { to: '/knowledge', label: '知识点' },
];
const adminNav = [
    { to: '/admin', label: '内容概览' },
    { to: '/admin/questions', label: '题目' },
    { to: '/admin/imports', label: '导入任务' },
    { to: '/admin/knowledge', label: '知识点' },
    { to: '/admin/sources', label: '来源' },
    { to: '/admin/issues', label: '举报' },
];
const isAdminArea = computed(() => route.path.startsWith('/admin'));
const nav = computed(() => (isAdminArea.value ? adminNav : learnerNav));
function isActive(to) {
    if (to === '/') {
        return route.path === '/';
    }
    return route.path === to || route.path.startsWith(to + '/');
}
debugger; /* PartiallyEnd: #3632/scriptSetup.vue */
const __VLS_ctx = {};
let __VLS_components;
let __VLS_directives;
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({});
__VLS_asFunctionalElement(__VLS_intrinsicElements.header, __VLS_intrinsicElements.header)({
    ...{ class: "topbar" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "layout-shell topbar-inner" },
});
const __VLS_0 = {}.RouterLink;
/** @type {[typeof __VLS_components.RouterLink, typeof __VLS_components.RouterLink, ]} */ ;
// @ts-ignore
const __VLS_1 = __VLS_asFunctionalComponent(__VLS_0, new __VLS_0({
    ...{ class: "topbar-brand" },
    to: "/",
}));
const __VLS_2 = __VLS_1({
    ...{ class: "topbar-brand" },
    to: "/",
}, ...__VLS_functionalComponentArgsRest(__VLS_1));
__VLS_3.slots.default;
if (__VLS_ctx.isAdminArea) {
    __VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
        ...{ class: "tag" },
        'data-tone': "accent",
        ...{ style: {} },
    });
}
var __VLS_3;
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "topbar-actions" },
});
if (__VLS_ctx.isAdmin()) {
    const __VLS_4 = {}.RouterLink;
    /** @type {[typeof __VLS_components.RouterLink, typeof __VLS_components.RouterLink, ]} */ ;
    // @ts-ignore
    const __VLS_5 = __VLS_asFunctionalComponent(__VLS_4, new __VLS_4({
        to: "/",
        ...{ class: "tag" },
    }));
    const __VLS_6 = __VLS_5({
        to: "/",
        ...{ class: "tag" },
    }, ...__VLS_functionalComponentArgsRest(__VLS_5));
    __VLS_7.slots.default;
    var __VLS_7;
}
if (__VLS_ctx.isAdmin()) {
    const __VLS_8 = {}.RouterLink;
    /** @type {[typeof __VLS_components.RouterLink, typeof __VLS_components.RouterLink, ]} */ ;
    // @ts-ignore
    const __VLS_9 = __VLS_asFunctionalComponent(__VLS_8, new __VLS_8({
        to: "/admin",
        ...{ class: "tag" },
    }));
    const __VLS_10 = __VLS_9({
        to: "/admin",
        ...{ class: "tag" },
    }, ...__VLS_functionalComponentArgsRest(__VLS_9));
    __VLS_11.slots.default;
    var __VLS_11;
}
__VLS_asFunctionalElement(__VLS_intrinsicElements.span, __VLS_intrinsicElements.span)({
    ...{ class: "muted mono" },
});
(__VLS_ctx.sessionUser()?.email);
const __VLS_12 = {}.RouterLink;
/** @type {[typeof __VLS_components.RouterLink, typeof __VLS_components.RouterLink, ]} */ ;
// @ts-ignore
const __VLS_13 = __VLS_asFunctionalComponent(__VLS_12, new __VLS_12({
    to: "/settings",
    ...{ class: "tag" },
}));
const __VLS_14 = __VLS_13({
    to: "/settings",
    ...{ class: "tag" },
}, ...__VLS_functionalComponentArgsRest(__VLS_13));
__VLS_15.slots.default;
var __VLS_15;
__VLS_asFunctionalElement(__VLS_intrinsicElements.div, __VLS_intrinsicElements.div)({
    ...{ class: "layout-shell layout-body" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.aside, __VLS_intrinsicElements.aside)({
    ...{ class: "layout-sidebar" },
});
__VLS_asFunctionalElement(__VLS_intrinsicElements.nav, __VLS_intrinsicElements.nav)({
    'aria-label': "主导航",
});
for (const [item] of __VLS_getVForSourceType((__VLS_ctx.nav))) {
    const __VLS_16 = {}.RouterLink;
    /** @type {[typeof __VLS_components.RouterLink, typeof __VLS_components.RouterLink, ]} */ ;
    // @ts-ignore
    const __VLS_17 = __VLS_asFunctionalComponent(__VLS_16, new __VLS_16({
        key: (item.to),
        to: (item.to),
        dataActive: (__VLS_ctx.isActive(item.to)),
        'aria-current': (__VLS_ctx.isActive(item.to) ? 'page' : undefined),
    }));
    const __VLS_18 = __VLS_17({
        key: (item.to),
        to: (item.to),
        dataActive: (__VLS_ctx.isActive(item.to)),
        'aria-current': (__VLS_ctx.isActive(item.to) ? 'page' : undefined),
    }, ...__VLS_functionalComponentArgsRest(__VLS_17));
    __VLS_19.slots.default;
    (item.label);
    var __VLS_19;
}
__VLS_asFunctionalElement(__VLS_intrinsicElements.main, __VLS_intrinsicElements.main)({
    ...{ class: "layout-main" },
});
var __VLS_20 = {};
/** @type {__VLS_StyleScopedClasses['topbar']} */ ;
/** @type {__VLS_StyleScopedClasses['layout-shell']} */ ;
/** @type {__VLS_StyleScopedClasses['topbar-inner']} */ ;
/** @type {__VLS_StyleScopedClasses['topbar-brand']} */ ;
/** @type {__VLS_StyleScopedClasses['tag']} */ ;
/** @type {__VLS_StyleScopedClasses['topbar-actions']} */ ;
/** @type {__VLS_StyleScopedClasses['tag']} */ ;
/** @type {__VLS_StyleScopedClasses['tag']} */ ;
/** @type {__VLS_StyleScopedClasses['muted']} */ ;
/** @type {__VLS_StyleScopedClasses['mono']} */ ;
/** @type {__VLS_StyleScopedClasses['tag']} */ ;
/** @type {__VLS_StyleScopedClasses['layout-shell']} */ ;
/** @type {__VLS_StyleScopedClasses['layout-body']} */ ;
/** @type {__VLS_StyleScopedClasses['layout-sidebar']} */ ;
/** @type {__VLS_StyleScopedClasses['layout-main']} */ ;
// @ts-ignore
var __VLS_21 = __VLS_20;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            sessionUser: sessionUser,
            isAdmin: isAdmin,
            isAdminArea: isAdminArea,
            nav: nav,
            isActive: isActive,
        };
    },
});
const __VLS_component = (await import('vue')).defineComponent({
    setup() {
        return {};
    },
});
export default {};
; /* PartiallyEnd: #4569/main.vue */
