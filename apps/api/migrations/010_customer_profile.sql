-- Customer delivery details collected when they create an account.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS phone TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS area TEXT NOT NULL DEFAULT '';
