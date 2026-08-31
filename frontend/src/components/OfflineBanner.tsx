import { useEffect, useState } from 'react'
import { useI18n } from '../i18n'

export default function OfflineBanner() {
  const { t } = useI18n()
  const [online, setOnline] = useState(navigator.onLine)
  useEffect(() => {
    const on = () => setOnline(true)
    const off = () => setOnline(false)
    window.addEventListener('online', on)
    window.addEventListener('offline', off)
    return () => { window.removeEventListener('online', on); window.removeEventListener('offline', off) }
  }, [])
  if (online) return null
  return <div className="bg-yellow-400 text-center text-sm py-1">{t('offline')}</div>
}
