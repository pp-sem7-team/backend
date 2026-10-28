-- name: CreateAnalysisResult :one
INSERT INTO analysis_results (
    id,
    change_id,
    status
)
VALUES (
    sqlc.arg(id),
    sqlc.arg(change_id),
    'pending'
)
RETURNING
    id,
    structured_changes,
    status,
    created_at,
    updated_at;

-- name: ApplyAnalysisResult :execrows
UPDATE analysis_results ar
SET
    structured_changes = sqlc.narg(structured_changes)::jsonb,
    status = sqlc.arg(status)::generation_status,
    updated_at = now()
WHERE ar.id = sqlc.arg(analysis_id)
  AND ar.status = 'pending'
  AND EXISTS (
      SELECT 1
      FROM outbox o
      WHERE o.id = sqlc.arg(message_id)
        AND o.message_type = 'analysis.request'
        AND o.resource_id = ar.id
  );

-- name: FailStalePendingAnalyses :execrows
UPDATE analysis_results
SET
    status = 'failed',
    updated_at = now()
WHERE status = 'pending'
  AND created_at < now() - (sqlc.arg(timeout_ms)::bigint * interval '1 millisecond');
