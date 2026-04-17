package dto

import (
	"groupeak/internal/models"
	"time"
)

type CreateTaskRequest struct {
	TaskName        string              `json:"task_name"`
	TaskDescription *string             `json:"task_description"`
	DueDate         *time.Time          `json:"due_date"`
	Status          models.TaskStatus   `json:"status"`
	Priority        models.TaskPriority `json:"priority"`
	AssigneeIDs     []int               `json:"assignee_ids"`
}

type PatchTaskRequest struct {
	Title           *string              `json:"title,omitempty"`
	TaskName        *string              `json:"task_name,omitempty"`
	Description     *string              `json:"description,omitempty"`
	TaskDescription *string              `json:"task_description,omitempty"`
	Comments        *string              `json:"comments,omitempty"`
	Status          *models.TaskStatus   `json:"status,omitempty"`
	Priority        *models.TaskPriority `json:"priority,omitempty"`
	AssigneeIDs     *[]int64             `json:"assignee_ids,omitempty"`
	DueAt           *time.Time           `json:"due_at,omitempty"`
	DueDate         *time.Time           `json:"due_date,omitempty"`
	ClearDueAt      bool                 `json:"clear_due_at,omitempty"`
}

type TaskFilterRequest struct {
	ProjectID *int64               `json:"project_id,omitempty"`
	Priority  *models.TaskPriority `json:"priority,omitempty"`
}

type ReviewTaskRequest struct {
	Decision string  `json:"decision"`
	Comment  *string `json:"comment"`
	DueAt    *string `json:"due_at"`
}
