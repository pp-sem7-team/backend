-- +goose Up

CREATE TYPE branch_status AS ENUM (
    'active',
    'closed'
);

CREATE TYPE generation_status AS ENUM (
    'pending',
    'done',
    'failed'
);

CREATE TYPE commit_type AS ENUM (
    'feat',
    'fix',
    'refactor',
    'perf',
    'docs',
    'test',
    'style',
    'chore'
);

CREATE TABLE users (
    id UUID PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX refresh_tokens_user_id_idx
    ON refresh_tokens (user_id);

CREATE INDEX refresh_tokens_expires_at_idx
    ON refresh_tokens (expires_at);

CREATE TABLE workspaces (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, name)
);

CREATE TABLE branches (
    id UUID PRIMARY KEY,
    workspace_id UUID NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    status branch_status NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, name)
);

CREATE TABLE changes (
    id UUID PRIMARY KEY,
    branch_id UUID NOT NULL REFERENCES branches (id) ON DELETE CASCADE,
    sequence_number INT NOT NULL,
    raw_diff TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (branch_id, sequence_number)
);

CREATE TABLE analysis_results (
    id UUID PRIMARY KEY,
    change_id UUID NOT NULL UNIQUE REFERENCES changes (id) ON DELETE CASCADE,
    structured_changes JSONB,
    status generation_status NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (status <> 'done' OR structured_changes IS NOT NULL)
);

CREATE TABLE commits (
    id UUID PRIMARY KEY,
    change_id UUID NOT NULL UNIQUE REFERENCES changes (id) ON DELETE CASCADE,
    type commit_type,
    scope TEXT,
    message TEXT,
    is_edited_by_user BOOLEAN NOT NULL DEFAULT false,
    status generation_status NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (status <> 'done' OR (type IS NOT NULL AND message IS NOT NULL))
);

CREATE TABLE pull_requests (
    id UUID PRIMARY KEY,
    branch_id UUID NOT NULL UNIQUE REFERENCES branches (id) ON DELETE CASCADE,
    title TEXT,
    description TEXT,
    is_edited_by_user BOOLEAN NOT NULL DEFAULT false,
    status generation_status NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (status <> 'done' OR (title IS NOT NULL AND description IS NOT NULL))
);

CREATE TABLE outbox (
    id UUID PRIMARY KEY,
    exchange TEXT NOT NULL,
    routing_key TEXT NOT NULL,
    headers JSONB NOT NULL DEFAULT '{}'::jsonb,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);

CREATE INDEX outbox_unpublished_idx
    ON outbox (published_at, created_at);


-- +goose Down

DROP TABLE outbox;
DROP TABLE pull_requests;
DROP TABLE commits;
DROP TABLE analysis_results;
DROP TABLE changes;
DROP TABLE branches;
DROP TABLE workspaces;
DROP TABLE refresh_tokens;
DROP TABLE users;

DROP TYPE commit_type;
DROP TYPE generation_status;
DROP TYPE branch_status;
