import { describe, it, expect } from 'vitest'
import { getHabitIntensity, formatNum } from './habitIntensity'
import type { Habit } from '../api'

const base: Habit = {
  id: 'h', name: 'h', group_name: '', type: 'numerical', goal_value: 4, goal_period: 'daily',
  is_negative: false, unit: '', archived_at: null, created_at: '', updated_at: '',
}

describe('getHabitIntensity', () => {
  it('shows empty cells for no value', () => {
    expect(getHabitIntensity(base, undefined)).toContain('bg-gray-100')
  })

  it('grades positive numerical habits by ratio to the goal', () => {
    expect(getHabitIntensity(base, 4)).toContain('bg-green-500')
    expect(getHabitIntensity(base, 1)).toContain('bg-green-100')
  })

  it('negative boolean: a slip is red, no slip is green', () => {
    const neg = { ...base, type: 'boolean' as const, is_negative: true, goal_value: 0 }
    expect(getHabitIntensity(neg, 0)).toContain('bg-green-200')
    expect(getHabitIntensity(neg, 1)).toContain('bg-red-500')
  })

  it('negative habit with goal 0 does not divide by zero', () => {
    const neg = { ...base, is_negative: true, goal_value: 0 }
    expect(getHabitIntensity(neg, 2)).toContain('bg-red-500')
  })
})

describe('formatNum', () => {
  it('removes floating point noise', () => {
    expect(formatNum(3.3000000000000003)).toBe('3.3')
    expect(formatNum(2)).toBe('2')
    expect(formatNum(0.125)).toBe('0.13')
  })
})
