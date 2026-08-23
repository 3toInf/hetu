<script setup lang="ts">
// App shell: top nav + routed content.
</script>

<template>
  <header class="topnav">
    <span class="brand">Hetu</span>
    <nav>
      <RouterLink to="/projects">Projects</RouterLink>
      <RouterLink to="/sessions">Sessions</RouterLink>
      <RouterLink to="/search">Search</RouterLink>
    </nav>
  </header>
  <main class="content">
    <RouterView />
  </main>
</template>
