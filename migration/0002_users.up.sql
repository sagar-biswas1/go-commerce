-- Users. Replaces the in-memory database.UserStore.
--
-- Identity is a UUID, matching products: ids are handed to clients, and a
-- sequential integer tells a client how many users exist and lets it walk them.
CREATE TABLE IF NOT EXISTS users (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email             TEXT NOT NULL,
    password_hash     TEXT NOT NULL,
    first_name        TEXT NOT NULL,
    last_name         TEXT NOT NULL,
    avatar_url        TEXT,
    phone_number      TEXT,
    role              TEXT NOT NULL DEFAULT 'user'
                          CHECK (role IN ('admin', 'user', 'staff')),
    status            TEXT NOT NULL DEFAULT 'active'
                          CHECK (status IN ('active', 'inactive', 'suspended', 'pending')),
    is_email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    last_login_at     TIMESTAMP WITH TIME ZONE,
    created_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at        TIMESTAMP WITH TIME ZONE
);

-- Email is compared case-insensitively, and a soft-deleted row must not block
-- the address forever, so uniqueness is a partial index on the lowered value
-- rather than a plain UNIQUE constraint.
CREATE UNIQUE INDEX IF NOT EXISTS users_email_lower_key
    ON users (lower(email))
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS users_created_at_idx ON users (created_at DESC);
