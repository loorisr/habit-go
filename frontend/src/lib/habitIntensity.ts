import type { Habit } from '../api'

/**
 * Shared intensity helper — used by Calendar (detail) and HabitTable (main page).
 * Mirrors backend progress logic but for single-day cell values.
 * Returns Tailwind classes for background/border/hover.
 */
export function getHabitIntensity(habit: Habit, value: number | undefined): string {
  if (value === undefined) return 'bg-gray-100 text-gray-300 hover:bg-gray-200 border-gray-200'
  if (habit.is_negative) {
    if (habit.type === 'boolean') {
      return value >= 1 ? 'bg-red-500 text-white hover:bg-red-600 border-red-600' : 'bg-green-200 text-green-900 hover:bg-green-300 border-green-300'
    }
    if (value <= habit.goal_value) return 'bg-green-200 text-green-900 hover:bg-green-300 border-green-300'
    const ratio = Math.min(value / habit.goal_value, 2)
    if (ratio > 1.5) return 'bg-red-500 text-white hover:bg-red-600 border-red-600'
    return 'bg-red-300 text-red-900 hover:bg-red-400 border-red-400'
  } else {
    if (habit.type === 'boolean') {
      return value >= 1 ? 'bg-green-500 text-white hover:bg-green-600 border-green-600' : 'bg-gray-100 text-gray-300 hover:bg-gray-200 border-gray-200'
    }
    const ratio = Math.min(value / habit.goal_value, 1)
    if (ratio >= 1) return 'bg-green-500 text-white hover:bg-green-600 border-green-600'
    if (ratio >= 0.66) return 'bg-green-300 text-green-900 hover:bg-green-400 border-green-400'
    if (ratio >= 0.33) return 'bg-green-200 text-green-900 hover:bg-green-300 border-green-300'
    if (ratio > 0) return 'bg-green-100 text-green-800 hover:bg-green-200 border-green-200'
    return 'bg-gray-100 text-gray-300 hover:bg-gray-200 border-gray-200'
  }
}

export function formatProgressPct(p: number): { display: string; actual: number } {
  const r = Math.round(p)
  if (r > 100) return { display: `100% (+${r - 100}%)`, actual: r }
  if (r < 0) return { display: `0%`, actual: r }
  return { display: `${r}%`, actual: r }
}
