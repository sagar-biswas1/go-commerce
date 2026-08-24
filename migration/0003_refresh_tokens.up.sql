-- Refresh tokens.
--
-- Only a SHA-256 hash of the token is stored: whoever reads this table cannot
-- replay what they find, the same reason a password column holds a bcrypt
-- digest and not a password.
--
-- family_id groups every token descended from one login. Rotation revokes the
-- presented token and issues its successor in the same family; if a revoked
-- token is ever presented again, the token was stolen, and the whole family is
-- revoked rather than just that one row.
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    family_id   UUID NOT NULL,
    token_hash  TEXT NOT NULL,
    user_agent  TEXT,
    ip_address  TEXT,
    expires_at  TIMESTAMP WITH TIME ZONE NOT NULL,
    revoked_at  TIMESTAMP WITH TIME ZONE,
    replaced_by UUID REFERENCES refresh_tokens (id) ON DELETE SET NULL,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS refresh_tokens_hash_key ON refresh_tokens (token_hash);
CREATE INDEX IF NOT EXISTS refresh_tokens_user_idx ON refresh_tokens (user_id);
CREATE INDEX IF NOT EXISTS refresh_tokens_family_idx ON refresh_tokens (family_id);
-- The cleanup janitor sweeps by expiry, so give it an index to sweep along.
CREATE INDEX IF NOT EXISTS refresh_tokens_expires_at_idx ON refresh_tokens (expires_at);
