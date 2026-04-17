package services

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"groupeak/internal/apperror"
	"groupeak/internal/dto"
	"groupeak/internal/eventbus"
	"groupeak/internal/models"
	"groupeak/internal/repository"
)

var (
	ErrTaskNotFound            = apperror.New(http.StatusNotFound, "task not found")
	ErrInvalidTaskStatus       = apperror.New(http.StatusBadRequest, "invalid task status for this operation")
	ErrNotTaskAssignee         = apperror.New(http.StatusForbidden, "user is not assignee of this task")
	ErrTaskTitleRequired       = apperror.New(http.StatusBadRequest, "task title is required")
	ErrInvalidTaskPriority     = apperror.New(http.StatusBadRequest, "invalid task priority")
	ErrInvalidTaskStatusValue  = apperror.New(http.StatusBadRequest, "invalid task status")
	ErrAssigneeNotFound        = apperror.New(http.StatusNotFound, "one or more assignees do not exist")
	ErrAssigneesNotInProject   = apperror.New(http.StatusBadRequest, "one or more assignees have no access to this project")
	ErrTaskNotFoundOrBadStatus = apperror.New(http.StatusBadRequest, "task not found or bad status")
	ErrDueAtRequired           = apperror.New(http.StatusBadRequest, "due_at required")
	ErrBadDecision             = apperror.New(http.StatusBadRequest, "bad decision")
	ErrDueDateInPast           = apperror.New(http.StatusBadRequest, "due date cannot be in the past")
	ErrTextTooLong             = apperror.New(http.StatusBadRequest, "text fields exceed maximum length")
	ErrInvalidTaskCategory     = apperror.New(http.StatusBadRequest, "invalid task category")
)

type TaskService struct {
	db     *sql.DB
	repo   repository.TaskRepository
	logger *slog.Logger
	bus    *eventbus.EventBus
}

func NewTaskService(db *sql.DB, repo repository.TaskRepository, logger *slog.Logger, bus *eventbus.EventBus) *TaskService {
	return &TaskService{db: db, repo: repo, logger: logger, bus: bus}
}

// RunInTx оборачивает операции в транзакцию
func (s *TaskService) RunInTx(ctx context.Context, fn func(tx repository.DBTX) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		s.logger.Error("failed to begin tx", slog.String("error", err.Error()))
		return apperror.New(http.StatusInternalServerError, "internal server error")
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		s.logger.Error("failed to commit tx", slog.String("error", err.Error()))
		return apperror.New(http.StatusInternalServerError, "internal server error")
	}
	return nil
}

func (s *TaskService) IsProjectOwner(ctx context.Context, projectID, userID int64) (bool, error) {
	ownerID, err := s.repo.GetProjectOwnerID(ctx, s.db, projectID)
	if err == sql.ErrNoRows {
		return false, ErrProjectNotFound
	} else if err != nil {
		return false, err
	}
	return ownerID == userID, nil
}

func (s *TaskService) validateTaskFields(title string, desc *string) error {
	if len(title) > 200 {
		return ErrTextTooLong
	}
	if desc != nil && len(*desc) > 255 {
		return ErrTextTooLong
	}
	return nil
}

