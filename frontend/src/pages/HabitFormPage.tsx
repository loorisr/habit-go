import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../api'
import { useI18n } from '../i18n'

export default function HabitFormPage() {
  const { t } = useI18n()
  const { id } = useParams()
  const isEdit = !!id
  const nav = useNavigate()
  const [form, setForm] = useState({ name: '', group_name: '', type: 'boolean' as 'boolean' | 'numerical', goal_value: 1, goal_period: 'daily' as 'daily' | 'weekly' | 'monthly', is_negative: false, unit: '' })
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(isEdit)
  const [groups, setGroups] = useState<string[]>([])

  useEffect(() => {
    api.listHabits(false).then(habits => {
      const g = Array.from(new Set(habits.map(h => h.group_name).filter(Boolean))).sort()
      setGroups(g)
    }).catch(()=>{})
    if (id) {
      api.getHabit(id).then(h => {
        setForm({ name: h.name, group_name: h.group_name, type: h.type, goal_value: h.goal_value, goal_period: h.goal_period, is_negative: h.is_negative, unit: h.unit || '' })
        setLoading(false)
      }).catch(e => { setError(e.message); setLoading(false) })
    }
  }, [id])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    if (!form.name.trim()) { setError(t('nameRequired')); return }
    if (form.goal_value <= 0) { setError(t('goalGt0')); return }
    try {
      if (isEdit && id) await api.updateHabit(id, form as any)
      else await api.createHabit(form as any)
      nav('/')
    } catch (e: any) { setError(e.message) }
  }

  if (loading) return <div className="py-10 text-center">{t('loading')}</div>

  return (
    <div className="max-w-lg mx-auto bg-white rounded shadow p-6">
      <h2 className="font-bold text-lg mb-4">{isEdit ? t('editHabit') : t('createHabit')}</h2>
      {error && <div className="bg-red-100 text-red-700 p-2 rounded mb-3 text-sm">{error}</div>}
      <form onSubmit={submit} className="space-y-4">
        <div>
          <label className="block text-sm font-medium">{t('name')} {t('required')}</label>
          <input className="w-full border rounded px-3 py-2" value={form.name} onChange={e => setForm({ ...form, name: e.target.value })} />
        </div>
        <div>
          <label className="block text-sm font-medium">{t('groupLabel')}</label>
          <input list="groups" className="w-full border rounded px-3 py-2" value={form.group_name} onChange={e => setForm({ ...form, group_name: e.target.value })} placeholder={t('noGroupPlaceholder')} />
          <datalist id="groups">
            {groups.map(g => <option key={g} value={g} />)}
          </datalist>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium">{t('typeLabel')}</label>
            <select className="w-full border rounded px-3 py-2" value={form.type} onChange={e => setForm({ ...form, type: e.target.value as any })}>
              <option value="boolean">{t('boolean')}</option>
              <option value="numerical">{t('numerical')}</option>
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium">{t('goal')}</label>
            <input type="number" step="any" className="w-full border rounded px-3 py-2" value={form.goal_value} onChange={e => setForm({ ...form, goal_value: parseFloat(e.target.value) || 0 })} />
          </div>
        </div>
        <div>
          <label className="block text-sm font-medium">{t('period')}</label>
          <select className="w-full border rounded px-3 py-2" value={form.goal_period} onChange={e => setForm({ ...form, goal_period: e.target.value as any })}>
            <option value="daily">{t('dailyLong')}</option>
            <option value="weekly">{t('weeklyLong')}</option>
            <option value="monthly">{t('monthlyLong')}</option>
          </select>
        </div>
        {form.type === 'numerical' && (
          <div>
            <label className="block text-sm font-medium">{t('unitOptional')}</label>
            <input className="w-full border rounded px-3 py-2" value={form.unit} onChange={e => setForm({ ...form, unit: e.target.value })} placeholder={t('unitPlaceholder')} />
          </div>
        )}
        <label className="flex items-center gap-2">
          <input type="checkbox" checked={form.is_negative} onChange={e => setForm({ ...form, is_negative: e.target.checked })} />
          <span className="text-sm">{t('negativeHabit')}</span>
        </label>
        <div className="flex gap-2">
          <button type="submit" className="bg-green-600 text-white px-4 py-2 rounded hover:bg-green-700">{isEdit ? t('save') : t('create')}</button>
          <button type="button" onClick={() => nav(-1)} className="border px-4 py-2 rounded">{t('cancel')}</button>
        </div>
      </form>
    </div>
  )
}
