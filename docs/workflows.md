# Workflows

The server stores each of these as separate events. A paid order is not accepted. An accepted order is not completed.

## Vendor lifecycle

1. An admin creates the account with business name, contact, phone, and address.
2. The vendor gets one expiring invite link and sets up access with Google on that account.
3. The admin registers the MTN or Airtel destination. Beta-Go never stores a PIN. Destination states are UNVERIFIED, PENDING_VERIFICATION, VERIFIED, FAILED, DISABLED.
4. A commission account is opened in UGX.
5. The vendor builds the shop: description, photos, logo, opening hours, categories, and items with whole-shilling prices. Go-live needs at least 3 available priced items, one shop photo, and opening hours.
6. An admin approves. The vendor is visible only when every eligibility rule below is true.

Account statuses: PENDING_APPROVAL, REJECTED, ACTIVE, INACTIVE, PAUSED, SUSPENDED. Opening hours and the busy toggle are flags (`is_open`, `accepting_orders`), not statuses.

A vendor can receive a new order only when all of these are true:

- Status is ACTIVE.
- Payment destination is VERIFIED.
- The shop is inside opening hours and accepting orders.
- The cart items are available, and the shop still meets the go-live minimum.
- Available commission balance is at least UGX 10,000 and at least this order's commission.
- No compliance block applies.

Available balance below UGX 10,000 sets the vendor INACTIVE in the same database transaction. The shop disappears. Orders already accepted continue. A verified top-up back to UGX 10,000 or more returns the vendor to ACTIVE, unless they are PAUSED or SUSPENDED. A warning is sent at UGX 20,000. INACTIVE is not a punishment.

## Order lifecycle

1. The customer picks food from one vendor. The server checks eligibility, item ownership, availability, quantity, and price. Minimum order is UGX 5,000.
2. The customer chooses collection or vendor delivery. The delivery fee is the vendor's, shown before payment, and excluded from commission. Collection shows a pickup code.
3. The order is created as PENDING_PAYMENT with an idempotency key, a price snapshot, and a commission-rate snapshot.
4. Payment is started with MTN MoMo or Airtel Money. Unpaid orders expire after 15 minutes. A late success is reconciled and refunded.
5. Only a verified provider success moves the payment to PAID and the order to AWAITING_VENDOR. The vendor gets a loud alert, with SMS if needed. The decision deadline is 10 minutes, with a reminder at 5.
6. Accept is one database transaction: lock the commission account, reserve the commission, set the order to CONFIRMED. If that reservation drops the available pool below UGX 10,000, the vendor becomes INACTIVE after commit.
7. The vendor moves the order through PREPARING, READY, and FULFILLED.
8. COMPLETED happens when the customer confirms, or 2 hours after FULFILLED if no dispute is open, or when an admin resolves a dispute.
9. Commission is captured once, on completion.

Order statuses: PENDING_PAYMENT, AWAITING_VENDOR, CONFIRMED, PREPARING, READY, FULFILLED, COMPLETED, REJECTED, CANCELLED, EXPIRED, DISPUTED.

Allowed moves:

- PENDING_PAYMENT → AWAITING_VENDOR, EXPIRED, CANCELLED
- AWAITING_VENDOR → CONFIRMED, REJECTED, EXPIRED, CANCELLED
- CONFIRMED → PREPARING, CANCELLED, DISPUTED
- PREPARING → READY, CANCELLED, DISPUTED
- READY → FULFILLED, CANCELLED, DISPUTED
- FULFILLED → COMPLETED, DISPUTED
- DISPUTED → COMPLETED, CANCELLED

Anything else is rejected. Terminal statuses are COMPLETED, REJECTED, CANCELLED, and EXPIRED.

Payment statuses are separate: PENDING, PAID, FAILED, EXPIRED, REFUND_PENDING, REFUNDED, REFUND_FAILED.

## Commission

Commission is 10% of the final food subtotal. Delivery, packaging, taxes, and provider fees are excluded. The amount is rounded half up to whole shillings. The rate is copied onto the order at creation and does not change later.

- **Available** funds can cover a new order and count toward the UGX 10,000 floor.
- **Reserved** funds are held for an accepted order that is not finished.
- **Captured** funds are Beta-Go revenue. They leave the pool.
- **Released** funds return to available when a reserved order is rejected or cancelled.

Available balance can never go below zero. Two accepts at the same time cannot spend the same funds: the accept transaction locks the commission account row. Capture happens once. Release happens once. Duplicate requests return the original result.

Worked example: pool UGX 50,000, food UGX 20,000. Accept reserves UGX 2,000, available becomes UGX 48,000. If the vendor then cannot fulfil, the reservation returns to UGX 50,000, the customer is refunded UGX 20,000, and no commission is captured.

Top-ups are verified on the server. Minimum UGX 10,000. Maximum single top-up UGX 1,000,000. A screenshot never credits a pool.

## Refunds and disputes

A refund is its own transaction. Deciding one is due, requesting it, the provider processing it, and the provider confirming it are four different steps. The money goes back to the original method. The total refunded can never exceed the amount paid. A failed refund stays visible for an admin retry.

A customer can open a dispute from FULFILLED until 24 hours later. That blocks auto-completion and commission capture. An unresolved dispute alerts a super admin after 48 hours. If a refund is ordered after commission was already captured, the correction is a new ledger adjustment. The old capture is never edited.

## Cancellation

- Before payment: close the order. A late success is flagged and refunded.
- Paid, not yet accepted: the customer may cancel. Full refund. No commission was reserved.
- After acceptance, before PREPARING: the customer asks and the vendor approves. Full refund. Reservation released.
- After PREPARING: the customer cannot cancel. The path is a dispute.
- Vendor cannot fulfil after acceptance: full refund, reservation released, and it counts against the vendor.
