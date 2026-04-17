package services

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"groupeak/internal/apperror"
	"groupeak/internal/eventbus"
	"groupeak/internal/models"
	"groupeak/internal/repository"
)

func (s *TaskService) TakeTask(ctx context.Context, userID, taskID int64) (*models.Task, error) {
	projectID, status, err := s.repo.GetTaskProjectAndStatus(ctx, s.db, taskID)
	if err == sql.ErrNoRows {
		return nil, ErrTaskNotFound
	}
	if err != nil {
		s.logger.Error("failed to get task project and status", slog.String("error", err.Error()))
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	isMember, err := s.repo.CheckProjectMember(ctx, s.db, projectID, userID)
	if err != nil {
		s.logger.Error("failed to check project member", slog.String("error", err.Error()))
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}
	if !isMember {
		return nil, ErrNotProjectMember
	}

	if status != string(models.TaskStatusTodo) && status != string(models.TaskStatusPostponed) {
		return nil, ErrInvalidTaskStatus
	}

	var updatedTask *models.Task
	err = s.RunInTx(ctx, func(tx repository.DBTX) error {
		if errTx := s.repo.UpdateTaskStatus(ctx, tx, taskID, string(models.TaskStatusInProgress)); errTx != nil {
			return errTx
		}
		t, errTx := s.repo.GetTaskByID(ctx, tx, taskID)
		updatedTask = t
		return errTx
	})

	if err != nil {
		s.logger.Error("failed to take task", slog.String("error", err.Error()))
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	// Отправляем событие о том, что задачу взяли в работу
	s.bus.Publish(eventbus.SystemEvent{
		Type:      models.EventTypeTaskUpdated,
		ProjectID: projectID,
		ActorID:   userID,
		EntityID:  taskID,
		Payload: map[string]interface{}{
			"status_changed": true,
			"new_status":     string(models.TaskStatusInProgress),
		},
	})

	return updatedTask, nil
}

func (s *TaskService) SubmitForReview(ctx context.Context, userID, projectID, taskID int64) error {
	isMember, err := s.repo.CheckProjectMember(ctx, s.db, projectID, userID)
	if err != nil {
		return apperror.New(http.StatusInternalServerError, "internal server error")
	}
	if !isMember {
		return ErrNotProjectMember
	}

	rowsAffected, err := s.repo.SubmitForReview(ctx, s.db, taskID, projectID, userID)
	if err != nil {
		s.logger.Error("failed to submit for review", slog.String("error", err.Error()))
		return apperror.New(http.StatusInternalServerError, "internal server error")
	}
	if rowsAffected == 0 {
		return ErrTaskNotFoundOrBadStatus
	}

	s.logger.Info("task submitted for review", slog.Int64("task_id", taskID), slog.Int64("user_id", userID))

	// Отправляем событие о том, что задача ушла на проверку
	s.bus.Publish(eventbus.SystemEvent{
		Type:      models.EventTypeTaskUpdated,
		ProjectID: projectID,
		ActorID:   userID,
		EntityID:  taskID,
		Payload: map[string]interface{}{
			"status_changed": true,
			"new_status":     string(models.TaskStatusReview),
		},
	})

	return nil
}

func (s *TaskService) ReviewTask(ctx context.Context, ownerID, projectID, taskID int64, decision string, comment *string, newDueAt *time.Time) error {
	ok, err := s.IsProjectOwner(ctx, projectID, ownerID)
	if err != nil {
		s.logger.Error("failed to check project owner for review", slog.String("error", err.Error()))
		return apperror.New(http.StatusInternalServerError, "internal server error")
	}
	if !ok {
		return ErrNotProjectOwner
	}

	switch decision {
	case "approve":
		rowsAffected, err := s.repo.ReviewTaskApprove(ctx, s.db, taskID, projectID, ownerID, comment)
		if err != nil {
			s.logger.Error("failed to approve task", slog.String("error", err.Error()))
			return apperror.New(http.StatusInternalServerError, "internal server error")
		}
		if rowsAffected == 0 {
			return ErrTaskNotFoundOrBadStatus
		}
		s.logger.Info("task approved", slog.Int64("task_id", taskID))

		// Собираем payload для approve
		payload := map[string]interface{}{
			"decision": "approve",
		}
		if comment != nil {
			payload["comment"] = *comment
		}

		s.bus.Publish(eventbus.SystemEvent{
			Type:      models.EventTypeTaskReviewed,
			ProjectID: projectID,
			ActorID:   ownerID,
			EntityID:  taskID,
			Payload:   payload,
		})

		return nil

	case "reject":
		if newDueAt == nil {
			return ErrDueAtRequired
		}

		rowsAffected, err := s.repo.ReviewTaskReject(ctx, s.db, taskID, projectID, ownerID, comment, newDueAt)
		if err != nil {
			s.logger.Error("failed to reject task", slog.String("error", err.Error()))
			return apperror.New(http.StatusInternalServerError, "internal server error")
		}
		if rowsAffected == 0 {
			return ErrTaskNotFoundOrBadStatus
		}
		s.logger.Info("task rejected", slog.Int64("task_id", taskID))

		payload := map[string]interface{}{
			"decision":   "reject",
			"new_due_at": newDueAt,
		}
		if comment != nil {
			payload["comment"] = *comment
		}

		s.bus.Publish(eventbus.SystemEvent{
			Type:      models.EventTypeTaskReviewed,
			ProjectID: projectID,
			ActorID:   ownerID,
			EntityID:  taskID,
			Payload:   payload,
		})

		return nil

	default:
		return ErrBadDecision
	}
}
