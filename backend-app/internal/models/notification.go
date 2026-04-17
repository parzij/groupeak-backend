package models

import (
	"encoding/json"
	"time"
)

type EventType string

const (
	EventTypeTaskCreated    EventType = "task_created"
	EventTypeTaskUpdated    EventType = "task_updated"
	EventTypeTaskDeleted    EventType = "task_deleted"
	EventTypeTaskReviewed   EventType = "task_reviewed"
	EventTypeProjectMember  EventType = "project_member_added"
	EventTypeProjectKicked  EventType = "project_member_removed"
	EventTypeEventCreated   EventType = "event_created"
	EventTypeEventUpdated   EventType = "event_updated"
	EventTypeEventDeleted   EventType = "event_deleted"
	EventTypeProjectUpdated EventType = "project_updated"
	EventTypeProjectDeleted EventType = "project_deleted"
	EventTypeTaskUnassigned EventType = "task_unassigned"
	EventTypeTaskDeadline   EventType = "task_deadline"
	EventTypeEventReminder  EventType = "event_reminder"
)

type Notification struct {
	ID        int64           `json:"id"`
	ProjectID int64           `json:"project_id"`
	UserID    int64           `json:"user_id"`
	ActorID   *int64          `json:"actor_id,omitempty"`
	Type      EventType       `json:"type"`
	EntityID  *int64          `json:"entity_id,omitempty"`
	Payload   json.RawMessage `json:"payload"`
	IsRead    bool            `json:"is_read"` // false = unread, true = read
	CreatedAt time.Time       `json:"created_at"`
}
