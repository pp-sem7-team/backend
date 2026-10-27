-- +goose Up

ALTER TABLE outbox
    ADD COLUMN message_type TEXT,
    ADD COLUMN resource_id UUID;

UPDATE outbox
SET
    message_type = routing_key,
    resource_id = COALESCE(
        (payload->>'analysis_id')::uuid,
        (payload->>'commit_id')::uuid,
        (payload->>'pull_request_id')::uuid
    );

ALTER TABLE outbox
    ALTER COLUMN message_type SET NOT NULL,
    ALTER COLUMN resource_id SET NOT NULL;

ALTER TABLE outbox
    DROP COLUMN exchange,
    DROP COLUMN routing_key,
    DROP COLUMN headers;

DROP INDEX outbox_unpublished_idx;

CREATE INDEX outbox_unpublished_idx
    ON outbox (created_at, id)
    WHERE published_at IS NULL;

-- +goose Down

DROP INDEX outbox_unpublished_idx;

CREATE INDEX outbox_unpublished_idx
    ON outbox (published_at, created_at);

ALTER TABLE outbox
    ADD COLUMN exchange TEXT,
    ADD COLUMN routing_key TEXT,
    ADD COLUMN headers JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE outbox
SET
    exchange = 'generation.commands',
    routing_key = message_type;

ALTER TABLE outbox
    ALTER COLUMN exchange SET NOT NULL,
    ALTER COLUMN routing_key SET NOT NULL;

ALTER TABLE outbox
    DROP COLUMN message_type,
    DROP COLUMN resource_id;
