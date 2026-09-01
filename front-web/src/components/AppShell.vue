<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { sessionUser, isAdmin } from '@/app/session'

const route = useRoute()

const learnerNav = [
  { to: '/', label: '学习概览' },
  { to: '/practice/new', label: '创建练习' },
  { to: '/history', label: '练习历史' },
  { to: '/wrong-items', label: '错题本' },
  { to: '/knowledge', label: '知识点' },
]

const adminNav = [
  { to: '/admin', label: '内容概览' },
  { to: '/admin/questions', label: '题目' },
  { to: '/admin/knowledge', label: '知识点' },
  { to: '/admin/sources', label: '来源' },
  { to: '/admin/issues', label: '举报' },
]

const isAdminArea = computed(() => route.path.startsWith('/admin'))
const nav = computed(() => (isAdminArea.value ? adminNav : learnerNav))

function isActive(to: string): boolean {
  if (to === '/') {
    return route.path === '/'
  }
  return route.path === to || route.path.startsWith(to + '/')
}
</script>

<template>
  <div>
    <header class="topbar">
      <div class="layout-shell topbar-inner">
        <RouterLink class="topbar-brand" to="/">
          AI 刷题<span v-if="isAdminArea" class="tag" data-tone="accent" style="margin-left: 10px">管理端</span>
        </RouterLink>
        <div class="topbar-actions">
          <RouterLink v-if="isAdmin()" to="/" class="tag">返回学习端</RouterLink>
          <RouterLink v-if="isAdmin()" to="/admin" class="tag">进入管理端</RouterLink>
          <span class="muted mono">{{ sessionUser()?.email }}</span>
          <RouterLink to="/settings" class="tag">设置</RouterLink>
        </div>
      </div>
    </header>
    <div class="layout-shell layout-body">
      <aside class="layout-sidebar">
        <nav aria-label="主导航">
          <RouterLink
            v-for="item in nav"
            :key="item.to"
            :to="item.to"
            :data-active="isActive(item.to)"
            :aria-current="isActive(item.to) ? 'page' : undefined"
          >
            {{ item.label }}
          </RouterLink>
        </nav>
      </aside>
      <main class="layout-main">
        <slot />
      </main>
    </div>
  </div>
</template>
