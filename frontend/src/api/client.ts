import type {
  AuthResponse,
  MediaType,
  Movie,
  Progress,
  PublicUser,
  ScanStatus,
  SubtitleTrack,
  TVShow,
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
  listMovies: () => request<Movie[]>('/movies'),
  getMovie: (id: string) => request<Movie>(`/movies/${id}`),
  listTVShows: () => request<TVShow[]>('/tvshows'),
  getTVShow: (id: string) => request<TVShow>(`/tvshows/${id}`),
  scanLibrary: (full = false) => request<ScanStatus>(`/admin/scan${full ? '?mode=full' : ''}`, { method: 'POST' }),
  scanStatus: () => request<ScanStatus>('/admin/scan/status'),
  listUsers: () => request<PublicUser[]>('/admin/users'),
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

export { TOKEN_KEY }
