import { Outlet, createFileRoute, redirect, useNavigate } from '@tanstack/react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { api } from '../lib/api'
import { clearToken, isAuthenticated } from '../lib/auth'

export const Route = createFileRoute('/dashboard')({
  beforeLoad: () => {
    if (!isAuthenticated()) {
      throw redirect({ to: '/login' })
    }
  },
  component: DashboardLayout,
})

function DashboardLayout() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [showNew, setShowNew] = useState(false)
  const [name, setName] = useState('')
  const [clientName, setClientName] = useState('')

  const { data: engagements = [], isLoading } = useQuery({
    queryKey: ['engagements'],
    queryFn: api.listEngagements,
  })

  const createMutation = useMutation({
    mutationFn: () => api.createEngagement(name, clientName),
    onSuccess: (eng) => {
      queryClient.invalidateQueries({ queryKey: ['engagements'] })
      setShowNew(false)
      setName('')
      setClientName('')
      navigate({ to: '/dashboard/engagement/$engagementId', params: { engagementId: eng.id } })
    },
  })

  const logout = () => {
    clearToken()
    navigate({ to: '/login' })
  }

  return (
    <div className="h-screen flex">
      <aside className="w-64 border-r border-surface-border bg-surface-raised flex flex-col">
        <div className="p-4 border-b border-surface-border">
          <h1 className="font-bold text-sm tracking-wide">OOB Collaborator</h1>
          <p className="text-xs text-gray-500 mt-0.5">Engagements</p>
        </div>

        <div className="p-2">
          <button
            onClick={() => setShowNew(!showNew)}
            className="w-full text-xs bg-accent/20 hover:bg-accent/30 border border-accent/40 text-accent rounded py-1.5 transition"
          >
            + New Engagement
          </button>
        </div>

        {showNew && (
          <div className="px-2 pb-2 space-y-2">
            <input
              placeholder="Engagement name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="w-full bg-surface border border-surface-border rounded px-2 py-1 text-xs"
            />
            <input
              placeholder="Client name"
              value={clientName}
              onChange={(e) => setClientName(e.target.value)}
              className="w-full bg-surface border border-surface-border rounded px-2 py-1 text-xs"
            />
            <button
              onClick={() => createMutation.mutate()}
              disabled={!name || !clientName || createMutation.isPending}
              className="w-full text-xs bg-accent text-white rounded py-1 disabled:opacity-50"
            >
              Create
            </button>
          </div>
        )}

        <nav className="flex-1 overflow-y-auto p-2 space-y-1">
          {isLoading && <p className="text-xs text-gray-500 px-2">Loading...</p>}
          {engagements.map((eng) => (
            <a
              key={eng.id}
              href={`/dashboard/engagement/${eng.id}`}
              onClick={(e) => {
                e.preventDefault()
                navigate({ to: '/dashboard/engagement/$engagementId', params: { engagementId: eng.id } })
              }}
              className="block px-3 py-2 rounded text-sm hover:bg-surface-border/50 transition"
            >
              <p className="font-medium truncate">{eng.name}</p>
              <p className="text-xs text-gray-500 truncate">{eng.client_name}</p>
            </a>
          ))}
        </nav>

        <div className="p-3 border-t border-surface-border">
          <button
            onClick={logout}
            className="text-xs text-gray-400 hover:text-gray-200 transition"
          >
            Sign out
          </button>
        </div>
      </aside>

      <main className="flex-1 overflow-hidden">
        <Outlet />
      </main>
    </div>
  )
}
