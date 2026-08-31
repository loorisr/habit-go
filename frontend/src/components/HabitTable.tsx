import { Link } from 'react-router-dom'
import type { Habit } from '../api'
import { localDateStr } from '../api'
import { useRef, useState, useMemo } from 'react'
import { useI18n } from '../i18n'
import { getHabitIntensity, formatProgressPct } from '../lib/habitIntensity'

type Props = {
  habits: Habit[]
  dates: string[]
  onToggle: (habit: Habit, date: string) => void
  onDecrement: (habit: Habit, date: string) => void
}

function CellValue({ habit, value }: { habit: Habit; value: number | undefined }) {
  if (value === undefined) return <span>-</span>
  if (habit.type === 'boolean') {
    return value >= 1 ? <span>✓</span> : <span>-</span>
  }
  return <span>{value}</span>
}

export default function HabitTable({ habits, dates, onToggle, onDecrement }: Props) {
  const { t } = useI18n()
  const [collapsed, setCollapsed] = useState<Set<string>>(() => new Set())
  const toggleGroup = (g: string) => {
    setCollapsed(prev => {
      const next = new Set(prev)
      if (next.has(g)) next.delete(g)
      else next.add(g)
      return next
    })
  }
  // Group — memoized (perf: avoids recompute on every render)
  const sortedGroups = useMemo(() => {
    const groups = new Map<string, Habit[]>()
    for (const h of habits) {
      const g = h.group_name?.trim() || t('noGroup')
      if (!groups.has(g)) groups.set(g, [])
      groups.get(g)!.push(h)
    }
    const sorted = Array.from(groups.entries()).sort((a, b) => a[0].localeCompare(b[0]))
    for (const [, list] of sorted) list.sort((a, b) => a.name.localeCompare(b.name))
    return sorted
  }, [habits, t])

  // Actually dates = [J-2, Hier, Aujourd'hui] with lastNDates 3 gives ordered oldest->newest.
  // We'll display as Aujourd'hui | Hier | J-2 but simpler display in order of dates reversed? Spec: Habit | Aujourd'hui | Hier | J-2
  // So we reorder dates reversed for columns
  const displayDates = [...dates].reverse() // today first

  return (
    <div className="space-y-6">
      {sortedGroups.map(([groupName, list]) => (
        <div key={groupName} className="bg-white rounded shadow overflow-hidden">
          <button onClick={() => toggleGroup(groupName)} className="w-full bg-gray-100 px-4 py-2 font-semibold text-sm flex items-center justify-between hover:bg-gray-200 text-left">
            <span>{groupName} <span className="font-normal text-gray-500">({list.length})</span></span>
            <span className="text-gray-500">{collapsed.has(groupName) ? '▸' : '▾'}</span>
          </button>
          {!collapsed.has(groupName) && <div className="overflow-x-auto">
            <table className="w-full text-sm table-fixed" role="grid" aria-label={groupName}>
              <colgroup>
                <col />
                <col style={{ width: '88px' }} />
                <col style={{ width: '88px' }} />
                <col style={{ width: '88px' }} />
              </colgroup>
              <thead>
                <tr className="text-xs text-gray-500 border-b" role="row">
                  <th className="text-left px-4 py-2 sticky left-0 bg-white" scope="col">{t('habit')}</th>
                  {displayDates.map(d => (
                    <th key={d} className="px-2 py-2 text-center whitespace-nowrap w-[88px] min-w-[88px]">
                      {d === localDateStr() ? t('today') : d === localDateStr(new Date(Date.now() - 86400000)) ? t('yesterday') : t('dayMinus2')}
                      <div className="font-normal text-[11px]">{d}</div>
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {list.map(habit => {
                  const map = new Map(habit.recent_entries?.map(e => [e.date, e.value]))
                  return (
                    <tr key={habit.id} className="border-b hover:bg-gray-50">
                      <td className="px-4 py-2 font-medium sticky left-0 bg-white">
                        <Link to={`/habits/${habit.id}`} className="hover:underline text-green-700 block">{habit.name}</Link>
                        {habit.progress && (() => {
                          const pct = formatProgressPct(habit.progress.percentage)
                          const showOverflow = pct.actual !== Math.round(habit.progress.percentage) || pct.actual > 100 || pct.actual < 0
                          return (
                            <div title={showOverflow ? `${Math.round(habit.progress.percentage)}%` : undefined} className={`mt-1.5 text-xs px-2 py-0.5 rounded inline-block font-mono ${habit.progress.success ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
                              {habit.progress.current}{habit.unit && habit.type === 'numerical' ? ' ' + habit.unit : ''} {habit.is_negative ? '≤' : '/'} {habit.progress.target}{habit.unit && habit.type === 'numerical' ? ' ' + habit.unit : ''} {habit.progress.success ? '✓' : '✗'} • {pct.display}
                            </div>
                          )
                        })()}
                      </td>
                      {displayDates.map(date => {
                        const val = map.get(date)
                        return (
                          <td key={date} className="px-2 py-2 w-[88px] min-w-[88px]">
                            <div className="flex justify-center">
                              <CellButton habit={habit} date={date} value={val} onToggle={onToggle} onDecrement={onDecrement} />
                            </div>
                          </td>
                        )
                      })}
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>}
        </div>
      ))}
      {sortedGroups.length === 0 && <div className="text-center text-gray-400 py-10">{t('noHabit')}</div>}
    </div>
  )
}

function CellButton({ habit, date, value, onToggle, onDecrement }: { habit: Habit; date: string; value: number | undefined; onToggle: (h: Habit, d: string) => void; onDecrement: (h: Habit, d: string) => void }) {
  const { t } = useI18n()
  const timerRef = useRef<number | null>(null)
  const longPressTriggered = useRef(false)
  const todayStr = localDateStr(new Date())
  const isToday = date === todayStr

  const handleContextMenu = (e: React.MouseEvent) => {
    e.preventDefault()
    onDecrement(habit, date)
  }

  const onPointerDown = () => {
    longPressTriggered.current = false
    timerRef.current = window.setTimeout(() => {
      longPressTriggered.current = true
      onDecrement(habit, date)
    }, 500)
  }
  const onPointerUp = () => {
    if (timerRef.current) clearTimeout(timerRef.current)
  }
  const onClick = () => {
    if (longPressTriggered.current) return
    onToggle(habit, date)
  }

  const ariaLabel = `${habit.name} ${date}: ${value !== undefined ? (habit.type === 'boolean' ? (value >= 1 ? '✓' : '-') : String(value)) : t('noHabit').includes('Aucune') ? 'vide' : 'empty'}`
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      onToggle(habit, date)
    } else if (e.key === '-' || e.key === '_' || e.key === 'ArrowDown') {
      e.preventDefault()
      onDecrement(habit, date)
    } else if (e.key === '+' || e.key === '=' || e.key === 'ArrowUp') {
      e.preventDefault()
      onToggle(habit, date)
    }
  }

  return (
    <button
      onClick={onClick}
      onContextMenu={handleContextMenu}
      onTouchStart={onPointerDown}
      onTouchEnd={onPointerUp}
      onMouseDown={onPointerDown}
      onMouseUp={onPointerUp}
      onMouseLeave={onPointerUp}
      onKeyDown={handleKeyDown}
      title={t('cellHint')}
      aria-label={ariaLabel}
      aria-pressed={habit.type === 'boolean' ? (value !== undefined && value >= 1) : undefined}
      role="gridcell"
      tabIndex={0}
      className={`w-14 h-9 rounded border flex items-center justify-center select-none font-medium focus:outline-none focus-visible:ring-2 focus-visible:ring-green-500 focus-visible:ring-offset-1 ${getHabitIntensity(habit, value)} ${isToday ? 'ring-2 ring-green-400 ring-offset-1' : ''}`}
    >
      <CellValue habit={habit} value={value} />
    </button>
  )
}
