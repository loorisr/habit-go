import { describe, it, expect } from 'vitest'
import { localDateStr, lastNDates } from './api'

describe('localDateStr', () => {
  it('formats a local date as YYYY-MM-DD', () => {
    expect(localDateStr(new Date(2025, 0, 5, 23, 59))).toBe('2025-01-05')
    expect(localDateStr(new Date(2025, 11, 31, 0, 0))).toBe('2025-12-31')
  })
})

describe('lastNDates', () => {
  it('returns n consecutive days, oldest first, ending on base', () => {
    expect(lastNDates(3, new Date(2025, 2, 1, 10))).toEqual(['2025-02-27', '2025-02-28', '2025-03-01'])
  })

  it('stays correct across DST changes (midday anchor)', () => {
    // 2025-03-30 is the start of DST in Europe
    expect(lastNDates(3, new Date(2025, 2, 31, 10))).toEqual(['2025-03-29', '2025-03-30', '2025-03-31'])
  })
})
