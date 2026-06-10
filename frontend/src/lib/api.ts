export interface Engagement {
  id: string
  name: string
  client_name: string
  created_at: string
}

export interface Payload {
  id: string
  engagement_id: string
  sub_domain: string
  description: string
  created_at: string
  full_domain: string
}

export interface IPRecon {
  ip: string
  reverse_dns: string
  country: string
  country_code: string
  region: string
  city: string
  lat?: number
  lon?: number
  isp: string
  org: string
  asn: string
  status: string
  updated_at: string
}

export interface HostedFile {
  id: string
  engagement_id: string
  path: string
  content_type: string
  size: number
  created_at: string
  example_url: string
}

export interface Interaction {
  id: string
  payload_id?: string
  engagement_id?: string
  sub_domain: string
  protocol: 'HTTP' | 'DNS' | 'SMTP'
  source_ip: string
  reverse_dns?: string
  ip_recon?: IPRecon
  host: string
  raw_data: string
  interacted_at: string
  delivered_at?: string
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(init?.headers as Record<string, string>),
  }

  const res = await fetch(path, { ...init, headers, credentials: 'include' })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error || `Request failed: ${res.status}`)
  }
  return res.json()
}

export const api = {
  login: (password: string) =>
    request<{ ok: boolean }>('/api/login', {
      method: 'POST',
      body: JSON.stringify({ password }),
    }),

  logout: () =>
    request<{ ok: boolean }>('/api/logout', {
      method: 'POST',
    }),

  me: () => request<{ authenticated: boolean }>('/api/me'),

  listEngagements: () => request<Engagement[]>('/api/engagements'),

  createEngagement: (name: string, client_name: string) =>
    request<Engagement>('/api/engagements', {
      method: 'POST',
      body: JSON.stringify({ name, client_name }),
    }),

  listInteractions: (engagementId: string) =>
    request<Interaction[]>(`/api/engagements/${engagementId}/interactions`),

  listPayloads: (engagementId: string) =>
    request<Payload[]>(`/api/engagements/${engagementId}/payloads`),

  generatePayload: (engagement_id: string, description: string) =>
    request<Payload>('/api/payloads/generate', {
      method: 'POST',
      body: JSON.stringify({ engagement_id, description }),
    }),

  listHostedFiles: (engagementId: string) =>
    request<HostedFile[]>(`/api/engagements/${engagementId}/files`),

  uploadHostedFile: async (engagementId: string, formData: FormData) => {
    const res = await fetch(`/api/engagements/${engagementId}/files`, {
      method: 'POST',
      body: formData,
      credentials: 'include',
    })
    if (!res.ok) {
      const body = await res.json().catch(() => ({}))
      throw new Error(body.error || `Request failed: ${res.status}`)
    }
    return res.json() as Promise<HostedFile>
  },

  deleteHostedFile: async (fileId: string) => {
    const res = await fetch(`/api/files/${fileId}`, {
      method: 'DELETE',
      credentials: 'include',
    })
    if (!res.ok) {
      const body = await res.json().catch(() => ({}))
      throw new Error(body.error || `Request failed: ${res.status}`)
    }
  },
}
