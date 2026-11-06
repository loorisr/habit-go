import { useEffect, useState, useCallback, useMemo, useRef } from 'react'
import { Link } from 'react-router-dom'
import { api, Habit, lastNDates, localDateStr } from '../api'
import HabitTable from '../components/HabitTable'
import { useI18n } from '../i18n'

const cellKey = (habitId: string, date: string) => `${habitId}|${date}`

export default function MainPage() {
  const { t } = useI18n()
  const [habits, setHabits] = useState<Habit[]>([])
  const [includeArchived, setIncludeArchived] = useState(false)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [dayKey, setDayKey] = useState(() => localDateStr())
  const dayRef = useRef(dayKey)

  // A3: last value we want for each cell (more recent than the rendered state during fast clicks),
  // the running PUT per cell, and counters that stop a stale refresh from overwriting a click.
  const latestValues = useRef(new Map<string, number>())
  const inFlight = useRef(new Map<string, Promise<void>>())
  const generation = useRef(0)
  const pendingSaves = useRef(0)

  const dates = useMemo(() => lastNDates(3), [dayKey])

  useEffect(() => {
    let cancelled = false
    const fetchHabits = async () => {
      setLoading(true)
      setError(null)
      try {
        const data = await api.listHabits(includeArchived, 3)
        if (!cancelled) setHabits(Array.isArray(data) ? data : [])
      } catch (e: any) {
        if (!cancelled && e.name !== 'AbortError') setError(e.message)
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    fetchHabits()
    return () => {
      cancelled = true
    }
  }, [includeArchived])

  const getValue = (h: Habit, date: string) => h.recent_entries?.find(e => e.date === date)?.value

  const optimisticUpdate = (habitId: string, date: string, newVal: number | undefined) => {
    setHabits(prev => prev.map(h => {
      if (h.id !== habitId) return h
      const entries = h.recent_entries ? [...h.recent_entries] : []
      const idx = entries.findIndex(e => e.date === date)
      if (newVal === undefined) {
        if (idx >= 0) entries.splice(idx, 1)
      } else {
        if (idx >= 0) entries[idx] = { date, value: newVal }
        else entries.push({ date, value: newVal })
      }
      return { ...h, recent_entries: entries }
    }))
  }

  // silent refresh (progress, server truth). Ignored if a click happened since it started,
  // or while saves are pending: the last save's refresh will apply.
  const refreshSilent = useCallback(async () => {
    const gen = generation.current
    try {
      const data = await api.listHabits(includeArchived, 3)
      if (gen === generation.current && pendingSaves.current === 0) {
        latestValues.current.clear()
        setHabits(Array.isArray(data) ? data : [])
      }
    } catch {}
  }, [includeArchived])

  // Keep the three day columns right after midnight or when the tab comes back (A5).
  useEffect(() => {
    const syncDay = () => {
      const now = localDateStr()
      if (now !== dayRef.current) {
        dayRef.current = now
        setDayKey(now)
        refreshSilent()
      }
    }
    const onVisible = () => {
      if (document.visibilityState === 'visible') {
        syncDay()
        refreshSilent()
      }
    }
    const timer = window.setInterval(syncDay, 60_000)
    document.addEventListener('visibilitychange', onVisible)
    return () => {
      clearInterval(timer)
      document.removeEventListener('visibilitychange', onVisible)
    }
  }, [refreshSilent])

  // Writes for one cell are serialized: while a PUT is running, newer values are queued
  // as "latest" and sent next, so the last click always wins.
  const saveCell = (habitId: string, date: string, value: number): Promise<void> => {
    const key = cellKey(habitId, date)
    latestValues.current.set(key, value)
    const running = inFlight.current.get(key)
    if (running) return running
    const run = (async () => {
      try {
        let sent: number
        do {
          sent = latestValues.current.get(key)!
          await api.upsertEntry(habitId, date, sent)
        } while (latestValues.current.get(key) !== sent)
      } finally {
        inFlight.current.delete(key)
      }
    })()
    inFlight.current.set(key, run)
    return run
  }

  const applyCell = (habit: Habit, date: string, next: number) => {
    setSaveError(null)
    generation.current++
    pendingSaves.current++
    optimisticUpdate(habit.id, date, next)
    saveCell(habit.id, date, next)
      .catch(() => setSaveError(t('saveError')))
      .finally(() => {
        pendingSaves.current--
        refreshSilent()
      })
  }

  const currentValue = (habit: Habit, date: string) =>
    latestValues.current.get(cellKey(habit.id, date)) ?? getValue(habit, date)

  const handleToggle = (habit: Habit, date: string) => {
    const cur = currentValue(habit, date)
    const next = habit.type === 'boolean'
      ? (cur && cur >= 1 ? 0 : 1)
      : (cur ?? 0) + 1
    applyCell(habit, date, next)
  }

  const handleDecrement = (habit: Habit, date: string) => {
    const cur = currentValue(habit, date) ?? 0
    const next = habit.type === 'boolean' ? 0 : Math.max(0, cur - 1)
    applyCell(habit, date, next)
  }

  if (loading) return <div className="py-10 text-center text-gray-400">{t('loading')}</div>
  if (error) return <div className="py-10 text-center text-red-500">{error}</div>

  // global progression split by period: daily / weekly / monthly.
  // C5: only active positive habits count (a negative "max" percentage is not a progress),
  // and the average is capped at 100% so one big overshoot does not hide the rest.
  const computeGlobal = (period: 'daily' | 'weekly' | 'monthly') => {
    const filtered = habits.filter(h => h.progress && h.goal_period === period && !h.archived_at && !h.is_negative)
    if (filtered.length === 0) return null
    const successCount = filtered.filter(h => h.progress!.success).length
    const total = filtered.length
    const successRate = (successCount / total) * 100
    const avgPercentage = filtered.reduce((s, h) => s + Math.min(100, h.progress!.percentage || 0), 0) / total
    return { successCount, total, successRate, avgPercentage }
  }
  const globalDaily = computeGlobal('daily')
  const globalWeekly = computeGlobal('weekly')
  const globalMonthly = computeGlobal('monthly')
  const hasAnyGlobal = !!(globalDaily || globalWeekly || globalMonthly)

  const renderGlobal = (
    label: string,
    data: ReturnType<typeof computeGlobal>,
    emptyMsg: string,
  ) => {
    if (!data) {
      return (
        <div className="bg-white rounded shadow p-4 opacity-60">
          <div className="flex justify-between items-center mb-2">
            <span className="font-semibold text-sm">{label}</span>
            <span className="text-xs text-gray-400">{emptyMsg}</span>
          </div>
          <div className="w-full bg-gray-200 rounded-full h-3">
            <div className="h-3 rounded-full bg-gray-300" style={{ width: '0%' }} />
          </div>
          <div className="text-xs text-gray-400 mt-1 flex items-center justify-between">
            <span>{t('noHabit')}</span>
            <Link to="/habits/new" className="text-green-600 underline text-xs">{t('create')}</Link>
          </div>
        </div>
      )
    }
    const avgDisplay = `${Math.round(data.avgPercentage)}%`
    return (
      <div className="bg-white rounded shadow p-4">
        <div className="flex justify-between items-center mb-2">
          <span className="font-semibold text-sm">{label}</span>
          <span className="text-xs text-gray-500">
            {data.successCount}/{data.total} {t('succeeded')} • {avgDisplay} {t('avg')}
          </span>
        </div>
        <div className="w-full bg-gray-200 rounded-full h-3">
          <div
            className={`h-3 rounded-full transition-all ${data.successRate === 100 ? 'bg-green-500' : data.successRate >= 50 ? 'bg-yellow-500' : 'bg-red-500'}`}
            style={{ width: `${Math.min(100, Math.max(0, data.successRate))}%` }}
          />
        </div>
        <div className="flex justify-end text-xs text-gray-400 mt-1">
          <span>{data.successRate === 100 ? t('allSucceeded') : data.successRate >= 50 ? t('inProgress') : t('toImprove')}</span>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap gap-2 items-center text-sm">
        <label className="flex items-center gap-1">
          <input type="checkbox" checked={includeArchived} onChange={e => setIncludeArchived(e.target.checked)} />
          {t('seeArchived')}
        </label>
        <span className="ml-auto text-xs text-gray-400">{t('tip')}</span>
      </div>

      {saveError && <div role="alert" className="bg-red-100 text-red-700 p-2 rounded text-sm">{saveError}</div>}

      {hasAnyGlobal ? (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
          {renderGlobal(`${t('globalDaily')} (${t('daily')})`, globalDaily, t('globalNoHabitDaily'))}
          {renderGlobal(`${t('globalWeekly')} (${t('weekly')})`, globalWeekly, t('globalNoHabitWeekly'))}
          {renderGlobal(`${t('globalMonthly')} (${t('monthly')})`, globalMonthly, t('globalNoHabitMonthly'))}
        </div>
      ) : habits.length > 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
          {renderGlobal(`${t('globalDaily')} (${t('daily')})`, null, t('globalNoHabitDaily'))}
          {renderGlobal(`${t('globalWeekly')} (${t('weekly')})`, null, t('globalNoHabitWeekly'))}
          {renderGlobal(`${t('globalMonthly')} (${t('monthly')})`, null, t('globalNoHabitMonthly'))}
        </div>
      ) : null}

      <HabitTable habits={habits} dates={dates} onToggle={handleToggle} onDecrement={handleDecrement} />
    </div>
  )
}
