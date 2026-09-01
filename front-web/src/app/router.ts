import { createRouter, createWebHistory } from 'vue-router'
import { ensureSessionReady, isAdmin, safeRedirect, sessionUser } from '@/app/session'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    // 公共
    { path: '/login', component: () => import('@/features/auth/LoginPage.vue') },
    { path: '/register', component: () => import('@/features/auth/RegisterPage.vue') },
    { path: '/forgot-password', component: () => import('@/features/auth/ForgotPasswordPage.vue') },
    { path: '/reset-password', component: () => import('@/features/auth/ResetPasswordPage.vue') },
    // 学习端
    { path: '/', component: () => import('@/features/dashboard/DashboardPage.vue') },
    { path: '/practice/new', component: () => import('@/features/practice/CreatePracticePage.vue') },
    { path: '/practice/:sessionId', component: () => import('@/features/practice/PracticePage.vue') },
    { path: '/practice/:sessionId/result', component: () => import('@/features/practice/PracticeResultPage.vue') },
    { path: '/history', component: () => import('@/features/practice/PracticeHistoryPage.vue') },
    { path: '/wrong-items', component: () => import('@/features/practice/WrongItemsPage.vue') },
    { path: '/knowledge', component: () => import('@/features/knowledge/KnowledgeListPage.vue') },
    { path: '/knowledge/:knowledgePointId', component: () => import('@/features/knowledge/KnowledgeDetailPage.vue') },
    { path: '/settings', component: () => import('@/features/settings/SettingsPage.vue') },
    // 管理端
    { path: '/admin', component: () => import('@/features/admin/AdminOverviewPage.vue') },
    { path: '/admin/questions', component: () => import('@/features/admin/AdminQuestionListPage.vue') },
    { path: '/admin/questions/new', component: () => import('@/features/admin/AdminQuestionEditPage.vue') },
    { path: '/admin/questions/:questionId', component: () => import('@/features/admin/AdminQuestionEditPage.vue') },
    { path: '/admin/knowledge', component: () => import('@/features/admin/AdminKnowledgePage.vue') },
    { path: '/admin/sources', component: () => import('@/features/admin/AdminSourcesPage.vue') },
    { path: '/admin/issues', component: () => import('@/features/admin/AdminIssuesPage.vue') },
    // 兜底
    { path: '/:pathMatch(.*)*', component: () => import('@/features/misc/NotFoundPage.vue') },
    { path: '/forbidden', component: () => import('@/features/misc/ForbiddenPage.vue') },
  ],
})

router.beforeEach(async (to) => {
  await ensureSessionReady()
  const loggedIn = sessionUser() !== null

  const publicRoutes = ['/login', '/register', '/forgot-password', '/reset-password']
  if (!loggedIn && !publicRoutes.includes(to.path)) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }
  if (loggedIn && publicRoutes.includes(to.path)) {
    return { path: '/' }
  }
  if (to.path.startsWith('/admin') && !isAdmin()) {
    return loggedIn ? { path: '/forbidden' } : { path: '/login', query: { redirect: safeRedirect(to.fullPath) } }
  }
  return true
})

export default router
