import type { QueryClient } from '@tanstack/react-query'
import type { Interaction, IPRecon } from './api'

function applyIPRecon(interactions: Interaction[], ipRecon: IPRecon): Interaction[] {
  return interactions.map((i) =>
    i.source_ip === ipRecon.ip ? { ...i, ip_recon: ipRecon } : i,
  )
}

export function connectInteractionWS(
  queryClient: QueryClient,
  engagementId: string | undefined,
) {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const ws = new WebSocket(`${protocol}//${window.location.host}/ws`)

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)

      if (msg.type === 'ip_recon' && msg.ip_recon) {
        const ipRecon = msg.ip_recon as IPRecon
        queryClient.setQueryData<Interaction[]>(
          ['interactions', engagementId],
          (old) => (old ? applyIPRecon(old, ipRecon) : old),
        )
        return
      }

      if (msg.type !== 'interaction' || !msg.interaction) return

      const interaction = msg.interaction as Interaction
      if (engagementId && interaction.engagement_id !== engagementId) return

      queryClient.setQueryData<Interaction[]>(
        ['interactions', engagementId],
        (old) => {
          const enriched: Interaction = {
            ...interaction,
            host: interaction.sub_domain
              ? `${interaction.sub_domain}.${window.location.hostname}`
              : window.location.hostname,
            raw_data:
              typeof interaction.raw_data === 'string'
                ? interaction.raw_data
                : JSON.stringify(interaction.raw_data),
          }
          if (!old) return [enriched]
          if (old.some((i) => i.id === enriched.id)) return old
          return [enriched, ...old]
        },
      )
    } catch {
      // ignore malformed messages
    }
  }

  return ws
}
