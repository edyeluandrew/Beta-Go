-- Customers can open an account with email and a password when they have no Google account.
-- Google sign-in still uses google_sub. A password account leaves that column empty.

ALTER TABLE users ALTER COLUMN google_sub DROP NOT NULL;

ALTER TABLE users ADD COLUMN password_hash TEXT;

ALTER TABLE users ADD CONSTRAINT users_has_sign_in
    CHECK (google_sub IS NOT NULL OR password_hash IS NOT NULL);
