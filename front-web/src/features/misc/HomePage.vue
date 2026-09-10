<script setup lang="ts">
import { sessionUser } from '@/app/session'
import DashboardPage from '@/features/dashboard/DashboardPage.vue'
import PublicHomePage from '@/features/auth/PublicHomePage.vue'

const loggedIn = sessionUser() !== null
</script>

<template>
  <DashboardPage v-if="loggedIn" />
  <PublicHomePage v-else />
</template>
