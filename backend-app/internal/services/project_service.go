package services

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"groupeak/internal/apperror"
	"groupeak/internal/dto"
	"groupeak/internal/eventbus"
	"groupeak/internal/models"
	"groupeak/internal/repository"
)

var (
	ErrProjectNotFound       = apperror.New(http.StatusNotFound, "project not found")
	ErrNotProjectOwner       = apperror.New(http.StatusForbidden, "user is not project owner")
	ErrNotProjectMember      = apperror.New(http.StatusForbidden, "user is not a member of this project")
	ErrAlreadyProjectMember  = apperror.New(http.StatusConflict, "user is already project member")
	ErrProjectNameIsRequired = apperror.New(http.StatusBadRequest, "project name is required")
	ErrCannotRemoveOwner     = apperror.New(http.StatusForbidden, "cannot remove project owner")
	ErrNoFieldsToUpdate      = apperror.New(http.StatusBadRequest, "no fields to update")
	ErrForbidden             = apperror.New(http.StatusForbidden, "forbidden")
)

type ProjectService struct {
	db     *sql.DB
	repo   repository.ProjectRepository
	logger *slog.Logger
	bus    *eventbus.EventBus
}

func NewProjectService(db *sql.DB, repo repository.ProjectRepository, logger *slog.Logger, bus *eventbus.EventBus) *ProjectService {
	return &ProjectService{db: db, repo: repo, logger: logger, bus: bus}
}

