import { useState } from 'react'
import type { Entry, Habit } from '../api'
import { localDateStr } from '../api'

type Props = {
  habit: Habit
  entries: Entry[]
  onEdit: (date: string, current?: number) => void
}

function intensityClass(habit: Habit, value: number | undefined): string {
  if (value === undefined) return 'bg-gray-100'
  if (habit.is_negative) {
    // success = value <= goal => light green if success, red if exceed
    if (value <= habit.goal_value) return 'bg-green-200 hover:bg-green-300'
    const ratio = Math.min(value / habit.goal_value, 2)
    if (ratio > 1.5) return 'bg-red-500 text-white'
    return 'bg-red-300'
  } else {
    if (habit.type === 'boolean') {
      return value >= 1 ? 'bg-green-500 text-white' : 'bg-gray-100'
    }
    const ratio = Math.min(value / habit.goal_value, 1)
    if (ratio >= 1) return 'bg-green-500 text-white'
    if (ratio >= 0.66) return 'bg-green-300'
    if (ratio >= 0.33) return 'bg-green-200'
    if (ratio > 0) return 'bg-green-100'
    return 'bg-gray-100'
  }
}

export default function Calendar({ habit, entries, onEdit }: Props) {
  const [cur, setCur] = useState(() => {
    const d = new Date()
    d.setDate(1)
    return d
  })
  const year = cur.getFullYear()
  const month = cur.getMonth()
  const first = new Date(year, month, 1)
  const last = new Date(year, month + 1, 0)
  const startWeekDay = (first.getDay() + 6) % 7 // Monday 0
  const daysInMonth = last.getDate()

  const map = new Map(entries.map(e => [e.date, e.value]))

  const cells: (string | null)[] = []
  for (let i = 0; i < startWeekDay; i++) cells.push(null)
  for (let d = 1; d <= daysInMonth; d++) {
    const dt = new Date(year, month, d)
    cells.push(localDateStr(dt))
  }

  const todayStr = localDateStr(new Date())

  return (
    <div className="bg-white rounded shadow p-4">
      <div className="flex items-center justify-between mb-3">
        <button onClick={() => setCur(new Date(year, month - 1, 1))} className="px-2 py-1 border rounded">‹</button>
        <div className="font-semibold capitalize">{cur.toLocaleDateString('fr-FR', { month: 'long', year: 'numeric' })}</div>
        <button onClick={() => setCur(new Date(year, month + 1, 1))} className="px-2 py-1 border rounded">›</button>
      </div>
      <button onClick={() => { const d=new Date(); d.setDate(1); setCur(d) }} className="text-xs text-green-600 mb-2">Aujourd'hui</button>
      <div className="grid grid-cols-7 gap-1 text-xs">
        {['Lun','Mar','Mer','Jeu','Ven','Sam','Dim'].map(d => <div key={d} className="text-center text-gray-400 py-1">{d}</div>)}
        {cells.map((date, idx) => {
          if (date === null) return <div key={idx} />
          const val = map.get(date)
          const isToday = date === todayStr
          return (
            <button
              key={date}
              onClick={() => onEdit(date, val)}
              title={val !== undefined ? `${date}: ${val}` : date}
              className={`h-10 rounded flex flex-col items-center justify-center border ${intensityClass(habit, val)} ${isToday ? 'ring-2 ring-green-400' : ''}`}
            >
              <span className="text-[11px]">{date.slice(8)}</span>
              {val !== undefined && <span className="text-[10px] font-semibold">{habit.type==='boolean' ? (val>=1?'✓':'-') : val}</span>}
            </button>
          )
        })}
      </div>
    </div>
  )
}
