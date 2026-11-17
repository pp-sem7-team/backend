-- +goose Up

ALTER TABLE outbox
    ADD COLUMN attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    ADD COLUMN next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN failed_at TIMESTAMPTZ,
    ADD COLUMN last_error TEXT;

DROP INDEX outbox_unpublished_idx;

CREATE INDEX outbox_unpublished_idx
    ON outbox (next_attempt_at, created_at, id)
    WHERE published_at IS NULL AND failed_at IS NULL;

CREATE INDEX outbox_failed_at_idx
    ON outbox (failed_at)
    WHERE failed_at IS NOT NULL;

-- +goose Down

DROP INDEX outbox_failed_at_idx;

DROP INDEX outbox_unpublished_idx;

ALTER TABLE outbox
    DROP COLUMN attempt_count,
    DROP COLUMN next_attempt_at,
    DROP COLUMN failed_at,
    DROP COLUMN last_error;

CREATE INDEX outbox_unpublished_idx
    ON outbox (created_at, id)
    WHERE published_at IS NULL;
