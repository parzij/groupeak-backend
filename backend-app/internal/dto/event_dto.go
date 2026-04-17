package dto

import (
	"groupeak/internal/models"
	"time"
)

type CreateEventRequest struct {
	Title          string     `json:"title"`
	MeetingURL     *string    `json:"meeting_url"`
	Description    *string    `json:"description"`
	StartAt        time.Time  `json:"start_at"`
	EndAt          *time.Time `json:"end_at"`
	ParticipantIDs []int      `json:"participant_ids"`
}

type UpdateEventRequest struct {
	Title          *string    `json:"title"`
	MeetingURL     *string    `json:"meeting_url"`
	Description    *string    `json:"description"`
	StartAt        *time.Time `json:"start_at"`
	EndAt          *time.Time `json:"end_at"`
	ParticipantIDs []int      `json:"participant_ids"`
}

type EventWithParticipants struct {
	models.Event
	Participants []int `json:"participants"`
}
