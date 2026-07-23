import { createApp } from 'vue'
import { createPinia } from 'pinia'
import './style.css'
import App from './App.vue'
import router from './router'
import { useAuthStore } from './stores/auth'
import { useConfigStore } from './stores/config'

async function bootstrap() {
  const app = createApp(App)
  app.use(createPinia())

  // Resolve the current user (if a token is stored) before the router runs
  // its first navigation guard, otherwise a hard reload on an admin-only
  // route would see auth.user as null and bounce to home. Config fetch
  // failing is non-fatal — the store's own defaults mirror the backend's,
  // so the app still behaves sensibly if this is unreachable.
  const auth = useAuthStore()
  const config = useConfigStore()
  await Promise.all([auth.fetchMe().catch(() => auth.logout()), config.fetch().catch(() => {})])

  app.use(router)
  app.mount('#app')
}

bootstrap()
