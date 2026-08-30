import { Link } from 'react-router-dom'
import type { Habit } from '../api'
import { localDateStr } from '../api'
import { useRef, useState } from 'react'

type Props = {
  habits: Habit[]
  dates: string[]
  onToggle: (habit: Habit, date: string) => void
  onDecrement: (habit: Habit, date: string) => void
}

function CellValue({ habit, value }: { habit: Habit; value: number | undefined }) {
  if (value === undefined) return <span className="text-gray-300">-</span>
  if (habit.type === 'boolean') {
    return value >= 1 ? <span className={habit.is_negative ? 'text-red-600' : 'text-green-600'}>✓</span> : <span className="text-gray-300">-</span>
  }
  return <span className={habit.is_negative && value > habit.goal_value ? 'text-red-600' : value > 0 ? 'text-green-600' : 'text-gray-500'}>{value}</span>
}

export default function HabitTable({ habits, dates, onToggle, onDecrement }: Props) {
  const [collapsed, setCollapsed] = useState<Set<string>>(() => new Set())
  const toggleGroup = (g: string) => {
    setCollapsed(prev => {
      const next = new Set(prev)
      if (next.has(g)) next.delete(g)
      else next.add(g)
      return next
    })
  }
  // Group
  const groups = new Map<string, Habit[]>()
  for (const h of habits) {
    const g = h.group_name?.trim() || 'Sans groupe'
    if (!groups.has(g)) groups.set(g, [])
    groups.get(g)!.push(h)
  }
  const sortedGroups = Array.from(groups.entries()).sort((a, b) => a[0].localeCompare(b[0]))
  for (const [, list] of sortedGroups) list.sort((a, b) => a.name.localeCompare(b.name))

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
            <table className="w-full text-sm">
              <thead>
                <tr className="text-xs text-gray-500 border-b">
                  <th className="text-left px-4 py-2 sticky left-0 bg-white">Habitude</th>
                  {displayDates.map(d => (
                    <th key={d} className="px-3 py-2 text-center whitespace-nowrap">
                      {d === localDateStr() ? "Aujourd'hui" : d === localDateStr(new Date(Date.now() - 86400000)) ? 'Hier' : 'J-2'}
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
                        <Link to={`/habits/${habit.id}`} className="hover:underline text-green-700">{habit.name}</Link>
                        <div className="text-xs text-gray-400">
                          {habit.type === 'numerical'
                            ? `${habit.goal_value}${habit.unit ? ' ' + habit.unit : ''} ${habit.is_negative ? '≤' : '/'} ${habit.goal_period}`
                            : `${habit.goal_value} ✓ ${habit.is_negative ? '≤' : '/'} ${habit.goal_period}`}
                        </div>
                        {habit.progress && (
                          <>
                            {(habit.goal_period === 'weekly' || habit.goal_period === 'monthly' || !habit.progress.success) && (
                              <div className={`mt-1 text-xs px-2 py-0.5 rounded inline-block font-mono ${habit.progress.success ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
                                {habit.progress.current}{habit.unit && habit.type === 'numerical' ? ' ' + habit.unit : ''} {habit.is_negative ? '≤' : '/'} {habit.progress.target}{habit.unit && habit.type === 'numerical' ? ' ' + habit.unit : ''} {habit.progress.success ? '✓' : '✗'} • {Math.round(habit.progress.percentage)}%
                              </div>
                            )}
                            <div className="mt-1 w-full max-w-[180px]">
                              <div className="flex justify-between text-[10px] text-gray-500">
                                <span>{Math.round(habit.progress.percentage)}%</span>
                                <span>{habit.progress.success ? '✓' : '✗'}</span>
                              </div>
                              <div className="w-full bg-gray-200 rounded-full h-1.5">
                                <div
                                  className={`h-1.5 rounded-full ${habit.progress.success ? 'bg-green-500' : 'bg-red-500'}`}
                                  style={{ width: `${Math.min(100, Math.max(0, habit.progress.percentage))}%` }}
                                />
                              </div>
                            </div>
                          </>
                        )}
                      </td>
                      {displayDates.map(date => {
                        const val = map.get(date)
                        return (
                          <td key={date} className="px-2 py-2">
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
      {sortedGroups.length === 0 && <div className="text-center text-gray-400 py-10">Aucune habitude. Créez-en une !</div>}
    </div>
  )
}

function CellButton({ habit, date, value, onToggle, onDecrement }: { habit: Habit; date: string; value: number | undefined; onToggle: (h: Habit, d: string) => void; onDecrement: (h: Habit, d: string) => void }) {
  const timerRef = useRef<number | null>(null)
  const longPressTriggered = useRef(false)

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

  return (
    <button
      onClick={onClick}
      onContextMenu={handleContextMenu}
      onTouchStart={onPointerDown}
      onTouchEnd={onPointerUp}
      onMouseDown={onPointerDown}
      onMouseUp={onPointerUp}
      onMouseLeave={onPointerUp}
      title="Clic +1 / toggle, appui long -1"
      className="w-14 h-9 rounded border flex items-center justify-center hover:bg-gray-100 active:bg-gray-200 select-none"
    >
      <CellValue habit={habit} value={value} />
    </button>
  )
}
