import { Component, type ErrorInfo, type ReactNode } from 'react'
import { useI18n } from '../i18n'

// A2: a render error (bad data from the server) must not blank the whole app.
type Props = { children: ReactNode }
type State = { failed: boolean }

export default class ErrorBoundary extends Component<Props, State> {
  state: State = { failed: false }

  static getDerivedStateFromError(): State {
    return { failed: true }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('render error', error, info.componentStack)
  }

  render() {
    if (this.state.failed) return <ErrorFallback />
    return this.props.children
  }
}

function ErrorFallback() {
  const { t } = useI18n()
  return (
    <div className="py-10 text-center text-red-600 space-y-3">
      <p>{t('errorBoundary')}</p>
      <button onClick={() => window.location.reload()} className="border px-3 py-1 rounded text-sm bg-white">
        {t('reload')}
      </button>
    </div>
  )
}
