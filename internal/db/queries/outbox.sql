-- name: CreateOutboxMessage :exec
INSERT INTO outbox (
    id,
    exchange,
    routing_key,
    headers,
    payload
)
VALUES (
    sqlc.arg(id),
    sqlc.arg(exchange),
    sqlc.arg(routing_key),
    sqlc.arg(headers),
    sqlc.arg(payload)
);

-- name: ListUnpublishedOutboxMessages :many
SELECT
    id,
    exchange,
    routing_key,
    payload
FROM outbox
WHERE published_at IS NULL
ORDER BY created_at, id
LIMIT sqlc.arg(batch_size);

-- name: MarkOutboxMessagePublished :exec
UPDATE outbox
SET published_at = now()
WHERE id = sqlc.arg(id)
  AND published_at IS NULL;
