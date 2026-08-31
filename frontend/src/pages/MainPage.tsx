import { useEffect, useState, useCallback, useMemo } from 'react'
import { api, Habit, lastNDates } from '../api'
import HabitTable from '../components/HabitTable'
import { useI18n } from '../i18n'

export default function MainPage() {
  const { t } = useI18n()
  const [habits, setHabits] = useState<Habit[]>([])
  const [includeArchived, setIncludeArchived] = useState(false)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const dates = useMemo(() => lastNDates(3), [])

  useEffect(() => {
    let cancelled = false
    const fetchHabits = async () => {
      setLoading(true)
      setError(null)
      try {
        const data = await api.listHabits(includeArchived, 3)
        if (!cancelled) setHabits(data)
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

  const handleToggle = async (habit: Habit, date: string) => {
    const cur = getValue(habit, date)
    let next: number | undefined
    if (habit.type === 'boolean') {
      next = cur && cur >= 1 ? 0 : 1
      if (next === 0) {
        // For boolean 0 we could delete entry or set 0; backend stores 0; but UI shows - for 0. We'll upsert 0
        // Keep 0 as value to allow toggle; alternatively delete. Keep 0.
      }
    } else {
      next = (cur ?? 0) + 1
    }
    // optimistic
    optimisticUpdate(habit.id, date, next)
    try {
      await api.upsertEntry(habit.id, date, next!)
    } catch (e) {
      // revert
      optimisticUpdate(habit.id, date, cur)
    }
  }

  const handleDecrement = async (habit: Habit, date: string) => {
    if (habit.type === 'boolean') {
      // for boolean, long press could clear?
      optimisticUpdate(habit.id, date, 0)
      try { await api.upsertEntry(habit.id, date, 0) } catch {}
      return
    }
    const cur = getValue(habit, date) ?? 0
    const next = Math.max(0, cur - 1)
    optimisticUpdate(habit.id, date, next)
    try {
      if (next === 0) {
        // keep 0 or delete? keep 0 to show 0
        await api.upsertEntry(habit.id, date, 0)
      } else {
        await api.upsertEntry(habit.id, date, next)
      }
    } catch {
      optimisticUpdate(habit.id, date, cur)
    }
  }

  // silent refresh for progress update after optimistic mutation (no loading spinner) — with abort guard to avoid race
  const refreshSilent = useCallback(async () => {
    try {
      const data = await api.listHabits(includeArchived, 3)
      setHabits(data)
    } catch {}
  }, [includeArchived])

  if (loading) return <div className="py-10 text-center text-gray-400">{t('loading')}</div>
  if (error) return <div className="py-10 text-center text-red-500">{error}</div>

  // global progression split by period: daily / weekly / monthly
  const computeGlobal = (period: 'daily' | 'weekly' | 'monthly') => {
    const filtered = habits.filter(h => h.progress && h.goal_period === period)
    if (filtered.length === 0) return null
    const successCount = filtered.filter(h => h.progress!.success).length
    const total = filtered.length
    const successRate = (successCount / total) * 100
    const avgPercentage = filtered.reduce((s, h) => s + (h.progress!.percentage || 0), 0) / total
    const sumCurrent = filtered.reduce((s, h) => s + h.progress!.current, 0)
    const sumTarget = filtered.reduce((s, h) => s + h.progress!.target, 0)
    const cumulPercentage = sumTarget > 0 ? (sumCurrent / sumTarget) * 100 : 0
    return { successCount, total, successRate, avgPercentage, sumCurrent, sumTarget, cumulPercentage }
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
            <a href="/habits/new" className="text-green-600 underline text-xs">{t('create')}</a>
          </div>
        </div>
      )
    }
    const avgDisplay = Math.round(data.avgPercentage) > 100 ? `100% (+${Math.round(data.avgPercentage) - 100}%)` : `${Math.round(data.avgPercentage)}%`
    const cumulDisplay = Math.round(data.cumulPercentage) > 100 ? `100% (+${Math.round(data.cumulPercentage) - 100}%)` : `${Math.round(data.cumulPercentage)}%`
    return (
      <div className="bg-white rounded shadow p-4">
        <div className="flex justify-between items-center mb-2">
          <span className="font-semibold text-sm">{label}</span>
          <span className="text-xs text-gray-500" title={Math.round(data.avgPercentage) !== Math.round(Math.min(100, data.avgPercentage)) ? `${Math.round(data.avgPercentage)}%` : undefined}>
            {data.successCount}/{data.total} {t('succeeded')} • {avgDisplay} {t('avg')}
          </span>
        </div>
        <div className="w-full bg-gray-200 rounded-full h-3">
          <div
            className={`h-3 rounded-full transition-all ${data.successRate === 100 ? 'bg-green-500' : data.successRate >= 50 ? 'bg-yellow-500' : 'bg-red-500'}`}
            style={{ width: `${Math.min(100, Math.max(0, data.successRate))}%` }}
          />
        </div>
        <div className="flex justify-between text-xs text-gray-400 mt-1">
          <span title={Math.round(data.cumulPercentage) !== Math.round(Math.min(100, data.cumulPercentage)) ? `${Math.round(data.cumulPercentage)}%` : undefined}>{t('cumul')}: {Number.isInteger(data.sumCurrent) ? data.sumCurrent : data.sumCurrent.toFixed(1)} / {Number.isInteger(data.sumTarget) ? data.sumTarget : data.sumTarget.toFixed(1)} ({cumulDisplay})</span>
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

      <HabitTable habits={habits} dates={dates} onToggle={async (h, d) => { await handleToggle(h, d); refreshSilent() }} onDecrement={async (h, d) => { await handleDecrement(h, d); refreshSilent() }} />
    </div>
  )
}
