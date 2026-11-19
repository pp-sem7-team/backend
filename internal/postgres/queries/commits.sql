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

-- name: ApplyCommitResult :execrows
UPDATE commits
SET status = sqlc.arg(status)::generation_status,
    type = sqlc.narg(type)::commit_type,
    scope = sqlc.narg(scope)::text,
    message = sqlc.narg(message)::text,
    updated_at = now()
WHERE id = sqlc.arg(commit_id)
  AND status = 'pending'
  AND message_id = sqlc.arg(message_id);

-- name: FailStalePendingCommits :execrows
UPDATE commits c
SET status = 'failed',
    updated_at = now()
FROM outbox o
WHERE c.status = 'pending'
  AND c.message_id = o.id
  AND o.message_type = 'commit.request'
  AND COALESCE(o.published_at, o.created_at) < sqlc.arg(cutoff)::timestamptz;

-- name: FailPendingCommitByChange :exec
UPDATE commits
SET status = 'failed',
    updated_at = now()
WHERE change_id = sqlc.arg(change_id)
  AND status = 'pending';
