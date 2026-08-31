export type Habit = {
  id: string
  name: string
  group_name: string
  type: 'boolean' | 'numerical'
  goal_value: number
  goal_period: 'daily' | 'weekly' | 'monthly'
  is_negative: boolean
  unit: string
  archived_at: string | null
  created_at: string
  updated_at: string
  recent_entries?: { date: string; value: number }[]
  progress?: { current: number; target: number; success: boolean; period: string; percentage: number }
}

export type Entry = {
  id: string
  habit_id: string
  date: string
  value: number
  created_at: string
  updated_at: string
}

const BASE = (import.meta as any).env?.VITE_API_URL || ''

// --- Auth token storage with remember support ---
const TOKEN_KEY = 'auth_token'
const REMEMBER_KEY = 'auth_remember'

export function getAuthToken(): string | null {
  try {
    // sessionStorage has priority for current tab
    return sessionStorage.getItem(TOKEN_KEY) || localStorage.getItem(TOKEN_KEY)
  } catch { return null }
}

export function isRememberEnabled(): boolean {
  try { return localStorage.getItem(REMEMBER_KEY) === '1' } catch { return false }
}

export function setAuthToken(token: string, remember: boolean) {
  clearAuthToken()
  try {
    if (remember) {
      localStorage.setItem(TOKEN_KEY, token)
      localStorage.setItem(REMEMBER_KEY, '1')
    } else {
      sessionStorage.setItem(TOKEN_KEY, token)
      // remove remember flag for session mode
      localStorage.removeItem(REMEMBER_KEY)
    }
  } catch {}
}

export function clearAuthToken() {
  try {
    localStorage.removeItem(TOKEN_KEY)
    sessionStorage.removeItem(TOKEN_KEY)
  } catch {}
}

function authHeaders(): Record<string, string> {
  const token = getAuthToken()
  if (token) return { Authorization: `Bearer ${token}` }
  return {}
}

async function req<T>(path: string, opts?: RequestInit): Promise<T> {
  const isFormData = opts?.body instanceof FormData
  const baseHeaders: Record<string, string> = isFormData ? {} : { 'Content-Type': 'application/json' }
  const headers: Record<string, string> = { ...baseHeaders, ...authHeaders(), ...(opts?.headers as Record<string,string> || {}) }
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), 15000)
  let res: Response
  try {
    res = await fetch(BASE + path, { ...opts, headers, signal: controller.signal })
  } catch (e: any) {
    clearTimeout(timeout)
    if (e?.name === 'AbortError') throw new Error('Timeout - serveur injoignable')
    throw e
  }
  clearTimeout(timeout)
  if (res.status === 401) {
    const txt = await res.text()
    try { window.dispatchEvent(new CustomEvent('auth:unauthorized')) } catch {}
    throw new Error(txt || 'Unauthorized (401) - veuillez vous reconnecter')
  }
  if (!res.ok) {
    const txt = await res.text()
    throw new Error(txt || res.statusText)
  }
  if (res.status === 204) return undefined as T
  const text = await res.text()
  if (!text) return undefined as T
  try {
    return JSON.parse(text) as T
  } catch {
    throw new Error('Invalid JSON response')
  }
}

