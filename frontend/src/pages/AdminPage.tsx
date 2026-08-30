import { useState } from 'react'
import { api } from '../api'

export default function AdminPage() {
  const [msg, setMsg] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const handleExport = async (type: 'habits' | 'entries') => {
    setMsg(null)
    setError(null)
    try {
      if (type === 'habits') await api.exportHabits()
      else await api.exportEntries()
    } catch (err: any) {
      setError(err.message || 'Erreur export')
    }
  }

  const handleImport = async (e: React.ChangeEvent<HTMLInputElement>, kind: 'habits' | 'entries') => {
    const file = e.target.files?.[0]
    const target = e.currentTarget
    if (!file) return
    if (file.size > 10 * 1024 * 1024) {
      setError('Fichier trop volumineux (max 10 MB)')
      target.value = ''
      return
    }
    if (!file.name.endsWith('.csv') && file.type !== 'text/csv') {
      // still allow but warn
    }
    setMsg(null)
    setError(null)
    try {
      const res = kind === 'habits' ? await api.importHabits(file) : await api.importEntries(file)
      const errs = (res as any)?.errors
      const hasErrors = Array.isArray(errs) ? errs.length > 0 : !!errs
      if (hasErrors) {
        const errStr = Array.isArray(errs) ? errs.join('; ') : String(errs)
        setMsg(`Import ${kind}: ${res.imported} importés. Erreurs: ${errStr}`)
      } else {
        setMsg(`Import ${kind}: ${res.imported} importés avec succès.`)
      }
    } catch (err: any) {
      setError(err.message || 'Erreur import')
    } finally {
      target.value = ''
    }
  }

  return (
    <div className="max-w-3xl mx-auto space-y-6">
      <h2 className="text-xl font-bold">Administration</h2>

      {msg && <div className="bg-green-50 text-green-700 p-3 rounded text-sm">{msg}</div>}
      {error && <div className="bg-red-100 text-red-700 p-3 rounded text-sm">{error}</div>}

      <div className="bg-white rounded shadow p-4 space-y-4">
        <h3 className="font-semibold">Export</h3>
        <p className="text-sm text-gray-500">Téléchargez les données au format CSV (compatible ré-import).</p>
        <div className="flex gap-2">
          <button onClick={() => handleExport('habits')} className="border px-3 py-2 rounded bg-white hover:bg-gray-50 text-sm">Export habits.csv</button>
          <button onClick={() => handleExport('entries')} className="border px-3 py-2 rounded bg-white hover:bg-gray-50 text-sm">Export entries.csv</button>
        </div>
      </div>

      <div className="bg-white rounded shadow p-4 space-y-4">
        <h3 className="font-semibold">Import</h3>
        <p className="text-sm text-gray-500">
          Importez des fichiers CSV. Les habitudes sont upsert par <code className="bg-gray-100 px-1 rounded">id</code> sinon <code className="bg-gray-100 px-1 rounded">name</code>. Les entrées sont upsert par <code className="bg-gray-100 px-1 rounded">habit_id</code> (ou <code className="bg-gray-100 px-1 rounded">habit_name</code>) + <code className="bg-gray-100 px-1 rounded">date</code>.
        </p>
        <p className="text-xs text-gray-400">
          Formats détaillés : voir <code>IMPORT_FORMAT.md</code> à la racine du projet.
        </p>
        <div className="flex flex-wrap gap-2">
          <label className="border px-3 py-2 rounded bg-white cursor-pointer hover:bg-gray-50 text-sm">
            Import habits
            <input type="file" accept=".csv" className="hidden" onChange={e => handleImport(e, 'habits')} />
          </label>
          <label className="border px-3 py-2 rounded bg-white cursor-pointer hover:bg-gray-50 text-sm">
            Import entries
            <input type="file" accept=".csv" className="hidden" onChange={e => handleImport(e, 'entries')} />
          </label>
        </div>

        <details className="text-sm bg-gray-50 p-3 rounded">
          <summary className="cursor-pointer font-medium">Aperçu format habits.csv</summary>
          <pre className="mt-2 text-xs overflow-x-auto">
id,name,group_name,type,goal_value,goal_period,is_negative,archived_at,created_at
{"\n"}550e8400-...,Méditation,Bien-être,boolean,1,daily,0,,2025-01-01T08:00:00Z
{"\n"}...,Sport,Santé,numerical,3,weekly,0,,2025-01-03T08:00:00Z
{"\n"}...,Cigarettes,Santé,numerical,2,daily,1,,2025-01-03T08:00:00Z</pre>
        </details>
        <details className="text-sm bg-gray-50 p-3 rounded">
          <summary className="cursor-pointer font-medium">Aperçu format entries.csv</summary>
          <pre className="mt-2 text-xs overflow-x-auto">
habit_id,habit_name,date,value
{"\n"}550e8400-...,,2025-08-30,1
{"\n"},Méditation,2025-08-29,1
{"\n"}abc123,,2025-08-30,2</pre>
        </details>
      </div>

      <div className="bg-white rounded shadow p-4">
        <h3 className="font-semibold mb-2">Documentation</h3>
        <p className="text-sm text-gray-600">
          Le fichier <code className="bg-gray-100 px-1 rounded">IMPORT_FORMAT.md</code> décrit exhaustivement les colonnes, valeurs par défaut, règles d'upsert et exemples pour les deux imports.
        </p>
      </div>
    </div>
  )
}
