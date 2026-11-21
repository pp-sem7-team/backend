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

-- name: LockCommit :one
SELECT
    cm.id,
    cm.status,
    ar.status AS analysis_status,
    ar.structured_changes
FROM commits cm
JOIN changes c ON c.id = cm.change_id
JOIN branches b ON b.id = c.branch_id
JOIN workspaces w ON w.id = b.workspace_id
JOIN analysis_results ar ON ar.change_id = c.id
WHERE cm.change_id = sqlc.arg(change_id)
  AND w.user_id = sqlc.arg(user_id)
FOR UPDATE OF cm;

-- name: ResetCommit :one
UPDATE commits
SET status = 'pending',
    type = NULL,
    scope = NULL,
    message = NULL,
    is_edited_by_user = false,
    message_id = sqlc.arg(message_id),
    updated_at = now()
WHERE id = sqlc.arg(commit_id)
RETURNING id, status, created_at, updated_at;

-- name: UpdateCommitMessage :one
UPDATE commits
SET type = sqlc.arg(type)::commit_type,
    scope = sqlc.narg(scope)::text,
    message = sqlc.arg(message)::text,
    is_edited_by_user = true,
    updated_at = now()
WHERE id = sqlc.arg(commit_id)
RETURNING id, type, scope, message, is_edited_by_user, status, created_at, updated_at;

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
