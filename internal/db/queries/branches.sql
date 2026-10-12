-- name: CreateBranch :one
INSERT INTO branches (
    id,
    workspace_id,
    name
)
SELECT
    sqlc.arg(id),
    w.id,
    sqlc.arg(name)
FROM workspaces w
WHERE w.id = sqlc.arg(workspace_id)
  AND w.user_id = sqlc.arg(user_id)
RETURNING
    id,
    workspace_id,
    name,
    status,
    created_at,
    updated_at;

-- name: GetBranch :one
SELECT
    b.id,
    b.workspace_id,
    b.name,
    b.status,
    b.created_at,
    b.updated_at
FROM branches b
JOIN workspaces w ON w.id = b.workspace_id
WHERE b.id = sqlc.arg(id)
  AND w.user_id = sqlc.arg(user_id);

-- name: ListBranches :many
SELECT
    b.id,
    b.workspace_id,
    b.name,
    b.status,
    b.created_at,
    b.updated_at
FROM branches b
JOIN workspaces w ON w.id = b.workspace_id
WHERE w.id = sqlc.arg(workspace_id)
  AND w.user_id = sqlc.arg(user_id)
  AND (
      sqlc.narg(status)::branch_status IS NULL
      OR b.status = sqlc.narg(status)::branch_status
  )
ORDER BY b.created_at DESC, b.id
LIMIT sqlc.arg(page_limit)::int
OFFSET sqlc.arg(page_offset)::int;

-- name: CountBranches :one
SELECT COUNT(*)
FROM branches b
JOIN workspaces w ON w.id = b.workspace_id
WHERE w.id = sqlc.arg(workspace_id)
  AND w.user_id = sqlc.arg(user_id)
  AND (
      sqlc.narg(status)::branch_status IS NULL
      OR b.status = sqlc.narg(status)::branch_status
  );

-- name: UpdateBranch :one
UPDATE branches b
SET
    name = COALESCE(sqlc.narg(name), b.name),
    status = COALESCE(sqlc.narg(status)::branch_status, b.status),
    updated_at = now()
FROM workspaces w
WHERE b.id = sqlc.arg(id)
  AND w.id = b.workspace_id
  AND w.user_id = sqlc.arg(user_id)
RETURNING
    b.id,
    b.workspace_id,
    b.name,
    b.status,
    b.created_at,
    b.updated_at;

-- name: DeleteBranch :execrows
DELETE FROM branches b
USING workspaces w
WHERE b.id = sqlc.arg(id)
  AND w.id = b.workspace_id
  AND w.user_id = sqlc.arg(user_id);
