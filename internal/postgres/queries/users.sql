-- name: CreateUser :exec
INSERT INTO users (
    id,
    email,
    password_hash
)
VALUES (
    sqlc.arg(id),
    sqlc.arg(email),
    sqlc.arg(password_hash)
);

-- name: GetUser :one
SELECT
    id,
    email,
    created_at,
    updated_at
FROM users
WHERE id = sqlc.arg(user_id);

-- name: GetUserCredentialsByEmail :one
SELECT
    id,
    password_hash
FROM users
WHERE email = sqlc.arg(email);

-- name: GetUserPasswordHash :one
SELECT password_hash
FROM users
WHERE id = sqlc.arg(user_id);

-- name: UpdateUserPassword :execrows
UPDATE users
SET
    password_hash = sqlc.arg(password_hash),
    updated_at = now()
WHERE id = sqlc.arg(user_id);