async function downloadCsv(path: string, filename: string) {
  const res = await fetch(BASE + path, { headers: { ...authHeaders() } })
  if (res.status === 401) {
    try { window.dispatchEvent(new CustomEvent('auth:unauthorized')) } catch {}
    const txt = await res.text()
    throw new Error(txt || 'Unauthorized')
  }
  if (!res.ok) {
    const txt = await res.text()
    throw new Error(txt || res.statusText)
  }
  const blob = await res.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

async function importCsv(path: string, file: File) {
  const fd = new FormData()
  fd.append('file', file)
  const res = await fetch(BASE + path, { method: 'POST', headers: { ...authHeaders() } as HeadersInit, body: fd })
  if (res.status === 401) {
    try { window.dispatchEvent(new CustomEvent('auth:unauthorized')) } catch {}
    const txt = await res.text()
    throw new Error(txt || 'Unauthorized')
  }
  if (!res.ok) {
    const txt = await res.text()
    throw new Error(txt || res.statusText)
  }
  const data = await res.json() as { imported: number; errors: string[] | string }
  // keep array for callers; normalize string to array for consistency
  if (typeof data.errors === 'string') {
    return { imported: data.imported, errors: data.errors ? [data.errors] : [] }
  }
  return { imported: data.imported, errors: Array.isArray(data.errors) ? data.errors : [] }
}

export const api = {
  // Auth
  login: async (password: string, remember: boolean) => {
    const res = await fetch(BASE + '/api/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password }),
    })
    if (!res.ok) {
      const txt = await res.text()
      throw new Error(txt || 'Mot de passe incorrect')
    }
    const data = await res.json() as { token: string }
    if (data.token) {
      setAuthToken(data.token, remember)
    }
    return data
  },
  logout: async () => {
    try { await fetch(BASE + '/api/logout', { method: 'POST', headers: { ...authHeaders() } }) } catch {}
    clearAuthToken()
  },
  authStatus: () => req<{ protected: boolean, authenticated: boolean }>('/api/auth/status'),

  listHabits: (includeArchived = false, withEntries?: number) => {
    const q = new URLSearchParams()
    if (includeArchived) q.set('include_archived', 'true')
    if (withEntries) {
      q.set('with_entries', String(withEntries))
      q.set('today', localDateStr(new Date()))
    } else {
      // still send today for progress computation to avoid server TZ divergence
      q.set('today', localDateStr(new Date()))
    }
    const qs = q.toString() ? '?' + q.toString() : ''
    return req<Habit[]>(`/api/habits${qs}`)
  },
  getHabit: (id: string) => {
    const q = new URLSearchParams()
    q.set('today', localDateStr(new Date()))
    return req<Habit>(`/api/habits/${id}?${q.toString()}`)
  },
  createHabit: (data: Partial<Habit>) => req<Habit>(`/api/habits`, { method: 'POST', body: JSON.stringify(data) }),
  updateHabit: (id: string, data: Partial<Habit>) => req<Habit>(`/api/habits/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  archiveHabit: (id: string) => req<{ status: string }>(`/api/habits/${id}/archive`, { method: 'POST' }),
  restoreHabit: (id: string) => req<{ status: string }>(`/api/habits/${id}/restore`, { method: 'POST' }),
  deleteHabit: (id: string) => req<void>(`/api/habits/${id}?hard=true`, { method: 'DELETE' }),
  getEntries: (habitId: string, from?: string, to?: string) => {
    const q = new URLSearchParams()
    if (from) q.set('from', from)
    if (to) q.set('to', to)
    const qs = q.toString() ? '?' + q.toString() : ''
    return req<Entry[]>(`/api/habits/${habitId}/entries${qs}`)
  },
  upsertEntry: (habitId: string, date: string, value: number) =>
    req<Entry>(`/api/habits/${habitId}/entries/${date}`, { method: 'PUT', body: JSON.stringify({ value }) }),
  deleteEntry: (habitId: string, date: string) =>
    req<void>(`/api/habits/${habitId}/entries/${date}`, { method: 'DELETE' }),
  exportHabits: () => downloadCsv('/api/export/habits', 'habits.csv'),
  exportEntries: () => downloadCsv('/api/export/entries', 'entries.csv'),
  // keep backward compat for old callers that expected URL string
  exportHabitsUrl: () => `${BASE}/api/export/habits`,
  exportEntriesUrl: () => `${BASE}/api/export/entries`,
  importHabits: (file: File) => importCsv('/api/import/habits', file),
  importEntries: (file: File) => importCsv('/api/import/entries', file),
}

// date helpers: local YYYY-MM-DD (manual, DST-safe)
export function localDateStr(d: Date = new Date()): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}
export function lastNDates(n: number, base: Date = new Date()): string[] {
  const out: string[] = []
  const baseMidday = new Date(base)
  baseMidday.setHours(12, 0, 0, 0)
  for (let i = n - 1; i >= 0; i--) {
    const dd = new Date(baseMidday)
    dd.setDate(baseMidday.getDate() - i)
    out.push(localDateStr(dd))
  }
  return out
}
export function formatMonth(d: Date, lang: 'fr' | 'en' = 'fr'): string {
  return d.toLocaleDateString(lang === 'en' ? 'en-US' : 'fr-FR', { month: 'long', year: 'numeric' })
}
