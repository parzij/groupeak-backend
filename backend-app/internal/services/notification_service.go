package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"

	"groupeak/internal/eventbus"
	"groupeak/internal/models"
	"groupeak/internal/repository"
)

type NotificationService struct {
	db        *sql.DB
	notifRepo repository.NotificationRepository
	taskRepo  repository.TaskRepository
	eventRepo repository.EventRepository
	logger    *slog.Logger
}

func NewNotificationService(db *sql.DB, nr repository.NotificationRepository, tr repository.TaskRepository, er repository.EventRepository, logger *slog.Logger) *NotificationService {
	return &NotificationService{
		db:        db,
		notifRepo: nr,
		taskRepo:  tr,
		eventRepo: er,
		logger:    logger,
	}
}

// HandleEvent ловит события из шины
func (s *NotificationService) HandleEvent(ctx context.Context, e eventbus.SystemEvent) {
	var targetUserIDs []int64

	switch e.Type {
	case models.EventTypeTaskCreated, models.EventTypeTaskUpdated,
		models.EventTypeTaskReviewed, models.EventTypeTaskDeadline:
		assignees, err := s.taskRepo.GetAssigneesForTask(ctx, s.db, e.EntityID)
		if err == nil {
			targetUserIDs = assignees
		}

	case models.EventTypeProjectMember, models.EventTypeProjectKicked,
		models.EventTypeTaskUnassigned:
		if tID, ok := e.Payload["target_user_id"].(int64); ok {
			targetUserIDs = []int64{tID}
		}

	case models.EventTypeEventCreated, models.EventTypeEventUpdated,
		models.EventTypeEventReminder:
		partsMap, err := s.eventRepo.GetParticipantsForEvents(ctx, s.db, []int{int(e.EntityID)})
		if err == nil && len(partsMap[int(e.EntityID)]) > 0 {
			for _, pID := range partsMap[int(e.EntityID)] {
				targetUserIDs = append(targetUserIDs, int64(pID))
			}
		}

	case models.EventTypeEventDeleted, models.EventTypeProjectUpdated,
		models.EventTypeProjectDeleted, models.EventTypeTaskDeleted:
		if pIDs, ok := e.Payload["target_user_ids"].([]int64); ok {
			targetUserIDs = pIDs
		} else if pIDsInt, ok := e.Payload["target_user_ids"].([]int); ok {
			for _, id := range pIDsInt {
				targetUserIDs = append(targetUserIDs, int64(id))
			}
		}
	}

	payloadBytes, err := json.Marshal(e.Payload)
	if err != nil {
		s.logger.Error("failed to marshal notification payload", slog.String("error", err.Error()))
		return
	}

	for _, targetID := range targetUserIDs {
		if e.ActorID != 0 && targetID == e.ActorID {
			continue
		}

		entID := e.EntityID
		actorID := e.ActorID

		n := &models.Notification{
			ProjectID: e.ProjectID,
			UserID:    targetID,
			ActorID:   &actorID,
			Type:      e.Type,
			EntityID:  &entID,
			Payload:   payloadBytes,
		}

		if err := s.notifRepo.Create(ctx, s.db, n); err != nil {
			s.logger.Error("failed to save notification",
				slog.Int64("user_id", targetID),
				slog.String("error", err.Error()))
		}
	}
}

func (s *NotificationService) GetMyNotifications(ctx context.Context, userID int64, limit, offset int) ([]models.Notification, error) {
	notifs, err := s.notifRepo.GetUserNotifications(ctx, s.db, userID, limit, offset)
	if notifs == nil {
		notifs = []models.Notification{}
	}
	return notifs, err
}

func (s *NotificationService) MarkAsRead(ctx context.Context, userID int64, notificationIDs []int64) error {
	return s.notifRepo.MarkAsRead(ctx, s.db, userID, notificationIDs)
}

func (s *NotificationService) MarkAllAsRead(ctx context.Context, userID int64) error {
	return s.notifRepo.MarkAllAsRead(ctx, s.db, userID)
}

func (s *NotificationService) GetUnreadCount(ctx context.Context, userID int64) (int, error) {
	return s.notifRepo.GetUnreadCount(ctx, s.db, userID)
}
