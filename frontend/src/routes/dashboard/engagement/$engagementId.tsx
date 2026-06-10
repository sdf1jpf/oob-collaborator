import { createFileRoute } from '@tanstack/react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useRef, useState } from 'react'
import { InteractionTable } from '../../../components/InteractionTable'
import { RawInspector } from '../../../components/RawInspector'
import { api } from '../../../lib/api'
import { connectInteractionWS } from '../../../lib/ws'

export const Route = createFileRoute('/dashboard/engagement/$engagementId')({
  component: EngagementView,
})

function formatBytes(size: number) {
  if (size < 1024) return `${size} B`
  return `${(size / 1024).toFixed(1)} KiB`
}

function EngagementView() {
  const { engagementId } = Route.useParams()
  const queryClient = useQueryClient()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [desc, setDesc] = useState('')
  const [showPayloads, setShowPayloads] = useState(false)
  const [showHostedFiles, setShowHostedFiles] = useState(false)
  const [uploadPath, setUploadPath] = useState('')

  const { data: engagements = [] } = useQuery({
    queryKey: ['engagements'],
    queryFn: api.listEngagements,
  })
  const engagement = engagements.find((e) => e.id === engagementId)

  const { data: interactions = [], isLoading } = useQuery({
    queryKey: ['interactions', engagementId],
    queryFn: () => api.listInteractions(engagementId),
    refetchInterval: 30_000,
  })

  const { data: payloads = [] } = useQuery({
    queryKey: ['payloads', engagementId],
    queryFn: () => api.listPayloads(engagementId),
  })

  const { data: hostedFiles = [] } = useQuery({
    queryKey: ['hostedFiles', engagementId],
    queryFn: () => api.listHostedFiles(engagementId),
  })

  const generateMutation = useMutation({
    mutationFn: () => api.generatePayload(engagementId, desc),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['payloads', engagementId] })
      queryClient.invalidateQueries({ queryKey: ['hostedFiles', engagementId] })
      setDesc('')
      setShowPayloads(true)
    },
  })

  const uploadMutation = useMutation({
    mutationFn: (formData: FormData) => api.uploadHostedFile(engagementId, formData),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['hostedFiles', engagementId] })
      setUploadPath('')
      if (fileInputRef.current) fileInputRef.current.value = ''
      setShowHostedFiles(true)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (fileId: string) => api.deleteHostedFile(fileId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['hostedFiles', engagementId] })
    },
  })

  useEffect(() => {
    const ws = connectInteractionWS(queryClient, engagementId)
    return () => ws.close()
  }, [engagementId, queryClient])

  const selected = useMemo(
    () => interactions.find((i) => i.id === selectedId) ?? null,
    [interactions, selectedId],
  )

  const handleFileUpload = (file: File | null) => {
    if (!file) return
    const formData = new FormData()
    formData.append('file', file)
    const path = uploadPath.trim() || file.name
    formData.append('path', path)
    uploadMutation.mutate(formData)
  }

  return (
    <div className="h-full flex flex-col">
      <header className="px-4 py-3 border-b border-surface-border flex items-center justify-between gap-4">
        <div>
          <h2 className="font-semibold">{engagement?.name || 'Engagement'}</h2>
          <p className="text-xs text-gray-500">{engagement?.client_name}</p>
        </div>

        <div className="flex items-center gap-2 flex-wrap justify-end">
          <div className="flex items-center gap-1 text-xs text-green-400">
            <span className="w-2 h-2 rounded-full bg-green-400 animate-pulse" />
            Live
          </div>
          <input
            placeholder="Payload description"
            value={desc}
            onChange={(e) => setDesc(e.target.value)}
            className="bg-surface border border-surface-border rounded px-2 py-1 text-xs w-48"
          />
          <button
            onClick={() => generateMutation.mutate()}
            disabled={generateMutation.isPending}
            className="text-xs bg-accent text-white rounded px-3 py-1 disabled:opacity-50"
          >
            Generate Payload
          </button>
          <button
            onClick={() => setShowPayloads(!showPayloads)}
            className="text-xs border border-surface-border rounded px-3 py-1 hover:bg-surface-raised"
          >
            Payloads ({payloads.length || '…'})
          </button>
          <button
            onClick={() => setShowHostedFiles(!showHostedFiles)}
            className="text-xs border border-surface-border rounded px-3 py-1 hover:bg-surface-raised"
          >
            Hosted Files ({hostedFiles.length || '…'})
          </button>
        </div>
      </header>

      {showPayloads && payloads.length > 0 && (
        <div className="px-4 py-2 border-b border-surface-border bg-surface-raised/50 max-h-32 overflow-y-auto">
          {payloads.map((p) => (
            <div key={p.id} className="flex items-center justify-between text-xs py-1">
              <span className="font-mono text-accent">{p.full_domain}</span>
              <span className="text-gray-500">{p.description || '—'}</span>
              <button
                onClick={() => navigator.clipboard.writeText(p.full_domain)}
                className="text-gray-400 hover:text-white"
              >
                Copy
              </button>
            </div>
          ))}
        </div>
      )}

      {showHostedFiles && (
        <div className="px-4 py-2 border-b border-surface-border bg-surface-raised/50">
          <p className="text-xs text-gray-500 mb-2">
            Reference as{' '}
            <span className="font-mono text-gray-400">
              https://{'{payload}'}.yourdomain.com/yourfile.dtd
            </span>{' '}
            in external entity declarations (XXE, etc.). Any payload token in this engagement can
            serve uploaded files.
          </p>
          <div className="flex items-center gap-2 mb-2 flex-wrap">
            <input
              ref={fileInputRef}
              type="file"
              onChange={(e) => handleFileUpload(e.target.files?.[0] ?? null)}
              className="text-xs text-gray-400"
            />
            <input
              placeholder="Path override (optional)"
              value={uploadPath}
              onChange={(e) => setUploadPath(e.target.value)}
              className="bg-surface border border-surface-border rounded px-2 py-1 text-xs w-48"
            />
            {uploadMutation.isPending && (
              <span className="text-xs text-gray-500">Uploading…</span>
            )}
            {uploadMutation.isError && (
              <span className="text-xs text-red-400">{uploadMutation.error.message}</span>
            )}
          </div>
          {hostedFiles.length > 0 ? (
            <div className="max-h-32 overflow-y-auto">
              {hostedFiles.map((f) => (
                <div key={f.id} className="flex items-center justify-between text-xs py-1 gap-2">
                  <span className="font-mono text-accent shrink-0">{f.path}</span>
                  <span className="text-gray-500 truncate">{f.content_type}</span>
                  <span className="text-gray-500 shrink-0">{formatBytes(f.size)}</span>
                  <button
                    onClick={() => navigator.clipboard.writeText(f.example_url)}
                    className="text-gray-400 hover:text-white shrink-0"
                  >
                    Copy URL
                  </button>
                  <button
                    onClick={() => deleteMutation.mutate(f.id)}
                    disabled={deleteMutation.isPending}
                    className="text-red-400 hover:text-red-300 shrink-0 disabled:opacity-50"
                  >
                    Delete
                  </button>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-xs text-gray-500">No hosted files yet.</p>
          )}
        </div>
      )}

      <div className="flex-1 flex overflow-hidden">
        <div className="flex-1 overflow-hidden">
          {isLoading ? (
            <p className="p-4 text-gray-500 text-sm">Loading interactions...</p>
          ) : (
            <InteractionTable
              data={interactions}
              selectedId={selectedId}
              onSelect={(row) => setSelectedId(row.id)}
            />
          )}
        </div>
        <div className="w-[420px] shrink-0">
          <RawInspector interaction={selected} onClose={() => setSelectedId(null)} />
        </div>
      </div>
    </div>
  )
}
