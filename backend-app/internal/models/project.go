package models

import "time"

type Project struct {
	ID          int64      `json:"id"`
	OwnerID     int64      `json:"owner_id"`
	Name        string     `json:"project_name"`
	Description *string    `json:"description,omitempty"`
	DeadlineAt  *time.Time `json:"deadline_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type ProjectRole string

const (
	ProjectRoleOwner  ProjectRole = "owner"
	ProjectRoleMember ProjectRole = "member"
)

type ProjectMember struct {
	ID        int64       `json:"id"`
	ProjectID int64       `json:"project_id"`
	UserID    int64       `json:"user_id"`
	Role      ProjectRole `json:"role"`
	CreatedAt time.Time   `json:"created_at"`
}

type ProjectMemberWithUser struct {
	ID        int64       `json:"id"`
	ProjectID int64       `json:"project_id"`
	UserID    int64       `json:"user_id"`
	Role      ProjectRole `json:"role"`
	CreatedAt time.Time   `json:"created_at"`
	FullName  string      `json:"full_name"`
	Email     string      `json:"email"`
	Position  string      `json:"position"`
	AvatarURL *string     `json:"avatar_url,omitempty"`
}

type ProjectInvite struct {
	ID        int64     `json:"id"`
	ProjectID int64     `json:"project_id"`
	Email     string    `json:"email"`
	Token     string    `json:"token"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ProjectWithStats struct {
	Project
	MemberCount int `json:"member_count"`
	TaskCount   int `json:"task_count"`
}
