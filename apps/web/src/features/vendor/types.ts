export type VendorShop = {
  id: string
  businessName: string
  description: string
  address: string
  logoUrl: string
  shopPhotoUrl: string
  status: string
  isOpen: boolean
  acceptingOrders: boolean
  deliveryFeeUgx: number
}

export type MenuItem = {
  id: string
  name: string
  description: string
  priceUgx: number
  available: boolean
  photoUrl: string
}

export type Category = {
  id: string
  name: string
}

export type OpeningHour = {
  weekday: number
  opensAt: string
  closesAt: string
  closed: boolean
}

export type VendorWorkspace = {
  vendor: VendorShop
  hours: OpeningHour[]
  categories: Category[]
  items: MenuItem[]
}

export type KitchenOrder = {
  id: string
  status: string
  fulfilment: string
  totalUgx: number
  pickupCode: string
  cancelRequested: boolean
}

export type PoolAccount = {
  availableUgx: number
  reservedUgx: number
  vendorStatus: string
  lowBalance: boolean
}

export type LedgerEntry = {
  id: string
  stream: string
  direction: string
  amountUgx: number
  operation: string
}
