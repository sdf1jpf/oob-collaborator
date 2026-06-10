import { useMemo, useState } from 'react'
import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  useReactTable,
} from '@tanstack/react-table'
import type { Interaction } from '../lib/api'
import { ProtocolBadge } from './ProtocolBadge'

const columnHelper = createColumnHelper<Interaction>()

function httpPathFromRaw(raw: string, protocol: string): string {
  if (protocol !== 'HTTP') return '—'
  try {
    const parsed = JSON.parse(raw)
    return parsed.path || parsed.request_uri || '—'
  } catch {
    return '—'
  }
}

interface Props {
  data: Interaction[]
  selectedId: string | null
  onSelect: (row: Interaction) => void
}

export function InteractionTable({ data, selectedId, onSelect }: Props) {
  const [globalFilter, setGlobalFilter] = useState('')
  const [protocolFilter, setProtocolFilter] = useState<string>('ALL')

  const filteredData = useMemo(() => {
    if (protocolFilter === 'ALL') return data
    return data.filter((d) => d.protocol === protocolFilter)
  }, [data, protocolFilter])

  const columns = useMemo(
    () => [
      columnHelper.accessor('interacted_at', {
        header: 'Time',
        cell: (info) => new Date(info.getValue()).toLocaleString(),
      }),
      columnHelper.accessor('protocol', {
        header: 'Protocol',
        cell: (info) => <ProtocolBadge protocol={info.getValue()} />,
      }),
      columnHelper.accessor('source_ip', { header: 'Source IP' }),
      columnHelper.accessor('sub_domain', { header: 'Subdomain' }),
      columnHelper.display({
        id: 'path',
        header: 'Path',
        cell: ({ row }) => (
          <span className="font-mono text-xs truncate max-w-[160px] block">
            {httpPathFromRaw(row.original.raw_data, row.original.protocol)}
          </span>
        ),
      }),
      columnHelper.accessor('host', {
        header: 'Host',
        cell: (info) => (
          <span className="font-mono text-xs truncate max-w-[200px] block">
            {info.getValue()}
          </span>
        ),
      }),
    ],
    [],
  )

  const table = useReactTable({
    data: filteredData,
    columns,
    state: { globalFilter },
    onGlobalFilterChange: setGlobalFilter,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    initialState: { pagination: { pageSize: 15 } },
  })

  return (
    <div className="flex flex-col h-full">
      <div className="flex items-center gap-3 p-3 border-b border-surface-border">
        <input
          value={globalFilter}
          onChange={(e) => setGlobalFilter(e.target.value)}
          placeholder="Filter by IP or subdomain..."
          className="flex-1 bg-surface border border-surface-border rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-accent"
        />
        <div className="flex gap-1">
          {['ALL', 'HTTP', 'DNS', 'SMTP'].map((p) => (
            <button
              key={p}
              onClick={() => setProtocolFilter(p)}
              className={`text-xs px-2 py-1 rounded border transition ${
                protocolFilter === p
                  ? 'bg-accent-muted border-accent text-accent'
                  : 'border-surface-border text-gray-400 hover:text-gray-200'
              }`}
            >
              {p}
            </button>
          ))}
        </div>
      </div>

      <div className="flex-1 overflow-auto">
        <table className="w-full text-sm">
          <thead className="sticky top-0 bg-surface-raised z-10">
            {table.getHeaderGroups().map((hg) => (
              <tr key={hg.id} className="border-b border-surface-border">
                {hg.headers.map((header) => (
                  <th
                    key={header.id}
                    className="text-left px-3 py-2 text-xs font-medium text-gray-400 uppercase tracking-wide"
                  >
                    {flexRender(header.column.columnDef.header, header.getContext())}
                  </th>
                ))}
              </tr>
            ))}
          </thead>
          <tbody>
            {table.getRowModel().rows.map((row) => (
              <tr
                key={row.id}
                onClick={() => onSelect(row.original)}
                className={`border-b border-surface-border/50 cursor-pointer hover:bg-surface-raised/80 transition ${
                  selectedId === row.original.id ? 'bg-accent-muted/30' : ''
                }`}
              >
                {row.getVisibleCells().map((cell) => (
                  <td key={cell.id} className="px-3 py-2">
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </td>
                ))}
              </tr>
            ))}
            {table.getRowModel().rows.length === 0 && (
              <tr>
                <td colSpan={columns.length} className="text-center py-12 text-gray-500">
                  No interactions yet. Generate a payload and trigger an OOB hit.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      <div className="flex items-center justify-between px-3 py-2 border-t border-surface-border text-xs text-gray-400">
        <span>
          {table.getFilteredRowModel().rows.length} interaction(s)
        </span>
        <div className="flex gap-2">
          <button
            onClick={() => table.previousPage()}
            disabled={!table.getCanPreviousPage()}
            className="px-2 py-1 rounded border border-surface-border disabled:opacity-40"
          >
            Prev
          </button>
          <span>
            Page {table.getState().pagination.pageIndex + 1} of {table.getPageCount() || 1}
          </span>
          <button
            onClick={() => table.nextPage()}
            disabled={!table.getCanNextPage()}
            className="px-2 py-1 rounded border border-surface-border disabled:opacity-40"
          >
            Next
          </button>
        </div>
      </div>
    </div>
  )
}
