import { useEffect, useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { api, ApiError, Habit, Entry } from '../api'
import Calendar from '../components/Calendar'
import { useI18n } from '../i18n'
import { formatNum } from '../lib/habitIntensity'

export default function HabitPage() {
  const { t } = useI18n()
  const { id } = useParams<{ id: string }>()
  const nav = useNavigate()
  const [habit, setHabit] = useState<Habit | null>(null)
  const [entries, setEntries] = useState<Entry[]>([])
  const [loadError, setLoadError] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [editing, setEditing] = useState<{ date: string; value: number | undefined } | null>(null)
  const [editVal, setEditVal] = useState<string>('')

  useEffect(() => {
    if (!id) return
    let cancelled = false
    const load = async () => {
      try {
        const h = await api.getHabit(id)
        if (cancelled) return
        setHabit(h)
        // A6: all entries, so the calendar shows and protects every month
        const es = await api.getEntries(id)
        if (!cancelled) setEntries(es)
      } catch (e: unknown) {
        if (cancelled) return
        // C2: show the failure instead of "loading" forever
        setLoadError(e instanceof ApiError && e.status === 404 ? t('habitNotFound') : t('networkError'))
      }
    }
    load()
    return () => { cancelled = true }
  }, [id])

  // Escape closes the edit dialog (C9)
  useEffect(() => {
    if (!editing) return
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') setEditing(null) }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [editing])

  const reload = async () => {
    if (!id) return
    setEntries(await api.getEntries(id))
    setHabit(await api.getHabit(id))
  }

  // Runs an action; on failure shows a message and returns false (C2).
  const run = async (fn: () => Promise<void>): Promise<boolean> => {
    setActionError(null)
    try {
      await fn()
      return true
    } catch {
      setActionError(t('actionFailed'))
      return false
    }
  }

  const onEdit = (date: string, current?: number) => {
    setEditing({ date, value: current })
    setEditVal(current !== undefined ? String(current) : habit?.type === 'boolean' ? '1' : '0')
  }

  const save = async () => {
    if (!id || !editing) return
    const v = parseFloat(editVal)
    if (isNaN(v) || v < 0) { setActionError(t('actionFailed')); return }
    const ok = await run(async () => {
      await api.upsertEntry(id, editing.date, v)
      await reload()
    })
    if (ok) setEditing(null)
  }

  const clearEntry = async () => {
    if (!id || !editing) return
    const ok = await run(async () => {
      await api.deleteEntry(id, editing.date)
      await reload()
    })
    if (ok) setEditing(null)
  }

  const archive = async () => {
    if (!id || !habit) return
    await run(async () => {
      if (habit.archived_at) await api.restoreHabit(id)
      else await api.archiveHabit(id)
      setHabit(await api.getHabit(id))
    })
  }

  const hardDelete = async () => {
    if (!id) return
    if (!confirm(t('confirmDelete'))) return
    const ok = await run(() => api.deleteHabit(id))
    if (ok) nav('/')
  }

  if (loadError) {
    return (
      <div className="py-10 text-center text-red-500" role="alert">
        {loadError} <Link to="/" className="text-green-600 underline">{t('backHome')}</Link>
      </div>
    )
  }
  if (!habit) return <div className="py-10 text-center">{t('loading')}</div>

  return (
    <div className="space-y-6">
      {actionError && !editing && <div role="alert" className="bg-red-100 text-red-700 p-2 rounded text-sm">{actionError}</div>}
      <div className="bg-white rounded shadow p-4">
        <div className="flex items-start justify-between">
          <div>
            <h2 className="text-xl font-bold">{habit.name}</h2>
            <div className="text-sm text-gray-500">
              {habit.group_name || t('noGroup')} • {t(habit.type) === habit.type ? habit.type : t(habit.type)}
              {habit.type === 'numerical' && habit.unit ? ` • ${t('objective')} ${habit.goal_value} ${habit.unit} ${habit.is_negative ? '≤' : '/'} ${t(habit.goal_period)}` : ` • ${t('objective')} ${habit.goal_value} ${habit.is_negative ? '≤' : '/'} ${t(habit.goal_period)}`}
              {habit.is_negative ? ' (max)' : ''}
              {habit.type === 'numerical' && habit.unit && ` • ${t('unitLabel')} ${habit.unit}`}
            </div>
            {habit.progress && (() => {
              const p = Math.round(habit.progress.percentage)
              const displayPct = p < 0 ? `0%` : `${p}%`
              const unit = habit.unit && habit.type === 'numerical' ? ' ' + habit.unit : ''
              return (
                <div className="mt-2 space-y-1">
                  <div className={`text-sm px-2 py-1 rounded inline-block ${habit.progress.success ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
                    {t('progress')}: {formatNum(habit.progress.current)}{unit} {habit.is_negative ? '≤' : '/'} {formatNum(habit.progress.target)}{unit} • {displayPct} {habit.progress.success ? '✓' : '✗'} ({t(habit.progress.period) || habit.progress.period})
                  </div>
                  <div className="w-full max-w-xs bg-gray-200 rounded-full h-2">
                    <div className={`h-2 rounded-full ${habit.progress.success ? 'bg-green-500' : 'bg-red-500'}`} style={{ width: `${Math.min(100, Math.max(0, habit.progress.percentage))}%` }} />
                  </div>
                </div>
              )
            })()}
            {habit.archived_at && <div className="text-xs text-orange-600 mt-1">{t('archived')}</div>}
          </div>
          <div className="flex gap-2">
            <Link to={`/habits/${habit.id}/edit`} className="border px-3 py-1 rounded text-sm bg-white">{t('edit')}</Link>
            <button onClick={archive} className="border px-3 py-1 rounded text-sm bg-white">{habit.archived_at ? t('restore') : t('archive')}</button>
            <button onClick={hardDelete} className="border px-3 py-1 rounded text-sm bg-red-50 text-red-600">{t('delete')}</button>
          </div>
        </div>
      </div>

      <Calendar habit={habit} entries={entries} onEdit={onEdit} />

      {editing && (
        <div className="fixed inset-0 bg-black/30 flex items-center justify-center p-4 z-20">
          <div role="dialog" aria-modal="true" aria-labelledby="edit-entry-title" className="bg-white rounded shadow p-4 w-full max-w-sm">
            <h3 id="edit-entry-title" className="font-semibold mb-2">{t('editDate')} {editing.date}</h3>
            {habit.type === 'boolean' ? (
              <div className="space-y-3">
                <label className="flex items-center gap-2">
                  <input type="checkbox" checked={editVal === '1' || editVal === 'true'} onChange={e => setEditVal(e.target.checked ? '1' : '0')} />
                  {t('done')}
                </label>
              </div>
            ) : (
              <div className="flex items-center gap-2">
                <button onClick={() => setEditVal(String(Math.max(0, (parseFloat(editVal) || 0) - 1)))} className="border px-3 py-1 rounded">-</button>
                <input type="number" step="any" value={editVal} onChange={e => setEditVal(e.target.value)} className="flex-1 border rounded px-3 py-2" />
                <button onClick={() => setEditVal(String((parseFloat(editVal) || 0) + 1))} className="border px-3 py-1 rounded">+</button>
                {habit.unit && <span className="text-sm text-gray-500">{habit.unit}</span>}
              </div>
            )}
            {actionError && <div role="alert" className="bg-red-100 text-red-700 p-2 rounded text-sm mt-3">{actionError}</div>}
            <div className="flex gap-2 mt-4">
              <button onClick={save} className="bg-green-600 text-white px-3 py-1 rounded">{t('save')}</button>
              <button onClick={clearEntry} className="border px-3 py-1 rounded">{t('clear')}</button>
              <button onClick={() => setEditing(null)} className="border px-3 py-1 rounded ml-auto">{t('close')}</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
