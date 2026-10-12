-- name: CreateWorkspace :one
INSERT INTO workspaces (
    id,
    user_id,
    name
)
VALUES (
    sqlc.arg(id),
    sqlc.arg(user_id),
    sqlc.arg(name)
)
RETURNING
    id,
    name,
    created_at,
    updated_at;

-- name: GetWorkspace :one
SELECT
    id,
    name,
    created_at,
    updated_at
FROM workspaces
WHERE id = sqlc.arg(id)
  AND user_id = sqlc.arg(user_id);

-- name: WorkspaceExists :one
SELECT EXISTS (
    SELECT 1
    FROM workspaces
    WHERE id = sqlc.arg(id)
      AND user_id = sqlc.arg(user_id)
);

-- name: ListWorkspaces :many
SELECT
    id,
    name,
    created_at,
    updated_at
FROM workspaces
WHERE user_id = sqlc.arg(user_id)
ORDER BY created_at DESC, id
LIMIT sqlc.arg(page_limit)::int
OFFSET sqlc.arg(page_offset)::int;

-- name: CountWorkspaces :one
SELECT COUNT(*)
FROM workspaces
WHERE user_id = sqlc.arg(user_id);

-- name: UpdateWorkspace :one
UPDATE workspaces
SET
    name = sqlc.arg(name),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND user_id = sqlc.arg(user_id)
RETURNING
    id,
    name,
    created_at,
    updated_at;

-- name: DeleteWorkspace :execrows
DELETE FROM workspaces
WHERE id = sqlc.arg(id)
  AND user_id = sqlc.arg(user_id);
