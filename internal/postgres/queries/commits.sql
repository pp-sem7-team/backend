-- name: CreateCommit :one
INSERT INTO commits (
    id,
    change_id,
    message_id
)
VALUES (
    sqlc.arg(id),
    sqlc.arg(change_id),
    sqlc.arg(message_id)
)
RETURNING
    id,
    status,
    created_at,
    updated_at;

-- name: GetPendingCommitByChange :one
SELECT id, message_id
FROM commits
WHERE change_id = sqlc.arg(change_id)
  AND status = 'pending';

-- name: FailPendingCommitByChange :exec
UPDATE commits
SET status = 'failed',
    updated_at = now()
WHERE change_id = sqlc.arg(change_id)
  AND status = 'pending';
