import type { QueryClient } from '@tanstack/react-query'
import type { Interaction } from './api'

export function connectInteractionWS(
  queryClient: QueryClient,
  engagementId: string | undefined,
) {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const ws = new WebSocket(`${protocol}//${window.location.host}/ws`)

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)
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
