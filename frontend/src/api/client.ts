import type {
  AuthResponse,
  ClientConfig,
  MediaType,
  Movie,
  MoviesPage,
  Progress,
  PublicUser,
  ScanStatus,
  SearchResult,
  SubtitleTrack,
  TVShow,
  TVShowsPage,
} from '../types'

const BASE = '/api'
const TOKEN_KEY = 'seahorse_token'

function authHeaders(): Record<string, string> {
  const token = localStorage.getItem(TOKEN_KEY)
  return token ? { Authorization: `Bearer ${token}` } : {}
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(BASE + path, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...authHeaders(),
      ...(options.headers as Record<string, string> | undefined),
    },
  })

  if (!res.ok) {
    let message = res.statusText
    try {
      const body = await res.json()
      message = body.error ?? message
    } catch {
      // response had no JSON body
    }
    throw new Error(message)
  }

  if (res.status === 204) {
    return undefined as T
  }
  return res.json() as Promise<T>
}

export const api = {
  register: (email: string, password: string) =>
    request<AuthResponse>('/auth/register', {
      method: 'POST',
      body: JSON.stringify({ user_email: email, user_password: password }),
    }),
  login: (email: string, password: string) =>
    request<AuthResponse>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ user_email: email, user_password: password }),
    }),
  me: () => request<PublicUser>('/user/me'),
  listMovies: (page = 1, pageSize = 48, sort?: 'newest') =>
    request<MoviesPage>(`/movies?page=${page}&page_size=${pageSize}${sort ? `&sort=${sort}` : ''}`),
  getMovie: (id: string) => request<Movie>(`/movies/${id}`),
  listTVShows: (page = 1, pageSize = 48, sort?: 'newest') =>
    request<TVShowsPage>(`/tvshows?page=${page}&page_size=${pageSize}${sort ? `&sort=${sort}` : ''}`),
  getTVShow: (id: string) => request<TVShow>(`/tvshows/${id}`),
  search: (params: {
    q?: string
    year?: string
    genre?: string
    type?: 'movies' | 'tvshows'
    page?: number
    pageSize?: number
  }) => {
    const query = new URLSearchParams()
    if (params.q) query.set('q', params.q)
    if (params.year) query.set('year', params.year)
    if (params.genre) query.set('genre', params.genre)
    if (params.type) query.set('type', params.type)
    query.set('page', String(params.page ?? 1))
    query.set('page_size', String(params.pageSize ?? 48))
    return request<SearchResult>(`/search?${query.toString()}`)
  },
  listGenres: () => request<string[]>('/genres'),
  getConfig: () => request<ClientConfig>('/config'),
  scanLibrary: (full = false) => request<ScanStatus>(`/admin/scan${full ? '?mode=full' : ''}`, { method: 'POST' }),
  listUsers: () => request<PublicUser[]>('/admin/users'),
  createUser: (email: string, password: string) =>
    request<PublicUser>('/admin/users', {
      method: 'POST',
      body: JSON.stringify({ user_email: email, user_password: password }),
    }),
  setUserPassword: (userId: string, newPassword: string) =>
    request<{ ok: boolean }>(`/admin/users/${userId}/password`, {
      method: 'PUT',
      body: JSON.stringify({ new_password: newPassword }),
    }),
  saveProgress: (mediaType: MediaType, mediaId: string, positionSeconds: number, durationSeconds: number) =>
    request<Progress>('/progress', {
      method: 'PUT',
      body: JSON.stringify({
        media_type: mediaType,
        media_id: mediaId,
        position_seconds: positionSeconds,
        duration_seconds: durationSeconds,
      }),
    }),
  getProgress: async (mediaType: MediaType, mediaId: string): Promise<Progress | null> => {
    try {
      return await request<Progress>(`/progress/${mediaType}/${mediaId}`)
    } catch {
      return null
    }
  },
  listSubtitles: (kind: 'movies' | 'episodes', id: string) => request<SubtitleTrack[]>(`/subtitles/${kind}/${id}`),
}

// Native <video>/<img>/<track> elements can't set an Authorization header, so
// these endpoints also accept the JWT as a query param.
export function streamURL(kind: 'movies' | 'episodes', id: string): string {
  const token = localStorage.getItem(TOKEN_KEY) ?? ''
  return `${BASE}/stream/${kind}/${id}?token=${encodeURIComponent(token)}`
}

export function coverURL(kind: 'movies' | 'tvshows', id: string): string {
  const token = localStorage.getItem(TOKEN_KEY) ?? ''
  return `${BASE}/images/${kind}/${id}/cover?token=${encodeURIComponent(token)}`
}

export function subtitleURL(kind: 'movies' | 'episodes', id: string, trackId: string): string {
  const token = localStorage.getItem(TOKEN_KEY) ?? ''
  return `${BASE}/subtitles/${kind}/${id}/vtt?track=${encodeURIComponent(trackId)}&token=${encodeURIComponent(token)}`
}

// EventSource can't set an Authorization header either, so the live scan
// status stream also takes the token as a query param.
export function scanEventsURL(): string {
  const token = localStorage.getItem(TOKEN_KEY) ?? ''
  return `${BASE}/admin/scan/events?token=${encodeURIComponent(token)}`
}

export { TOKEN_KEY }
