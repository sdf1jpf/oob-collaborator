import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/dashboard/')({
  component: DashboardHome,
})

function DashboardHome() {
  return (
    <div className="h-full flex items-center justify-center text-gray-500">
      <div className="text-center">
        <p className="text-lg">Select an engagement</p>
        <p className="text-sm mt-1">or create a new one from the sidebar</p>
      </div>
    </div>
  )
}
