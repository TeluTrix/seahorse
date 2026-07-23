import { createApp } from 'vue'
import { createPinia } from 'pinia'
import './style.css'
import App from './App.vue'
import router from './router'
import { useAuthStore } from './stores/auth'

async function bootstrap() {
  const app = createApp(App)
  app.use(createPinia())

  // Resolve the current user (if a token is stored) before the router runs
  // its first navigation guard, otherwise a hard reload on an admin-only
  // route would see auth.user as null and bounce to home.
  const auth = useAuthStore()
  await auth.fetchMe().catch(() => auth.logout())

  app.use(router)
  app.mount('#app')
}

bootstrap()
