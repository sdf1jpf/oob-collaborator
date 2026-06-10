const colors: Record<string, string> = {
  HTTP: 'bg-blue-900/60 text-blue-300 border-blue-700',
  DNS: 'bg-purple-900/60 text-purple-300 border-purple-700',
  SMTP: 'bg-amber-900/60 text-amber-300 border-amber-700',
}

export function ProtocolBadge({ protocol }: { protocol: string }) {
  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border ${colors[protocol] || 'bg-gray-800 text-gray-300 border-gray-600'}`}
    >
      {protocol}
    </span>
  )
}
