-- name: UpsertPendingPullRequest :one
INSERT INTO pull_requests (
    id,
    branch_id,
    message_id
)
VALUES (
    sqlc.arg(id),
    sqlc.arg(branch_id),
    sqlc.arg(message_id)
)
ON CONFLICT (branch_id) DO UPDATE
SET title = NULL,
    description = NULL,
    is_edited_by_user = false,
    status = 'pending',
    message_id = EXCLUDED.message_id,
    updated_at = now()
WHERE pull_requests.status <> 'pending'
RETURNING
    id,
    branch_id,
    title,
    description,
    is_edited_by_user,
    status,
    created_at,
    updated_at;

-- name: GetPullRequest :one
SELECT
    pr.id,
    pr.branch_id,
    pr.title,
    pr.description,
    pr.is_edited_by_user,
    pr.status,
    pr.created_at,
    pr.updated_at
FROM pull_requests pr
JOIN branches b ON b.id = pr.branch_id
JOIN workspaces w ON w.id = b.workspace_id
WHERE pr.branch_id = sqlc.arg(branch_id)
  AND w.user_id = sqlc.arg(user_id);

-- name: PullRequestExists :one
SELECT EXISTS (
    SELECT 1
    FROM pull_requests
    WHERE branch_id = sqlc.arg(branch_id)
);

-- name: ListPullRequestChanges :many
SELECT
    c.sequence_number,
    (ar.id IS NOT NULL)::boolean AS analysis_done,
    ar.structured_changes,
    (cm.id IS NOT NULL)::boolean AS commit_done,
    cm.type AS commit_type,
    cm.scope AS commit_scope,
    cm.message AS commit_message
FROM changes c
LEFT JOIN analysis_results ar ON ar.change_id = c.id
    AND ar.status = 'done'
LEFT JOIN commits cm ON cm.change_id = c.id
    AND cm.status = 'done'
WHERE c.branch_id = sqlc.arg(branch_id)
ORDER BY c.sequence_number;

-- name: EditPullRequest :one
UPDATE pull_requests
SET title = sqlc.arg(title)::text,
    description = sqlc.arg(description)::text,
    is_edited_by_user = true,
    updated_at = now()
WHERE branch_id = sqlc.arg(branch_id)
  AND status = 'done'
RETURNING
    id,
    branch_id,
    title,
    description,
    is_edited_by_user,
    status,
    created_at,
    updated_at;
