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
    payload,
    attempt_count
FROM outbox
WHERE published_at IS NULL
  AND failed_at IS NULL
  AND next_attempt_at <= now()
ORDER BY next_attempt_at, created_at, id
LIMIT sqlc.arg(batch_size);

-- name: MarkOutboxMessagePublished :exec
UPDATE outbox
SET published_at = now(),
    attempt_count = attempt_count + 1,
    last_error = NULL
WHERE id = sqlc.arg(id)
  AND published_at IS NULL
  AND failed_at IS NULL;

-- name: MarkOutboxMessageAttemptFailed :exec
UPDATE outbox
SET attempt_count = attempt_count + 1,
    last_error = sqlc.arg(error_message)::text,
    next_attempt_at = sqlc.arg(next_attempt_at),
    failed_at = sqlc.narg(failed_at)
WHERE id = sqlc.arg(id)
  AND published_at IS NULL
  AND failed_at IS NULL;

-- name: DeleteExpiredOutboxMessages :execrows
DELETE FROM outbox
WHERE published_at < sqlc.arg(cutoff)::timestamptz
   OR failed_at < sqlc.arg(cutoff)::timestamptz;
