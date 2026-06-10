import { createFileRoute, redirect } from '@tanstack/react-router'
import { checkAuth } from '../lib/auth'

export const Route = createFileRoute('/')({
  beforeLoad: async () => {
    throw redirect({ to: (await checkAuth()) ? '/dashboard' : '/login' })
  },
})
