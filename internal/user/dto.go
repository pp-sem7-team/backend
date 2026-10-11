package user

import "time"

type UserResponse struct {
	ID        string    `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Email     string    `json:"email" example:"user@example.com"`
	CreatedAt time.Time `json:"created_at" example:"2026-01-01T12:00:00Z"`
	UpdatedAt time.Time `json:"updated_at" example:"2026-01-01T12:00:00Z"`
}

type UpdatePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required" example:"old-password"`
	NewPassword     string `json:"new_password" binding:"required" example:"new-password"`
}