func (s *TaskService) CreateTask(ctx context.Context, userID, projectID int64, req dto.CreateTaskRequest) (*models.Task, error) {
	ok, err := s.IsProjectOwner(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotProjectOwner
	}

	req.TaskName = strings.TrimSpace(req.TaskName)
	if req.TaskName == "" {
		return nil, ErrTaskTitleRequired
	}

	if err := s.validateTaskFields(req.TaskName, req.TaskDescription); err != nil {
		return nil, err
	}

	if req.DueDate != nil && req.DueDate.Before(time.Now()) {
		return nil, ErrDueDateInPast
	}

	if req.Status != "" && req.Status != models.TaskStatusTodo && req.Status != models.TaskStatusInProgress && req.Status != models.TaskStatusDone && req.Status != models.TaskStatusPostponed && req.Status != models.TaskStatusReview {
		return nil, ErrInvalidTaskStatusValue
	}
	if req.Priority != "" && req.Priority != models.TaskPriorityLow && req.Priority != models.TaskPriorityMedium && req.Priority != models.TaskPriorityHigh {
		return nil, ErrInvalidTaskPriority
	}

	if req.Status == "" {
		req.Status = models.TaskStatusTodo
	}
	if req.Priority == "" {
		req.Priority = models.TaskPriorityMedium
	}

	task := &models.Task{
		ProjectID:   projectID,
		Title:       req.TaskName,
		Description: req.TaskDescription,
		Status:      req.Status,
		Priority:    req.Priority,
		DueAt:       req.DueDate,
	}

	err = s.RunInTx(ctx, func(tx repository.DBTX) error {
		if len(req.AssigneeIDs) > 0 {
			ids := make([]int64, len(req.AssigneeIDs))
			for i, id := range req.AssigneeIDs {
				ids[i] = int64(id)
			}
			found, errTx := s.repo.GetProjectMembersExistence(ctx, tx, projectID, ids)
			if errTx != nil {
				return errTx
			}
			if len(found) != len(ids) {
				return ErrAssigneesNotInProject
			}
		}

		if errTx := s.repo.CreateTask(ctx, tx, task); errTx != nil {
			return errTx
		}

		for _, aid := range req.AssigneeIDs {
			if errTx := s.repo.AddAssignee(ctx, tx, task.ID, int64(aid)); errTx != nil {
				return errTx
			}
		}
		return nil
	})

	if err != nil {
		s.logger.Error("failed to create task", slog.String("error", err.Error()))
		return nil, err
	}

	s.bus.Publish(eventbus.SystemEvent{
		Type:      models.EventTypeTaskCreated,
		ProjectID: projectID,
		ActorID:   userID,
		EntityID:  task.ID,
		Payload:   map[string]interface{}{"title": task.Title},
	})
	return task, nil
}

