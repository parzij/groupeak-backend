package dto

import "time"

type CreateProjectRequest struct {
	Name        string     `json:"name"`
	Description *string    `json:"description"`
	DeadlineAt  *time.Time `json:"deadline_at"`
}

type PatchProjectRequest struct {
	Name            *string    `json:"name,omitempty"`
	ProjectName     *string    `json:"project_name,omitempty"`
	Description     *string    `json:"description,omitempty"`
	DeadlineAt      *time.Time `json:"deadline_at,omitempty"`
	ClearDeadlineAt bool       `json:"clear_deadline_at,omitempty"`
}

type InviteMemberRequest struct {
	Email string `json:"email"`
}
