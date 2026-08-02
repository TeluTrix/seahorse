// Runtime-tunable values served by GET /api/config (see api.ClientConfig on
// the backend) — the frontend is a prebuilt static bundle, so it can't read
// the server's env vars directly.
export interface ClientConfig {
  default_page_size: number
  player_seek_seconds: number
  resume_threshold_seconds: number
  progress_report_interval_seconds: number
  registration_enabled: boolean
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
  remux_status?: 'pending' | 'active'
  tagline?: string
  original_language?: string
  budget?: number
  revenue?: number
  production_companies?: string
  production_countries?: string
}

export interface MediaInfo {
  container: string
  file_size_bytes: number
  bitrate_kbps?: number
  video_codec?: string
  width?: number
  height?: number
  audio_codec?: string
  audio_channels?: number
}

export interface Episode {
  id: string
  episode_number: number
  title: string
  overview: string
  still_url: string
  runtime_minutes?: number
  progress?: Progress
  remux_status?: 'pending' | 'active'
}

// Enough about an episode's place in its show for the player's breadcrumb
// trail — the player itself only ever receives an episode id.
export interface EpisodeContext {
  id: string
  title: string
  overview: string
  episode_number: number
  season_number: number
  show_id: string
  show_title: string
}

// The episode that plays right after a given one, for the player's "Watch
// Next" prompt.
export interface NextEpisode {
  id: string
  title: string
  episode_number: number
  season_number: number
  still_url: string
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

export interface Actor {
  name: string
  profile_url?: string
  credits: number
}

export interface ActorsPage {
  actors: Actor[]
  page: number
  page_size: number
  total: number
}

export interface ActorFilmography {
  name: string
  movies: Movie[]
  tv_shows: TVShow[]
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
