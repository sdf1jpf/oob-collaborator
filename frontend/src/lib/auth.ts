export async function checkAuth(): Promise<boolean> {
  try {
    const res = await fetch('/api/me', { credentials: 'include' })
    return res.ok
  } catch {
    return false
  }
}
