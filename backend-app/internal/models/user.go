package models

import "time"

type User struct {
	ID        int64     `json:"id"`
	FullName  string    `json:"full_name"`
	AvatarURL *string   `json:"avatar_url,omitempty"`
	Email     string    `json:"email"`
	Position  *string   `json:"position"`
	BirthDate *string   `json:"birth_date,omitempty"`
	About     *string   `json:"about,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TaskStats struct {
	Done    int `json:"done"`
	Overdue int `json:"overdue"`
}
