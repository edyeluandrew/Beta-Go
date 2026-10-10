-- Phase 3: one-vendor orders. Payment stays PENDING until the provider phase.
-- Accept, reject, and commission capture are later migrations.

CREATE TABLE platform_settings (
    key TEXT PRIMARY KEY,
    value_int INTEGER NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO platform_settings (key, value_int) VALUES
    ('commission_rate_percent', 10),
    ('minimum_order_ugx', 5000);

CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_user_id UUID NOT NULL REFERENCES users (id),
    vendor_id UUID NOT NULL REFERENCES vendors (id),
    status TEXT NOT NULL DEFAULT 'PENDING_PAYMENT',
    fulfilment TEXT NOT NULL,
    delivery_address TEXT NOT NULL DEFAULT '',
    customer_phone TEXT NOT NULL DEFAULT '',
    food_subtotal_ugx INTEGER NOT NULL,
    delivery_fee_ugx INTEGER NOT NULL,
    total_ugx INTEGER NOT NULL,
    commission_rate_percent INTEGER NOT NULL,
    commission_ugx INTEGER NOT NULL,
    idempotency_key TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT orders_status_check CHECK (status IN (
        'PENDING_PAYMENT', 'AWAITING_VENDOR', 'CONFIRMED', 'PREPARING', 'READY',
        'FULFILLED', 'COMPLETED', 'REJECTED', 'CANCELLED', 'EXPIRED', 'DISPUTED'
    )),
    CONSTRAINT orders_fulfilment_check CHECK (fulfilment IN ('collection', 'delivery')),
    CONSTRAINT orders_money_check CHECK (
        food_subtotal_ugx >= 0 AND delivery_fee_ugx >= 0 AND total_ugx >= 0 AND commission_ugx >= 0
    ),
    UNIQUE (customer_user_id, idempotency_key)
);

CREATE TABLE order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    menu_item_id UUID,
    name TEXT NOT NULL,
    quantity INTEGER NOT NULL,
    unit_price_ugx INTEGER NOT NULL,
    subtotal_ugx INTEGER NOT NULL,
    CONSTRAINT order_items_quantity_check CHECK (quantity > 0),
    CONSTRAINT order_items_price_check CHECK (unit_price_ugx > 0 AND subtotal_ugx > 0)
);

CREATE TABLE order_status_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    previous_status TEXT,
    new_status TEXT NOT NULL,
    actor_user_id UUID,
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL UNIQUE REFERENCES orders (id),
    status TEXT NOT NULL DEFAULT 'PENDING',
    amount_ugx INTEGER NOT NULL,
    currency TEXT NOT NULL DEFAULT 'UGX',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT payments_status_check CHECK (status IN (
        'PENDING', 'PAID', 'FAILED', 'EXPIRED', 'REFUND_PENDING', 'REFUNDED', 'REFUND_FAILED'
    )),
    CONSTRAINT payments_amount_check CHECK (amount_ugx >= 0)
);
