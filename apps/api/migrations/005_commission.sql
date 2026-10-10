-- Phase 5: prepaid commission pool for Model A.
-- Food payments never write these tables. Accept still lives in phase 6.

INSERT INTO platform_settings (key, value_int) VALUES
    ('commission_activity_floor_ugx', 10000),
    ('commission_low_balance_ugx', 20000),
    ('commission_topup_min_ugx', 10000),
    ('commission_topup_max_ugx', 1000000),
    ('commission_adjustment_dual_approve_ugx', 50000)
ON CONFLICT (key) DO NOTHING;

CREATE TABLE commission_ledger (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vendor_id UUID NOT NULL REFERENCES vendors (id),
    stream TEXT NOT NULL,
    direction TEXT NOT NULL,
    amount_ugx INTEGER NOT NULL,
    operation TEXT NOT NULL,
    operation_id UUID NOT NULL,
    order_id UUID REFERENCES orders (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT commission_ledger_stream_check CHECK (stream IN ('available', 'reserved', 'revenue', 'external')),
    CONSTRAINT commission_ledger_direction_check CHECK (direction IN ('debit', 'credit')),
    CONSTRAINT commission_ledger_amount_check CHECK (amount_ugx > 0),
    CONSTRAINT commission_ledger_operation_check CHECK (operation IN ('TOP_UP', 'RESERVE', 'CAPTURE', 'RELEASE', 'ADJUSTMENT')),
    UNIQUE (operation, operation_id, stream, direction)
);

CREATE TABLE commission_top_ups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vendor_id UUID NOT NULL REFERENCES vendors (id),
    amount_ugx INTEGER NOT NULL,
    provider TEXT NOT NULL,
    payer_phone TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING',
    provider_reference TEXT,
    idempotency_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT commission_topups_amount_check CHECK (amount_ugx > 0),
    CONSTRAINT commission_topups_provider_check CHECK (provider IN ('mtn', 'airtel')),
    CONSTRAINT commission_topups_status_check CHECK (status IN ('PENDING', 'PAID', 'FAILED', 'EXPIRED')),
    UNIQUE (vendor_id, idempotency_key)
);

CREATE UNIQUE INDEX commission_top_ups_reference_key
    ON commission_top_ups (provider_reference)
    WHERE provider_reference IS NOT NULL;

CREATE TABLE commission_adjustments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vendor_id UUID NOT NULL REFERENCES vendors (id),
    amount_ugx INTEGER NOT NULL,
    reason TEXT NOT NULL,
    requested_by UUID NOT NULL REFERENCES users (id),
    approved_by UUID REFERENCES users (id),
    status TEXT NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT commission_adjustments_amount_check CHECK (amount_ugx <> 0),
    CONSTRAINT commission_adjustments_reason_check CHECK (length(trim(reason)) > 0),
    CONSTRAINT commission_adjustments_status_check CHECK (status IN ('PENDING', 'APPLIED', 'REJECTED'))
);

CREATE TABLE commission_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider TEXT NOT NULL,
    event_id TEXT NOT NULL,
    top_up_id UUID REFERENCES commission_top_ups (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, event_id)
);
