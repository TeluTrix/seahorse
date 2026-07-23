// Runtime-tunable values served by GET /api/config (see api.ClientConfig on
// the backend) — the frontend is a prebuilt static bundle, so it can't read
// the server's env vars directly.
export interface ClientConfig {
  default_page_size: number
  player_seek_seconds: number
  resume_threshold_seconds: number
  progress_report_interval_seconds: number
}

export type Role = 'user' | 'admin'

export interface PublicUser {
  user_id: string
  user_email: string
  user_role: Role
}

export interface AuthResponse {
  token: string
  user: PublicUser
}

export interface Progress {
  position_seconds: number
  duration_seconds: number
  completed: boolean
  updated_at: string
}

export interface CastMember {
  name: string
  character: string
  profile_url?: string
}

export interface Movie {
  id: string
  title: string
  overview: string
  poster_url: string
  backdrop_url: string
  has_local_cover: boolean
  release_date: string
  vote_average: number
  genres: string
  runtime_minutes?: number
  director?: string
  cast?: CastMember[]
  progress?: Progress
}

export interface Episode {
  id: string
  episode_number: number
  title: string
  overview: string
  still_url: string
  runtime_minutes?: number
  progress?: Progress
}

export interface Season {
  id: string
  season_number: number
  episodes: Episode[]
}

export interface TVShow {
  id: string
  title: string
  overview: string
  poster_url: string
  backdrop_url: string
  has_local_cover: boolean
  first_air_date: string
  vote_average: number
  genres: string
  creators?: string
  cast?: CastMember[]
  seasons?: Season[]
}

export type ScanState = 'idle' | 'running' | 'done' | 'error'

export interface RemuxJob {
  file: string
  percent: number
}

export interface ScanStatus {
  state: ScanState
  current_item?: string
  movies_found: number
  shows_found: number
  episodes_found: number
  remux_jobs?: RemuxJob[]
  error?: string
  started_at?: string
  finished_at?: string
}

export interface MoviesPage {
  movies: Movie[]
  page: number
  page_size: number
  total: number
}

export interface TVShowsPage {
  tv_shows: TVShow[]
  page: number
  page_size: number
  total: number
}

export interface SearchResult {
  movies: Movie[]
  movies_total: number
  tv_shows: TVShow[]
  tv_shows_total: number
  page: number
  page_size: number
}

export type MediaType = 'movie' | 'episode'

export type SubtitleSource = 'external' | 'embedded'

export interface SubtitleTrack {
  id: string
  label: string
  language: string
  source: SubtitleSource
}
