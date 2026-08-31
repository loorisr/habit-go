import { useState } from 'react'
import { api, isRememberEnabled } from '../api'
import { useI18n } from '../i18n'

type Props = { onLogin: () => void }

export default function LoginPage({ onLogin }: Props) {
  const { t } = useI18n()
  const [password, setPassword] = useState('')
  const [remember, setRemember] = useState(() => isRememberEnabled())
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      await api.login(password, remember)
      onLogin()
    } catch (err: any) {
      setError(err.message || 'Mot de passe incorrect')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="max-w-md mx-auto mt-16 bg-white rounded shadow p-6">
      <h2 className="text-xl font-bold mb-2 text-center">{t('login')}</h2>
      <p className="text-sm text-gray-500 mb-4 text-center">{t('protectedInstance')}</p>
      {error && <div className="bg-red-100 text-red-700 p-2 rounded mb-3 text-sm">{error}</div>}
      <form onSubmit={submit} className="space-y-4">
        <div>
          <label className="block text-sm font-medium mb-1">{t('password')}</label>
          <input
            type="password"
            className="w-full border rounded px-3 py-2"
            value={password}
            onChange={e => setPassword(e.target.value)}
            placeholder={t('passwordPlaceholder')}
            autoFocus
          />
        </div>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={remember} onChange={e => setRemember(e.target.checked)} />
          {t('rememberPassword')}
        </label>
        <button
          type="submit"
          disabled={loading}
          className="w-full bg-green-600 text-white py-2 rounded hover:bg-green-700 disabled:opacity-50"
        >
          {loading ? t('connecting') : t('loginBtn')}
        </button>
      </form>
    </div>
  )
}
