-- name: CreateAnalysisResult :one
INSERT INTO analysis_results (
    id,
    change_id,
    message_id,
    status
)
VALUES (
    sqlc.arg(id),
    sqlc.arg(change_id),
    sqlc.arg(message_id),
    'pending'
)
RETURNING
    id,
    structured_changes,
    status,
    created_at,
    updated_at;

-- name: ApplyAnalysisResult :one
UPDATE analysis_results ar
SET
    structured_changes = sqlc.narg(structured_changes)::jsonb,
    status = sqlc.arg(status)::generation_status,
    updated_at = now()
WHERE ar.id = sqlc.arg(analysis_id)
  AND ar.status = 'pending'
  AND ar.message_id = sqlc.arg(message_id)
RETURNING ar.change_id;

-- name: FailStalePendingAnalyses :one
WITH stale_analyses AS (
    UPDATE analysis_results
    SET status = 'failed',
        updated_at = now()
    WHERE status = 'pending'
      AND created_at < sqlc.arg(cutoff)::timestamptz
    RETURNING change_id
), failed_commits AS (
    UPDATE commits
    SET status = 'failed',
        updated_at = now()
    WHERE status = 'pending'
      AND change_id IN (SELECT change_id FROM stale_analyses)
)
SELECT COUNT(*)::bigint FROM stale_analyses;
