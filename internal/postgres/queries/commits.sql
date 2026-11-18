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
