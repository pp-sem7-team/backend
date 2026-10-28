-- +goose Up

ALTER TABLE analysis_results
    ADD COLUMN message_id UUID NOT NULL;

ALTER TABLE commits
    ADD COLUMN message_id UUID NOT NULL;

ALTER TABLE pull_requests
    ADD COLUMN message_id UUID NOT NULL;

ALTER TABLE outbox
    ADD COLUMN message_type TEXT NOT NULL;

ALTER TABLE outbox
    DROP COLUMN exchange,
    DROP COLUMN routing_key,
    DROP COLUMN headers;

DROP INDEX outbox_unpublished_idx;

CREATE INDEX outbox_unpublished_idx
    ON outbox (created_at, id)
    WHERE published_at IS NULL;

CREATE INDEX outbox_published_at_idx
    ON outbox (published_at)
    WHERE published_at IS NOT NULL;

-- +goose Down

DROP INDEX outbox_published_at_idx;

DROP INDEX outbox_unpublished_idx;

ALTER TABLE outbox
    ADD COLUMN exchange TEXT NOT NULL DEFAULT 'generation.commands',
    ADD COLUMN routing_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN headers JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE outbox
SET routing_key = message_type;

ALTER TABLE outbox
    ALTER COLUMN exchange DROP DEFAULT,
    ALTER COLUMN routing_key DROP DEFAULT,
    DROP COLUMN message_type;

CREATE INDEX outbox_unpublished_idx
    ON outbox (published_at, created_at);

ALTER TABLE pull_requests
    DROP COLUMN message_id;

ALTER TABLE commits
    DROP COLUMN message_id;

ALTER TABLE analysis_results
    DROP COLUMN message_id;
