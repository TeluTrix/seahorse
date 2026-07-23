import type { AuthResponse, Movie, ScanStatus, TVShow, PublicUser } from '../types'

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
  scanLibrary: () => request<ScanStatus>('/admin/scan', { method: 'POST' }),
  scanStatus: () => request<ScanStatus>('/admin/scan/status'),
}

// Native <video> elements can't set an Authorization header, so the stream
// endpoint also accepts the JWT as a query param.
export function streamURL(kind: 'movies' | 'episodes', id: string): string {
  const token = localStorage.getItem(TOKEN_KEY) ?? ''
  return `${BASE}/stream/${kind}/${id}?token=${encodeURIComponent(token)}`
}

export { TOKEN_KEY }
