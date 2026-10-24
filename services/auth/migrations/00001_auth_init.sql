-- +goose Up
-- Auth owns identities only; financial profiles and applications live in client_db.
CREATE TABLE organizations (
    id uuid PRIMARY KEY,
    inn text NOT NULL UNIQUE CHECK (inn ~ '^[0-9]{10}([0-9]{2})?$'),
    name text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 256),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (updated_at >= created_at)
);

CREATE TABLE users (
    id uuid PRIMARY KEY,
    email text NOT NULL CHECK (
        email = btrim(email) AND char_length(email) BETWEEN 3 AND 254 AND
        email ~ '^[^[:space:]@]+@[^[:space:]@]+\.[^[:space:]@]+$'
    ),
    full_name text NOT NULL CHECK (char_length(btrim(full_name)) BETWEEN 1 AND 256),
    -- PHC string: Argon2id v19, cost parameters, 16-byte salt, 32-byte result.
    password_hash text NOT NULL CHECK (
        password_hash ~ '^\$argon2id\$v=19\$m=[1-9][0-9]*,t=[1-9][0-9]*,p=[1-9][0-9]*\$[A-Za-z0-9+/]{22}\$[A-Za-z0-9+/]{43}$'
    ),
    role text NOT NULL CHECK (role IN ('CLIENT', 'ANALYST', 'ADMIN')),
    org_id uuid REFERENCES organizations (id),
    can_submit boolean NOT NULL DEFAULT false,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((role = 'CLIENT' AND org_id IS NOT NULL) OR
           (role IN ('ANALYST', 'ADMIN') AND org_id IS NULL)),
    CHECK (role = 'CLIENT' OR NOT can_submit),
    CHECK (updated_at >= created_at)
);

CREATE UNIQUE INDEX users_email_ci_idx ON users (lower(email));
CREATE INDEX users_org_idx ON users (org_id) WHERE org_id IS NOT NULL;

-- Store SHA-256 of a cryptographically random refresh token, never the token.
-- Refresh does not rotate the token or extend its original lifetime.
CREATE TABLE refresh_tokens (
    token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    user_id uuid NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    CHECK (expires_at > created_at AND expires_at <= created_at + interval '7 days'),
    CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);

CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id);
CREATE INDEX refresh_tokens_expires_idx ON refresh_tokens (expires_at);

-- +goose Down
DROP TABLE refresh_tokens;
DROP TABLE users;
DROP TABLE organizations;
