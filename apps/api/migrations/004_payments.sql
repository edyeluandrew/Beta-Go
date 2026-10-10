-- Phase 4: start a payment and record a verified provider result.
-- Live MTN and Airtel calls are not in this migration. Refund execution is later.

ALTER TABLE payments
    ADD COLUMN provider TEXT,
    ADD COLUMN payer_phone TEXT NOT NULL DEFAULT '',
    ADD COLUMN provider_reference TEXT,
    ADD COLUMN initiated_at TIMESTAMPTZ,
    ADD COLUMN expires_at TIMESTAMPTZ,
    ADD COLUMN paid_at TIMESTAMPTZ,
    ADD COLUMN late_success BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE payments
    ADD CONSTRAINT payments_provider_check CHECK (provider IS NULL OR provider IN ('mtn', 'airtel'));

CREATE UNIQUE INDEX payments_reference_key ON payments (provider_reference) WHERE provider_reference IS NOT NULL;

CREATE INDEX payments_pending_idx ON payments (status) WHERE status = 'PENDING';

CREATE TABLE payment_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider TEXT NOT NULL,
    event_id TEXT NOT NULL,
    payment_id UUID REFERENCES payments (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, event_id)
);

INSERT INTO platform_settings (key, value_int)
VALUES ('unpaid_expiry_minutes', 15)
ON CONFLICT (key) DO NOTHING;
