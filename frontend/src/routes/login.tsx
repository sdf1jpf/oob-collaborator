import { createFileRoute, redirect, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { api } from '../lib/api'
import { checkAuth } from '../lib/auth'

export const Route = createFileRoute('/login')({
  beforeLoad: async () => {
    if (await checkAuth()) {
      throw redirect({ to: '/dashboard' })
    }
  },
  component: LoginPage,
})

function LoginPage() {
  const navigate = useNavigate()
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      await api.login(password)
      navigate({ to: '/dashboard' })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-4">
      <form
        onSubmit={submit}
        className="w-full max-w-sm bg-surface-raised border border-surface-border rounded-lg p-6 space-y-4"
      >
        <div>
          <h1 className="text-xl font-bold">OOB Collaborator</h1>
          <p className="text-sm text-gray-400 mt-1">Sign in to the engagement dashboard</p>
        </div>

        {error && (
          <p className="text-sm text-red-400 bg-red-900/20 border border-red-800 rounded px-3 py-2">
            {error}
          </p>
        )}

        <div>
          <label className="block text-xs text-gray-400 mb-1">Admin Password</label>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="w-full bg-surface border border-surface-border rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-accent"
            autoFocus
          />
        </div>

        <button
          type="submit"
          disabled={loading || !password}
          className="w-full bg-accent hover:bg-blue-600 disabled:opacity-50 text-white rounded py-2 text-sm font-medium transition"
        >
          {loading ? 'Signing in...' : 'Sign In'}
        </button>
      </form>
    </div>
  )
}