func (s *TaskService) UpdateTask(ctx context.Context, userID, projectID, taskID int64, req dto.PatchTaskRequest) (*models.Task, error) {
	ok, err := s.IsProjectOwner(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotProjectOwner
	}

	_, err = s.repo.GetTaskByID(ctx, s.db, taskID)
	if err == sql.ErrNoRows {
		return nil, ErrTaskNotFound
	}

	oldAssignees, _ := s.repo.GetAssigneesForTask(ctx, s.db, taskID)

	setParts := make([]string, 0)
	args := make([]interface{}, 0)
	idx := 1
	changes := make(map[string]interface{})

	titleToUpdate := req.Title
	if titleToUpdate == nil && req.TaskName != nil {
		titleToUpdate = req.TaskName
	}
	if titleToUpdate != nil {
		val := strings.TrimSpace(*titleToUpdate)
		if val == "" {
			return nil, ErrTaskTitleRequired
		}
		if len(val) > 200 {
			return nil, ErrTextTooLong
		}
		setParts = append(setParts, fmt.Sprintf("title = $%d", idx))
		args = append(args, val)
		idx++
		changes["title_changed"] = true
	}

	descToUpdate := req.Description
	if descToUpdate == nil && req.TaskDescription != nil {
		descToUpdate = req.TaskDescription
	}
	if descToUpdate != nil {
		val := strings.TrimSpace(*descToUpdate)
		if len(val) > 255 {
			return nil, ErrTextTooLong
		}
		if val == "" {
			setParts = append(setParts, "description = NULL")
		} else {
			setParts = append(setParts, fmt.Sprintf("description = $%d", idx))
			args = append(args, val)
			idx++
		}
		changes["description_changed"] = true
	}

	if req.Comments != nil {
		if len(*req.Comments) > 255 {
			return nil, ErrTextTooLong
		}
		setParts = append(setParts, fmt.Sprintf("comments = $%d", idx))
		args = append(args, *req.Comments)
		idx++
		changes["comments_changed"] = true
	}

	if req.Status != nil {
		if *req.Status != models.TaskStatusTodo && *req.Status != models.TaskStatusInProgress && *req.Status != models.TaskStatusDone && *req.Status != models.TaskStatusPostponed && *req.Status != models.TaskStatusReview {
			return nil, ErrInvalidTaskStatusValue
		}
		setParts = append(setParts, fmt.Sprintf("status = $%d", idx))
		args = append(args, *req.Status)
		idx++
		changes["status_changed"] = true
		changes["new_status"] = *req.Status
	}

	if req.Priority != nil {
		if *req.Priority != models.TaskPriorityLow && *req.Priority != models.TaskPriorityMedium && *req.Priority != models.TaskPriorityHigh {
			return nil, ErrInvalidTaskPriority
		}
		setParts = append(setParts, fmt.Sprintf("priority = $%d", idx))
		args = append(args, *req.Priority)
		idx++
		changes["priority_changed"] = true
		changes["new_priority"] = *req.Priority
	}

	dueAtToUpdate := req.DueAt
	if dueAtToUpdate == nil && req.DueDate != nil {
		dueAtToUpdate = req.DueDate
	}
	if req.ClearDueAt {
		setParts = append(setParts, "due_at = NULL")
		changes["due_at_changed"] = true
	} else if dueAtToUpdate != nil {
		setParts = append(setParts, fmt.Sprintf("due_at = $%d", idx))
		args = append(args, *dueAtToUpdate)
		idx++
		changes["due_at_changed"] = true
	}

	if req.AssigneeIDs != nil {
		changes["assignees_changed"] = true
	}

	if len(setParts) == 0 && req.AssigneeIDs == nil {
		return nil, ErrNoFieldsToUpdate
	}

	var updatedTask *models.Task

	err = s.RunInTx(ctx, func(tx repository.DBTX) error {
		if req.AssigneeIDs != nil {
			if errTx := s.repo.ClearAssignees(ctx, tx, taskID); errTx != nil {
				return errTx
			}
			for _, aid := range *req.AssigneeIDs {
				if errTx := s.repo.AddAssignee(ctx, tx, taskID, aid); errTx != nil {
					return errTx
				}
			}
		}

		if len(setParts) > 0 {
			t, errTx := s.repo.UpdateTaskDynamic(ctx, tx, taskID, setParts, args)
			if errTx != nil {
				return errTx
			}
			updatedTask = t
		} else {
			t, errTx := s.repo.GetTaskByID(ctx, tx, taskID)
			if errTx != nil {
				return errTx
			}
			updatedTask = t
		}
		return nil
	})

	if err != nil {
		s.logger.Error("failed to update task", slog.String("error", err.Error()))
		return nil, err
	}

	assignees, _ := s.repo.GetAssigneesForTask(ctx, s.db, taskID)
	updatedTask.AssigneeIDs = assignees

	if req.AssigneeIDs != nil {
		newAssigneesMap := make(map[int64]bool)
		for _, aid := range *req.AssigneeIDs {
			newAssigneesMap[aid] = true
		}

		for _, oldID := range oldAssignees {
			if !newAssigneesMap[oldID] {
				s.bus.Publish(eventbus.SystemEvent{
					Type:      models.EventTypeTaskUnassigned,
					ProjectID: projectID,
					ActorID:   userID,
					EntityID:  taskID,
					Payload: map[string]interface{}{
						"target_user_id": oldID,
					},
				})
			}
		}
	}

	if len(changes) > 0 {
		s.bus.Publish(eventbus.SystemEvent{
			Type:      models.EventTypeTaskUpdated,
			ProjectID: projectID,
			ActorID:   userID,
			EntityID:  taskID,
			Payload:   changes,
		})
	}

	return updatedTask, nil
}

func (s *TaskService) GetProjectTaskByID(ctx context.Context, userID, projectID, taskID int64) (*models.Task, error) {
	isMember, err := s.repo.CheckProjectMember(ctx, s.db, projectID, userID)
	if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}
	if !isMember {
		return nil, ErrNotProjectMember
	}

	task, err := s.repo.GetTaskByID(ctx, s.db, taskID)
	if err == sql.ErrNoRows || (task != nil && task.ProjectID != projectID) {
		return nil, ErrTaskNotFound
	} else if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	assignees, err := s.repo.GetAssigneesForTask(ctx, s.db, taskID)
	if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}
	task.AssigneeIDs = assignees

	return task, nil
}

