import { useState } from 'react'
import type { Interaction, IPRecon } from '../lib/api'

function formatRawData(raw: string, protocol: string): string {
  try {
    const parsed = JSON.parse(raw)
    if (protocol === 'HTTP') {
      const method = parsed.method || 'GET'
      const host = parsed.host || ''
      const requestLine =
        parsed.request_uri ||
        (parsed.path || '/') +
          (parsed.query
            ? '?' + new URLSearchParams(flattenQuery(parsed.query)).toString()
            : '')
      let out = `${method} ${requestLine} HTTP/1.1\nHost: ${host}\n`
      if (parsed.headers) {
        for (const [k, vals] of Object.entries(parsed.headers)) {
          for (const v of vals as string[]) {
            out += `${k}: ${v}\n`
          }
        }
      }
      out += '\n'
      if (parsed.body) out += parsed.body
      return out
    }
    return JSON.stringify(parsed, null, 2)
  } catch {
    return raw
  }
}

function httpPath(raw: string): string {
  try {
    const parsed = JSON.parse(raw)
    return parsed.path || parsed.request_uri || '—'
  } catch {
    return '—'
  }
}

function flattenQuery(q: Record<string, string[]>) {
  const out: Record<string, string> = {}
  for (const [k, vals] of Object.entries(q)) {
    out[k] = vals.join(',')
  }
  return out
}

export function RawInspector({
  interaction,
  onClose,
}: {
  interaction: Interaction | null
  onClose: () => void
}) {
  const [copied, setCopied] = useState(false)

  if (!interaction) {
    return (
      <div className="h-full flex items-center justify-center text-gray-500 text-sm p-6">
        Select an interaction to inspect
      </div>
    )
  }

  const formatted = formatRawData(interaction.raw_data, interaction.protocol)

  const copy = async () => {
    await navigator.clipboard.writeText(formatted)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="h-full flex flex-col border-l border-surface-border bg-surface-raised">
      <div className="flex items-center justify-between px-4 py-3 border-b border-surface-border">
        <h3 className="font-semibold text-sm">Inspector</h3>
        <div className="flex gap-2">
          <button
            onClick={copy}
            className="text-xs px-2 py-1 rounded bg-surface-border hover:bg-gray-600 transition"
          >
            {copied ? 'Copied!' : 'Copy'}
          </button>
          <button
            onClick={onClose}
            className="text-xs px-2 py-1 rounded bg-surface-border hover:bg-gray-600 transition"
          >
            Close
          </button>
        </div>
      </div>

      <div className="p-4 space-y-3 text-sm overflow-y-auto flex-1">
        <div className="grid grid-cols-2 gap-2">
          <Meta label="Time" value={new Date(interaction.interacted_at).toLocaleString()} />
          <Meta label="Protocol" value={interaction.protocol} />
          <Meta label="Source IP" value={interaction.source_ip} />
          <Meta label="Host" value={interaction.host || interaction.sub_domain} />
          <Meta label="Path" value={httpPath(interaction.raw_data)} />
          <Meta label="Subdomain" value={interaction.sub_domain || '—'} />
        </div>

        <div>
          <p className="text-xs text-gray-400 mb-2 uppercase tracking-wide">IP Recon</p>
          <div className="grid grid-cols-2 gap-2">
            <Meta
              label="Location"
              value={formatLocation(interaction.ip_recon)}
            />
            <Meta
              label="Reverse DNS"
              value={interaction.ip_recon?.reverse_dns || interaction.reverse_dns || '—'}
            />
            <Meta label="ASN" value={interaction.ip_recon?.asn || '—'} />
            <Meta label="ISP" value={interaction.ip_recon?.isp || '—'} />
            <Meta label="Org" value={interaction.ip_recon?.org || '—'} />
            <Meta
              label="Coordinates"
              value={formatCoordinates(interaction.ip_recon)}
            />
          </div>
        </div>

        <div>
          <p className="text-xs text-gray-400 mb-1 uppercase tracking-wide">Raw Request</p>
          <pre className="bg-black/40 border border-surface-border rounded p-3 text-xs font-mono overflow-x-auto whitespace-pre-wrap text-green-300/90">
            {formatted}
          </pre>
        </div>
      </div>
    </div>
  )
}

function formatLocation(recon?: IPRecon): string {
  if (!recon) return 'Pending…'
  const parts = [recon.city, recon.region, recon.country].filter(Boolean)
  if (parts.length === 0) return '—'
  return parts.join(', ')
}

function formatCoordinates(recon?: IPRecon): string {
  if (!recon || recon.lat == null || recon.lon == null) return '—'
  return `${recon.lat.toFixed(4)}, ${recon.lon.toFixed(4)}`
}

function Meta({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-xs text-gray-500">{label}</p>
      <p className="font-mono text-gray-200 truncate">{value}</p>
    </div>
  )
}
