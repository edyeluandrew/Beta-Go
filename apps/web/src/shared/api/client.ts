export class ApiError extends Error {
  status: number
  code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
  })

  if (response.status === 204) {
    return undefined as T
  }

  const body: unknown = await response.json().catch(() => null)

  if (!response.ok) {
    const error =
      body && typeof body === 'object' && 'error' in body
        ? (body as { error?: { code?: string; message?: string } }).error
        : undefined
    throw new ApiError(response.status, error?.code ?? 'error', error?.message ?? 'Something went wrong.')
  }

  return body as T
}
