-- Phase 6: accept, reject, fulfilment, refunds, and disputes.

ALTER TABLE orders
    ADD COLUMN pickup_code TEXT NOT NULL DEFAULT '',
    ADD COLUMN cancel_requested BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN decision_deadline_at TIMESTAMPTZ,
    ADD COLUMN fulfilled_at TIMESTAMPTZ;

INSERT INTO platform_settings (key, value_int) VALUES
    ('vendor_decision_minutes', 10),
    ('autocomplete_minutes', 120),
    ('dispute_window_hours', 24),
    ('dispute_escalate_hours', 48)
ON CONFLICT (key) DO NOTHING;

CREATE TABLE refunds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders (id),
    amount_ugx INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'REQUESTED',
    reason TEXT NOT NULL DEFAULT '',
    provider_reference TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT refunds_amount_check CHECK (amount_ugx > 0),
    CONSTRAINT refunds_status_check CHECK (status IN ('REQUESTED', 'PENDING', 'REFUNDED', 'FAILED'))
);

CREATE UNIQUE INDEX refunds_one_open ON refunds (order_id) WHERE status IN ('REQUESTED', 'PENDING');

CREATE TABLE disputes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL UNIQUE REFERENCES orders (id),
    customer_user_id UUID NOT NULL REFERENCES users (id),
    reason TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'OPEN',
    opened_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    escalate_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    resolved_by UUID REFERENCES users (id),
    resolution TEXT NOT NULL DEFAULT '',
    CONSTRAINT disputes_reason_check CHECK (length(trim(reason)) > 0),
    CONSTRAINT disputes_status_check CHECK (status IN ('OPEN', 'RESOLVED'))
);

CREATE TABLE alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vendor_id UUID REFERENCES vendors (id),
    order_id UUID REFERENCES orders (id),
    kind TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
