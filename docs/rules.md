# Rules that stay fixed

- Beta-Go is a marketplace for Kabale. Customers and vendors are the commercial parties.
- Admins create vendor accounts. Vendors manage their own menu, prices, and photos.
- Public Google sign-in creates customers only.
- Payments are MTN MoMo and Airtel Money. No cash on delivery in the MVP.
- Payment model is A: the customer pays the vendor destination. Commission is the prepaid pool.
- Payment routing and refunds are proved with the provider before any live transaction.
- Payment success is verified on the server only.
- Payment, vendor acceptance, fulfilment, and completion are separate events.
- Commission is 10% of the food subtotal. The rate is snapshotted on the order.
- Commission comes from a prepaid pool, separate from food revenue.
- Available pool balance never goes negative.
- Below UGX 10,000 available, the vendor is INACTIVE until a verified top-up.
- Orders already accepted continue while the vendor is inactive.
- Commission is reserved on acceptance, captured once on completion, and released on a valid rejection or cancellation.
- Rejected, expired, or validly cancelled paid orders are refunded. Starting a refund is not the same as finishing one.
- Vendors arrange delivery. Beta-Go has no riders.
- Order, payment, refund, and commission statuses stay separate.
- Financial operations are idempotent, double-entry, and auditable. Posted ledger rows are never edited.
- Disputes block completion and commission capture.
- Uncertain money operations are reconciled, never ignored.
- All amounts are whole shillings.
- Secrets stay in the environment. Product code never hardcodes them. A security report never returns secret values.
- The server is the RBAC enforcer. UI guards are not enough. Public Google sign-in creates `customer` only. Vendor is granted by invite accept. `support` and `finance` are granted by a super_admin. Nobody grants themselves `super_admin`. Support cannot make financial adjustments or grant roles.
- Production refuses to start with sandbox payments, an insecure cookie, the example session secret, or a localhost web origin.

Launch defaults, changeable later in platform settings: commission 10%, activity floor UGX 10,000, low-balance warning UGX 20,000, top-up UGX 10,000 to UGX 1,000,000, minimum order UGX 5,000, unpaid expiry 15 minutes, vendor decision 10 minutes, auto-complete 2 hours after fulfilled, dispute window 24 hours, dispute escalation 48 hours.

Out of scope for the MVP: platform riders, multi-vendor carts, self-service vendor signup, loyalty, partial refunds, and more than one country.
