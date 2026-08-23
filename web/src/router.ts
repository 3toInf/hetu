// Client-side routes. Views are lazy-loaded (code-split) per route.

import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/projects' },
    { path: '/projects', component: () => import('./views/ProjectsView.vue') },
    { path: '/sessions', component: () => import('./views/SessionsView.vue') },
    { path: '/sessions/:id', component: () => import('./views/SessionView.vue') },
    { path: '/search', component: () => import('./views/SearchView.vue') }
  ]
})

export default router
