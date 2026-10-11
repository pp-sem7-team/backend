-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (
    id,
    user_id,
    token_hash,
    expires_at
)
VALUES (
    sqlc.arg(id),
    sqlc.arg(user_id),
    sqlc.arg(token_hash),
    sqlc.arg(expires_at)
);

-- name: GetUserIDByRefreshTokenHash :one
SELECT user_id
FROM refresh_tokens
WHERE token_hash = sqlc.arg(token_hash)
  AND expires_at > now();

-- name: RotateRefreshToken :execrows
UPDATE refresh_tokens
SET token_hash = sqlc.arg(new_token_hash),
    expires_at = sqlc.arg(expires_at)
WHERE token_hash = sqlc.arg(old_token_hash)
  AND expires_at > now();

-- name: DeleteRefreshTokenByHash :exec
DELETE FROM refresh_tokens
WHERE token_hash = sqlc.arg(token_hash);

-- name: DeleteExpiredRefreshTokens :execrows
DELETE FROM refresh_tokens
WHERE expires_at <= now();
