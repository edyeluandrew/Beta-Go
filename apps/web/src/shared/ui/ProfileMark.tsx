import { CircleUser } from 'lucide-react'
import type { User } from '../auth/roles'

export function ProfileMark({ user, className = 'h-10 w-10' }: { user: User; className?: string }) {
  if (user.pictureUrl) {
    return <img src={user.pictureUrl} alt="" referrerPolicy="no-referrer" className={`${className} rounded-full object-cover`} />
  }
  return (
    <span className={`${className} grid place-items-center rounded-full bg-leaf text-forest`}>
      <CircleUser aria-hidden className="h-1/2 w-1/2" />
    </span>
  )
}
