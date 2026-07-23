import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import LoginView from '../views/LoginView.vue'
import RegisterView from '../views/RegisterView.vue'
import HomeView from '../views/HomeView.vue'
import SearchView from '../views/SearchView.vue'
import MoviesView from '../views/MoviesView.vue'
import TVShowsView from '../views/TVShowsView.vue'
import MovieDetailView from '../views/MovieDetailView.vue'
import TVShowDetailView from '../views/TVShowDetailView.vue'
import PlayerView from '../views/PlayerView.vue'
import AdminLayout from '../components/AdminLayout.vue'
import AdminLibraryView from '../views/AdminLibraryView.vue'
import AdminUsersView from '../views/AdminUsersView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: LoginView, meta: { public: true } },
    { path: '/register', name: 'register', component: RegisterView, meta: { public: true } },
    { path: '/', name: 'home', component: HomeView },
    { path: '/search', name: 'search', component: SearchView },
    { path: '/movies', name: 'movies-overview', component: MoviesView },
    { path: '/tvshows', name: 'tvshows-overview', component: TVShowsView },
    { path: '/movies/:id', name: 'movie', component: MovieDetailView, props: true },
    { path: '/tvshows/:id', name: 'tvshow', component: TVShowDetailView, props: true },
    { path: '/watch/movie/:id', name: 'watch-movie', component: PlayerView, props: true },
    { path: '/watch/episode/:id', name: 'watch-episode', component: PlayerView, props: true },
    {
      path: '/admin',
      component: AdminLayout,
      meta: { requiresAdmin: true },
      children: [
        { path: '', redirect: { name: 'admin-library' } },
        { path: 'library', name: 'admin-library', component: AdminLibraryView },
        { path: 'users', name: 'admin-users', component: AdminUsersView },
      ],
    },
  ],
})

router.beforeEach((to) => {
  const auth = useAuthStore()

  if (!to.meta.public && !auth.isAuthenticated) {
    return { name: 'login' }
  }
  if (to.meta.requiresAdmin && !auth.isAdmin) {
    return { name: 'home' }
  }
  return true
})

export default router
