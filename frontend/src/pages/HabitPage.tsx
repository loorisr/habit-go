import { useEffect, useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { api, Habit, Entry } from '../api'
import Calendar from '../components/Calendar'
import { useI18n } from '../i18n'

export default function HabitPage() {
  const { t } = useI18n()
  const { id } = useParams<{ id: string }>()
  const nav = useNavigate()
  const [habit, setHabit] = useState<Habit | null>(null)
  const [entries, setEntries] = useState<Entry[]>([])
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
        // wide range to cover calendar navigation (60d back, 30d forward) without double fetch race
        const from = new Date(); from.setDate(from.getDate() - 60)
        const to = new Date(); to.setDate(to.getDate() + 30)
        const es = await api.getEntries(id, from.toLocaleDateString('en-CA'), to.toLocaleDateString('en-CA'))
        if (!cancelled) setEntries(es)
      } catch (e) {
        console.error(e)
      }
    }
    load()
    return () => { cancelled = true }
  }, [id])

  const onEdit = (date: string, current?: number) => {
    setEditing({ date, value: current })
    setEditVal(current !== undefined ? String(current) : habit?.type === 'boolean' ? '1' : '0')
  }

  const save = async () => {
    if (!id || !editing) return
    const v = parseFloat(editVal)
    if (isNaN(v)) return
    await api.upsertEntry(id, editing.date, v)
    setEditing(null)
    // refresh
    const es = await api.getEntries(id)
    setEntries(es)
    // also reload habit progress
    const h = await api.getHabit(id)
    setHabit(h)
  }

  const clearEntry = async () => {
    if (!id || !editing) return
    await api.deleteEntry(id, editing.date)
    setEditing(null)
    const es = await api.getEntries(id)
    setEntries(es)
    const h = await api.getHabit(id)
    setHabit(h)
  }

  const archive = async () => {
    if (!id || !habit) return
    if (habit.archived_at) await api.restoreHabit(id)
    else await api.archiveHabit(id)
    const h = await api.getHabit(id)
    setHabit(h)
  }

  const hardDelete = async () => {
    if (!id) return
    if (!confirm(t('confirmDelete'))) return
    await api.deleteHabit(id)
    nav('/')
  }

  if (!habit) return <div className="py-10 text-center">{t('loading')}</div>

  return (
    <div className="space-y-6">
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
              const displayPct = p > 100 ? `100% (+${p - 100}%)` : p < 0 ? `0%` : `${p}%`
              return (
                <div className="mt-2 space-y-1">
                  <div title={p !== Math.round(Math.min(100, Math.max(0, habit.progress.percentage))) ? `${p}%` : undefined} className={`text-sm px-2 py-1 rounded inline-block ${habit.progress.success ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
                    {t('progress')}: {habit.progress.current}{habit.unit && habit.type === 'numerical' ? ' ' + habit.unit : ''} {habit.is_negative ? '≤' : '/'} {habit.progress.target}{habit.unit && habit.type === 'numerical' ? ' ' + habit.unit : ''} • {displayPct} {habit.progress.success ? '✓' : '✗'} ({t(habit.progress.period) || habit.progress.period})
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
          <div className="bg-white rounded shadow p-4 w-full max-w-sm">
            <h3 className="font-semibold mb-2">{t('editDate')} {editing.date}</h3>
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
