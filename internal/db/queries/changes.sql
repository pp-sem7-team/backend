-- name: LockBranchForChange :one
SELECT
    b.id,
    b.status
FROM branches b
JOIN workspaces w ON w.id = b.workspace_id
WHERE b.id = sqlc.arg(branch_id)
  AND w.user_id = sqlc.arg(user_id)
FOR UPDATE OF b;


-- name: CreateChange :exec
INSERT INTO changes (
    id,
    branch_id,
    sequence_number,
    raw_diff
)
VALUES (
    sqlc.arg(id),
    sqlc.arg(branch_id),
    (
        SELECT COALESCE(MAX(sequence_number), 0) + 1
        FROM changes
        WHERE branch_id = sqlc.arg(branch_id)
    ),
    sqlc.arg(raw_diff)
);

-- name: GetChange :one
SELECT
    c.id,
    c.branch_id,
    c.sequence_number,
    c.created_at,
    ar.id AS analysis_id,
    ar.structured_changes,
    ar.status AS analysis_status,
    ar.created_at AS analysis_created_at,
    ar.updated_at AS analysis_updated_at
FROM changes c
JOIN branches b ON b.id = c.branch_id
JOIN workspaces w ON w.id = b.workspace_id
JOIN analysis_results ar ON ar.change_id = c.id
WHERE c.id = sqlc.arg(id)
  AND w.user_id = sqlc.arg(user_id);

-- name: ListChangesByBranch :many
SELECT
    c.id,
    c.branch_id,
    c.sequence_number,
    c.created_at,
    ar.id AS analysis_id,
    ar.structured_changes,
    ar.status AS analysis_status,
    ar.created_at AS analysis_created_at,
    ar.updated_at AS analysis_updated_at
FROM changes c
JOIN branches b ON b.id = c.branch_id
JOIN workspaces w ON w.id = b.workspace_id
JOIN analysis_results ar ON ar.change_id = c.id
WHERE b.id = sqlc.arg(branch_id)
  AND w.user_id = sqlc.arg(user_id)
ORDER BY c.sequence_number ASC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: CountChangesByBranch :one
SELECT COUNT(*)::bigint
FROM changes c
JOIN branches b ON b.id = c.branch_id
JOIN workspaces w ON w.id = b.workspace_id
WHERE b.id = sqlc.arg(branch_id)
  AND w.user_id = sqlc.arg(user_id);
