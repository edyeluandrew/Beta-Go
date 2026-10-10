export type AdminVendor = {
  id: string
  businessName: string
  contactName: string
  contactEmail: string
  phone: string
  address: string
  description: string
  shopPhotoUrl: string
  status: string
  isOpen: boolean
  acceptingOrders: boolean
  deliveryFeeUgx: number
}

export type Destination = {
  provider: string
  account: string
  status: string
}

export type PoolAccount = {
  vendorId: string
  availableUgx: number
  reservedUgx: number
  vendorStatus: string
  lowBalance: boolean
}

export type Adjustment = {
  id: string
  amountUgx: number
  reason: string
  status: string
}

export type SecurityReport = {
  env: string
  cookieSecure: boolean
  googleConfigured: boolean
  paymentProvider: string
  paymentWebhookSet: boolean
  sessionSecretSet: boolean
  sessionSecretIsExample: boolean
  webOrigin: string
  sandboxBlocked: boolean
}

export type AuditEntry = {
  id: string
  actorId: string
  action: string
  entityId: string
  reason: string
  createdAt: string
}

export type BackupCheck = {
  id: string
  note: string
  createdAt: string
}