// RunInTx оборачивает операции в транзакцию для ProjectService
func (s *ProjectService) RunInTx(ctx context.Context, fn func(tx repository.DBTX) error) error {
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

func (s *ProjectService) CreateProject(ctx context.Context, userID int64, req dto.CreateProjectRequest) (*models.Project, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return nil, ErrProjectNameIsRequired
	}

	p := &models.Project{
		OwnerID:     userID,
		Name:        req.Name,
		Description: req.Description,
		DeadlineAt:  req.DeadlineAt,
	}

	err := s.RunInTx(ctx, func(tx repository.DBTX) error {
		if errTx := s.repo.CreateProject(ctx, tx, p); errTx != nil {
			return errTx
		}
		return s.repo.AddProjectMember(ctx, tx, p.ID, userID, "owner")
	})

	if err != nil {
		s.logger.Error("failed to create project", slog.String("error", err.Error()))
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	s.logger.Info("project created", slog.Int64("project_id", p.ID), slog.Int64("owner_id", userID))
	return p, nil
}

func (s *ProjectService) UpdateProject(ctx context.Context, userID, projectID int64, req dto.PatchProjectRequest) (*models.Project, error) {
	isOwner, err := s.repo.IsProjectOwner(ctx, s.db, projectID, userID)
	if err == sql.ErrNoRows {
		return nil, ErrProjectNotFound
	} else if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}
	if !isOwner {
		return nil, ErrNotProjectOwner
	}

	setParts := make([]string, 0, 3)
	args := make([]interface{}, 0, 3)
	idx := 1
	changed := false

	nameToUpdate := req.Name
	if nameToUpdate == nil && req.ProjectName != nil {
		nameToUpdate = req.ProjectName
	}

	if nameToUpdate != nil {
		val := strings.TrimSpace(*nameToUpdate)
		if val == "" {
			return nil, ErrProjectNameIsRequired
		}
		setParts = append(setParts, fmt.Sprintf("name = $%d", idx))
		args = append(args, val)
		idx++
		changed = true
	}

	if req.Description != nil {
		val := strings.TrimSpace(*req.Description)
		if val == "" {
			setParts = append(setParts, "description = NULL")
		} else {
			setParts = append(setParts, fmt.Sprintf("description = $%d", idx))
			args = append(args, val)
			idx++
		}
		changed = true
	}

	if req.ClearDeadlineAt {
		setParts = append(setParts, "deadline_at = NULL")
		changed = true
	} else if req.DeadlineAt != nil {
		setParts = append(setParts, fmt.Sprintf("deadline_at = $%d", idx))
		args = append(args, *req.DeadlineAt)
		idx++
		changed = true
	}

	if !changed {
		return nil, ErrNoFieldsToUpdate
	}

	updatedProj, err := s.repo.UpdateProjectDynamic(ctx, s.db, projectID, setParts, args)
	if err != nil {
		s.logger.Error("failed to update project", slog.String("error", err.Error()))
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	members, errMembers := s.repo.ListProjectMembers(ctx, s.db, projectID)
	var targetUserIDs []int64
	if errMembers == nil {
		for _, m := range members {
			targetUserIDs = append(targetUserIDs, m.UserID)
		}
	}

	s.bus.Publish(eventbus.SystemEvent{
		Type:      models.EventTypeProjectUpdated,
		ProjectID: projectID,
		ActorID:   userID,
		EntityID:  projectID,
		Payload: map[string]interface{}{
			"name_changed":        nameToUpdate != nil,
			"deadline_at_changed": req.DeadlineAt != nil || req.ClearDeadlineAt,
			"target_user_ids":     targetUserIDs,
		},
	})

	return updatedProj, nil
}

func (s *ProjectService) DeleteProject(ctx context.Context, userID, projectID int64) error {
	isOwner, err := s.repo.IsProjectOwner(ctx, s.db, projectID, userID)
	if err == sql.ErrNoRows {
		return ErrProjectNotFound
	} else if err != nil {
		return apperror.New(http.StatusInternalServerError, "internal server error")
	}
	if !isOwner {
		return ErrNotProjectOwner
	}

	members, errMembers := s.repo.ListProjectMembers(ctx, s.db, projectID)
	var targetUserIDs []int64
	if errMembers == nil {
		for _, m := range members {
			targetUserIDs = append(targetUserIDs, m.UserID)
		}
	}

	projectDetails, _ := s.repo.GetProjectDetails(ctx, s.db, projectID)
	projName := "Удаленный проект"
	if projectDetails != nil {
		projName = projectDetails.Project.Name
	}

	if err := s.repo.DeleteProject(ctx, s.db, projectID); err != nil {
		s.logger.Error("failed to delete project", slog.String("error", err.Error()))
		return apperror.New(http.StatusInternalServerError, "internal server error")
	}

	s.bus.Publish(eventbus.SystemEvent{
		Type:      models.EventTypeProjectDeleted,
		ProjectID: projectID,
		ActorID:   userID,
		EntityID:  projectID,
		Payload: map[string]interface{}{
			"project_name":    projName,
			"target_user_ids": targetUserIDs,
			"deleted":         true,
		},
	})

	s.logger.Info("project deleted", slog.Int64("project_id", projectID))
	return nil
}

func (s *ProjectService) ListUserProjects(ctx context.Context, userID int64) ([]models.Project, error) {
	projects, err := s.repo.ListUserProjects(ctx, s.db, userID)
	if err != nil {
		s.logger.Error("failed to list user projects", slog.String("error", err.Error()))
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}
	if projects == nil {
		projects = []models.Project{}
	}
	return projects, nil
}

func (s *ProjectService) GetProjectDetails(ctx context.Context, userID, projectID int64) (*models.ProjectWithStats, error) {
	err := s.repo.EnsureProjectExists(ctx, s.db, projectID)
	if err == sql.ErrNoRows {
		return nil, ErrProjectNotFound
	} else if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	isMember, err := s.repo.IsProjectMember(ctx, s.db, projectID, userID)
	if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}
	if !isMember {
		return nil, ErrNotProjectMember
	}

	stats, err := s.repo.GetProjectDetails(ctx, s.db, projectID)
	if err == sql.ErrNoRows {
		return nil, ErrProjectNotFound
	} else if err != nil {
		s.logger.Error("failed to get project details", slog.String("error", err.Error()))
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	return stats, nil
}

func (s *ProjectService) checkRemovePolicy(requesterID, ownerID, targetUserID int64) error {
	if targetUserID == ownerID {
		return ErrCannotRemoveOwner
	}
	if requesterID != ownerID && requesterID != targetUserID {
		return ErrForbidden
	}
	return nil
}

func (s *ProjectService) RemoveMember(ctx context.Context, requesterID, projectID, targetUserID int64) error {
	ownerID, err := s.repo.GetProjectOwnerID(ctx, s.db, projectID)
	if err == sql.ErrNoRows {
		return ErrProjectNotFound
	} else if err != nil {
		return apperror.New(http.StatusInternalServerError, "internal server error")
	}

	if err := s.checkRemovePolicy(requesterID, ownerID, targetUserID); err != nil {
		return err
	}

	exists, err := s.repo.IsProjectMember(ctx, s.db, projectID, targetUserID)
	if err != nil {
		return apperror.New(http.StatusInternalServerError, "internal server error")
	}
	if !exists {
		return ErrNotProjectMember
	}

	err = s.RunInTx(ctx, func(tx repository.DBTX) error {
		if errTx := s.repo.RemoveProjectMember(ctx, tx, projectID, targetUserID); errTx != nil {
			return fmt.Errorf("delete member: %w", errTx)
		}
		if errTx := s.repo.RemoveMemberFromTasks(ctx, tx, projectID, targetUserID); errTx != nil {
			return fmt.Errorf("remove from tasks: %w", errTx)
		}
		return nil
	})

	if err != nil {
		s.logger.Error("failed to remove member", slog.String("error", err.Error()))
		return apperror.New(http.StatusInternalServerError, "internal server error")
	}

	s.bus.Publish(eventbus.SystemEvent{
		Type:      models.EventTypeProjectKicked,
		ProjectID: projectID,
		ActorID:   requesterID,
		EntityID:  targetUserID,
		Payload: map[string]interface{}{
			"target_user_id": targetUserID,
			"action":         "kicked",
		},
	})

	return nil
}
