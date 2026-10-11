import type { ButtonHTMLAttributes, ReactNode } from 'react'

export function Notice({ children }: { children: ReactNode }) {
  return <p className="mt-4 text-muted">{children}</p>
}

export function ErrorText({ children }: { children?: string }) {
  if (!children) return null
  return <p className="mt-3 text-danger">{children}</p>
}

export function PrimaryButton({
  children,
  className = '',
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      className={`rounded-full bg-forest px-4 py-2.5 text-white disabled:cursor-not-allowed disabled:opacity-50 ${className}`}
      {...props}
    >
      {children}
    </button>
  )
}

export function GhostButton({
  children,
  className = '',
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      className={`rounded-full border border-line bg-transparent px-3 py-1.5 text-ink disabled:opacity-50 ${className}`}
      {...props}
    >
      {children}
    </button>
  )
}

export function Field({
  label,
  error,
  children,
}: {
  label: string
  error?: string
  children: ReactNode
}) {
  return (
    <label className="grid gap-1.5 text-sm text-muted">
      {label}
      {children}
      {error ? <span className="text-danger">{error}</span> : null}
    </label>
  )
}

export const inputClass = 'w-full rounded-xl border border-line bg-white px-3 py-2 text-ink'

export function Card({ children, className = '' }: { children: ReactNode; className?: string }) {
  return <div className={`rounded-2xl border border-line bg-card p-4 ${className}`}>{children}</div>
}
