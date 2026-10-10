export type MenuItem = {
  id: string
  name: string
  description: string
  priceUgx: number
  available: boolean
  photoUrl: string
}

export type PublicVendor = {
  id: string
  businessName: string
  description: string
  address: string
  shopPhotoUrl: string
  logoUrl: string
  deliveryFeeUgx: number
  isOpen: boolean
  acceptingOrders: boolean
  dishes?: MenuItem[]
}

export type OpeningHour = {
  weekday: number
  opensAt: string
  closesAt: string
  closed: boolean
}

export type PublicShop = {
  vendor: PublicVendor
  hours: OpeningHour[]
  items: MenuItem[]
}

export type OrderItem = {
  itemId: string
  name: string
  quantity: number
  unitPriceUgx: number
  subtotalUgx: number
}

export type CustomerOrder = {
  id: string
  vendorId: string
  status: string
  fulfilment: string
  address: string
  phone: string
  foodSubtotalUgx: number
  deliveryFeeUgx: number
  totalUgx: number
  pickupCode: string
  cancelRequested: boolean
  items: OrderItem[]
}

export type Payment = {
  id: string
  orderId: string
  status: string
  amountUgx: number
  provider: string
  phone: string
  providerReference: string
  lateSuccess: boolean
  expiresAt?: string
}
