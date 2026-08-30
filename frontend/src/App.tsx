import { BrowserRouter, Routes, Route, Link } from 'react-router-dom'
import { useEffect, useState } from 'react'
import MainPage from './pages/MainPage'
import HabitPage from './pages/HabitPage'
import HabitFormPage from './pages/HabitFormPage'
import AdminPage from './pages/AdminPage'
import LoginPage from './pages/LoginPage'
import OfflineBanner from './components/OfflineBanner'
import { api, clearAuthToken, getAuthToken } from './api'

function AppInner() {
  const [auth, setAuth] = useState<{ protected: boolean; authenticated: boolean } | null>(null)
  const [loading, setLoading] = useState(true)

  const checkAuth = async () => {
    try {
      const s = await api.authStatus()
      setAuth(s)
    } catch {
      // if status itself 401, it still returns JSON; if network fails, assume not protected
      setAuth({ protected: false, authenticated: true })
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    checkAuth()
    const onUnauth = () => {
      clearAuthToken()
      setAuth(prev => prev ? { ...prev, authenticated: false } : { protected: true, authenticated: false })
      checkAuth()
    }
    window.addEventListener('auth:unauthorized', onUnauth)
    return () => window.removeEventListener('auth:unauthorized', onUnauth)
  }, [])

  // clear stale token when server says not authenticated (side-effect outside render)
  useEffect(() => {
    if (auth?.protected && !auth.authenticated && getAuthToken()) {
      clearAuthToken()
    }
  }, [auth])

  const handleLogin = async () => {
    await checkAuth()
  }

  const handleLogout = async () => {
    await api.logout()
    clearAuthToken()
    // also clear any persisted remember flag? keep preference
    setAuth(prev => prev ? { ...prev, authenticated: false } : { protected: true, authenticated: false })
    // force re-check
    checkAuth()
  }

  if (loading) return <div className="py-20 text-center text-gray-400" role="status" aria-live="polite">Chargement...</div>

  // If protected and not authenticated, show login
  if (auth?.protected && !auth.authenticated) {
    return (
      <div className="min-h-screen bg-gray-50">
        <header className="bg-white shadow-sm sticky top-0 z-10">
          <div className="max-w-5xl mx-auto px-4 py-3 flex items-center justify-between">
            <Link to="/" className="font-bold text-xl text-green-600">Habit Tracker</Link>
          </div>
        </header>
        <main className="max-w-5xl mx-auto px-4 py-6">
          <LoginPage onLogin={handleLogin} />
        </main>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white shadow-sm sticky top-0 z-10">
        <div className="max-w-5xl mx-auto px-4 py-3 flex items-center justify-between gap-2">
          <Link to="/" className="font-bold text-xl text-green-600">Habit Tracker</Link>
          <div className="flex items-center gap-2">
            <Link to="/admin" className="px-3 py-1.5 rounded text-sm border bg-white hover:bg-gray-50">Admin</Link>
            <Link to="/habits/new" className="bg-green-600 text-white px-3 py-1.5 rounded text-sm hover:bg-green-700">+ Nouvelle habitude</Link>
            {auth?.protected && (
              <button onClick={handleLogout} className="border px-3 py-1.5 rounded text-sm bg-white hover:bg-gray-100" title="Se déconnecter">
                Déconnexion
              </button>
            )}
          </div>
        </div>
      </header>
      <OfflineBanner />
      <main className="max-w-5xl mx-auto px-4 py-6">
        <Routes>
          <Route path="/" element={<MainPage />} />
          <Route path="/habits/new" element={<HabitFormPage />} />
          <Route path="/habits/:id" element={<HabitPage />} />
          <Route path="/habits/:id/edit" element={<HabitFormPage />} />
          <Route path="/admin" element={<AdminPage />} />
          <Route path="*" element={<div className="py-10 text-center text-gray-500">Page non trouvée — <Link to="/" className="text-green-600 underline">Retour accueil</Link></div>} />
        </Routes>
      </main>
    </div>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <AppInner />
    </BrowserRouter>
  )
}
