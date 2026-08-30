import { useEffect, useState, useCallback, useMemo } from 'react'
import { api, Habit, lastNDates } from '../api'
import HabitTable from '../components/HabitTable'

export default function MainPage() {
  const [habits, setHabits] = useState<Habit[]>([])
  const [includeArchived, setIncludeArchived] = useState(false)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const dates = useMemo(() => lastNDates(3), [])

  const fetchHabits = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await api.listHabits(includeArchived, 3)
      setHabits(data)
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }, [includeArchived])

  useEffect(() => { fetchHabits() }, [fetchHabits])

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

  // silent refresh for progress update after optimistic mutation (no loading spinner)
  const refreshSilent = useCallback(async () => {
    try {
      const data = await api.listHabits(includeArchived, 3)
      setHabits(data)
    } catch {}
  }, [includeArchived])

  if (loading) return <div className="py-10 text-center text-gray-400">Chargement...</div>
  if (error) return <div className="py-10 text-center text-red-500">{error}</div>

  // global progression (cumulated of all habits)
  const global = (() => {
    if (habits.length === 0) return null
    const withProgress = habits.filter(h => h.progress)
    if (withProgress.length === 0) return null
    const successCount = withProgress.filter(h => h.progress!.success).length
    const total = withProgress.length
    const successRate = (successCount / total) * 100
    const avgPercentage = withProgress.reduce((s, h) => s + (h.progress!.percentage || 0), 0) / total
    const sumCurrent = withProgress.reduce((s, h) => s + h.progress!.current, 0)
    const sumTarget = withProgress.reduce((s, h) => s + h.progress!.target, 0)
    const cumulPercentage = sumTarget > 0 ? (sumCurrent / sumTarget) * 100 : 0
    return { successCount, total, successRate, avgPercentage, sumCurrent, sumTarget, cumulPercentage }
  })()

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap gap-2 items-center text-sm">
        <label className="flex items-center gap-1">
          <input type="checkbox" checked={includeArchived} onChange={e => setIncludeArchived(e.target.checked)} />
          Voir archivées
        </label>
        <span className="ml-auto text-xs text-gray-400">Astuce: Clic +1 / toggle, appui long ou clic droit -1</span>
      </div>

      {global && (
        <div className="bg-white rounded shadow p-4">
          <div className="flex justify-between items-center mb-2">
            <span className="font-semibold text-sm">Progression globale (cumulée)</span>
            <span className="text-xs text-gray-500">
              {global.successCount}/{global.total} réussies • {Math.round(global.avgPercentage)}% moyen
            </span>
          </div>
          <div className="w-full bg-gray-200 rounded-full h-3">
            <div
              className={`h-3 rounded-full transition-all ${global.successRate === 100 ? 'bg-green-500' : global.successRate >= 50 ? 'bg-yellow-500' : 'bg-red-500'}`}
              style={{ width: `${Math.min(100, Math.max(0, global.successRate))}%` }}
            />
          </div>
          <div className="flex justify-between text-xs text-gray-400 mt-1">
            <span>Cumul: {Number.isInteger(global.sumCurrent) ? global.sumCurrent : global.sumCurrent.toFixed(1)} / {Number.isInteger(global.sumTarget) ? global.sumTarget : global.sumTarget.toFixed(1)} ({Math.round(global.cumulPercentage)}%)</span>
            <span>{global.successRate === 100 ? '✓ Toutes réussies' : global.successRate >= 50 ? 'En cours' : 'À améliorer'}</span>
          </div>
        </div>
      )}

      <HabitTable habits={habits} dates={dates} onToggle={async (h, d) => { await handleToggle(h, d); refreshSilent() }} onDecrement={async (h, d) => { await handleDecrement(h, d); refreshSilent() }} />
    </div>
  )
}
