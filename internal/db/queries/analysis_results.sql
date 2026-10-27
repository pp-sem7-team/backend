-- name: CreateAnalysisResult :exec
INSERT INTO analysis_results (
    id,
    change_id,
    status
)
VALUES (
    sqlc.arg(id),
    sqlc.arg(change_id),
    'pending'
);

-- name: UpdateAnalysisResult :one
UPDATE analysis_results
SET
    structured_changes = sqlc.narg(structured_changes)::jsonb,
    status = sqlc.arg(status)::generation_status,
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING
    id,
    change_id,
    structured_changes,
    status,
    created_at,
    updated_at;
