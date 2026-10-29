-- name: CreateOutboxMessage :exec
INSERT INTO outbox (
    id,
    message_type,
    payload
)
VALUES (
    sqlc.arg(id),
    sqlc.arg(message_type),
    sqlc.arg(payload)
);

-- name: ListUnpublishedOutboxMessages :many
SELECT
    id,
    message_type,
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

-- name: DeletePublishedOutboxMessages :execrows
DELETE FROM outbox
WHERE published_at IS NOT NULL
  AND published_at < now() - (sqlc.arg(retention_ms)::bigint * interval '1 millisecond');