func (s *TaskService) DeleteTask(ctx context.Context, userID, projectID, taskID int64) error {
	ok, err := s.IsProjectOwner(ctx, projectID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}

	assignees, _ := s.repo.GetAssigneesForTask(ctx, s.db, taskID)

	err = s.repo.DeleteTask(ctx, s.db, taskID)
	if err == sql.ErrNoRows {
		return ErrTaskNotFound
	} else if err != nil {
		return apperror.New(http.StatusInternalServerError, "internal server error")
	}

	s.bus.Publish(eventbus.SystemEvent{
		Type:      models.EventTypeTaskDeleted,
		ProjectID: projectID,
		ActorID:   userID,
		EntityID:  taskID,
		Payload: map[string]interface{}{
			"deleted":         true,
			"target_user_ids": assignees,
		},
	})
	return nil
}

func (s *TaskService) ListProjectTasks(ctx context.Context, userID, projectID, limit, offset int64) ([]models.Task, error) {
	isMember, err := s.repo.CheckProjectMember(ctx, s.db, projectID, userID)
	if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}
	if !isMember {
		return nil, ErrNotProjectMember
	}

	tasks, taskIDs, err := s.repo.ListProjectTasks(ctx, s.db, projectID, limit, offset)
	if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	return s.populateAssigneesForTasks(ctx, tasks, taskIDs)
}

func (s *TaskService) ListProjectTasksByCategory(ctx context.Context, userID, projectID int64, category models.TaskCategory) ([]models.Task, error) {
	isMember, err := s.repo.CheckProjectMember(ctx, s.db, projectID, userID)
	if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}
	if !isMember {
		return nil, ErrNotProjectMember
	}

	if category != models.TaskCategoryCurrent && category != models.TaskCategoryReview && category != models.TaskCategoryInactive {
		return nil, ErrInvalidTaskCategory
	}

	tasks, taskIDs, err := s.repo.ListTasksByCategory(ctx, s.db, projectID, category)
	if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	return s.populateAssigneesForTasks(ctx, tasks, taskIDs)
}

func (s *TaskService) ListProjectTasksByAssignees(ctx context.Context, userID, projectID int64, assignees []int64) ([]models.Task, error) {
	isMember, err := s.repo.CheckProjectMember(ctx, s.db, projectID, userID)
	if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}
	if !isMember {
		return nil, ErrNotProjectMember
	}

	tasks, taskIDs, err := s.repo.ListTasksByAssignees(ctx, s.db, projectID, assignees)
	if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	return s.populateAssigneesForTasks(ctx, tasks, taskIDs)
}

func (s *TaskService) ListTasksWithFilter(ctx context.Context, userID int64, filter dto.TaskFilterRequest) ([]models.Task, error) {
	tasks, taskIDs, err := s.repo.ListTasksWithFilter(ctx, s.db, userID, filter)
	if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	return s.populateAssigneesForTasks(ctx, tasks, taskIDs)
}

func (s *TaskService) ListNearestTasks(ctx context.Context, userID int64) ([]models.Task, error) {
	tasks, taskIDs, err := s.repo.ListNearestTasks(ctx, s.db, userID)
	if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	return s.populateAssigneesForTasks(ctx, tasks, taskIDs)
}

func (s *TaskService) GetMyTasks(ctx context.Context, userID int64) ([]models.Task, error) {
	tasks, taskIDs, err := s.repo.GetMyTasks(ctx, s.db, userID)
	if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	return s.populateAssigneesForTasks(ctx, tasks, taskIDs)
}

func (s *TaskService) populateAssigneesForTasks(ctx context.Context, tasks []models.Task, taskIDs []int64) ([]models.Task, error) {
	if len(tasks) == 0 {
		return tasks, nil
	}
	assigneesMap, err := s.repo.GetAssigneesForTasks(ctx, s.db, taskIDs)
	if err != nil {
		s.logger.Error("failed to fetch assignees", slog.String("error", err.Error()))
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}
	for i := range tasks {
		if ids := assigneesMap[tasks[i].ID]; ids != nil {
			tasks[i].AssigneeIDs = ids
		} else {
			tasks[i].AssigneeIDs = []int64{}
		}
	}
	return tasks, nil
}
