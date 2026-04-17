package models

import "time"

type Event struct {
	ID             int        `json:"id"`
	ProjectID      int        `json:"project_id"`
	Title          string     `json:"title"`
	MeetingURL     *string    `json:"meeting_url,omitempty"`
	Description    *string    `json:"description,omitempty"`
	StartAt        time.Time  `json:"start_at"`
	EndAt          *time.Time `json:"end_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ParticipantIDs []int      `json:"participant_ids"`
}
