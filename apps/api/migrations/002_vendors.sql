-- Phase 2: vendor onboarding, shop content, and an empty commission account.
-- Pool top-ups, reservations, and captures arrive in a later migration.

CREATE TABLE vendors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_name TEXT NOT NULL,
    contact_name TEXT NOT NULL,
    contact_email TEXT NOT NULL,
    phone TEXT NOT NULL,
    address TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    logo_url TEXT,
    shop_photo_url TEXT,
    status TEXT NOT NULL DEFAULT 'PENDING_APPROVAL',
    is_open BOOLEAN NOT NULL DEFAULT false,
    accepting_orders BOOLEAN NOT NULL DEFAULT false,
    delivery_fee_ugx INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT vendors_status_check CHECK (status IN (
        'PENDING_APPROVAL', 'REJECTED', 'ACTIVE', 'INACTIVE', 'PAUSED', 'SUSPENDED'
    )),
    CONSTRAINT vendors_delivery_fee_check CHECK (delivery_fee_ugx >= 0),
    CONSTRAINT vendors_names_not_blank CHECK (
        length(trim(business_name)) > 0
        AND length(trim(contact_name)) > 0
        AND length(trim(contact_email)) > 0
        AND length(trim(phone)) > 0
        AND length(trim(address)) > 0
    )
);

CREATE TABLE vendor_users (
    vendor_id UUID NOT NULL REFERENCES vendors (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (vendor_id, user_id)
);

CREATE UNIQUE INDEX vendor_users_one_shop_idx ON vendor_users (user_id);

CREATE TABLE vendor_invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vendor_id UUID NOT NULL REFERENCES vendors (id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE vendor_payment_destinations (
    vendor_id UUID PRIMARY KEY REFERENCES vendors (id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    account TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'UNVERIFIED',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT vendor_destination_provider_check CHECK (provider IN ('mtn', 'airtel')),
    CONSTRAINT vendor_destination_status_check CHECK (status IN (
        'UNVERIFIED', 'PENDING_VERIFICATION', 'VERIFIED', 'FAILED', 'DISABLED'
    )),
    CONSTRAINT vendor_destination_account_check CHECK (length(trim(account)) > 0)
);

CREATE TABLE vendor_hours (
    vendor_id UUID NOT NULL REFERENCES vendors (id) ON DELETE CASCADE,
    weekday SMALLINT NOT NULL,
    opens_at TEXT,
    closes_at TEXT,
    closed BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY (vendor_id, weekday),
    CONSTRAINT vendor_hours_weekday_check CHECK (weekday BETWEEN 0 AND 6)
);

CREATE TABLE menu_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vendor_id UUID NOT NULL REFERENCES vendors (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT menu_categories_name_check CHECK (length(trim(name)) > 0)
);

CREATE TABLE menu_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vendor_id UUID NOT NULL REFERENCES vendors (id) ON DELETE CASCADE,
    category_id UUID REFERENCES menu_categories (id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    price_ugx INTEGER NOT NULL,
    available BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT menu_items_name_check CHECK (length(trim(name)) > 0),
    CONSTRAINT menu_items_price_check CHECK (price_ugx > 0)
);

CREATE TABLE commission_accounts (
    vendor_id UUID PRIMARY KEY REFERENCES vendors (id) ON DELETE CASCADE,
    available_ugx INTEGER NOT NULL DEFAULT 0,
    reserved_ugx INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT commission_available_check CHECK (available_ugx >= 0),
    CONSTRAINT commission_reserved_check CHECK (reserved_ugx >= 0)
);

CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id UUID REFERENCES users (id),
    action TEXT NOT NULL,
    entity_id UUID,
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
