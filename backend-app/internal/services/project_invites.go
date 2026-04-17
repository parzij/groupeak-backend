package services

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"

	"groupeak/internal/apperror"
	"groupeak/internal/dto"
	"groupeak/internal/eventbus"
	"groupeak/internal/models"
	"groupeak/internal/repository"
)

var (
	ErrInviteNotFound       = apperror.New(http.StatusNotFound, "invite not found")
	ErrInviteAlreadyUsed    = apperror.New(http.StatusConflict, "invite is already used or revoked")
	ErrInviteAlreadyPending = apperror.New(http.StatusConflict, "invite already pending for this email")
	ErrInviteEmailMismatch  = apperror.New(http.StatusForbidden, "invite email does not match user email")
	ErrInviteTokenRequired  = apperror.New(http.StatusBadRequest, "invite token is required")
)

func (s *ProjectService) InviteMember(ctx context.Context, ownerID, projectID int64, req dto.InviteMemberRequest) (*models.ProjectInvite, error) {
	isOwner, err := s.repo.IsProjectOwner(ctx, s.db, projectID, ownerID)
	if err == sql.ErrNoRows {
		return nil, ErrProjectNotFound
	} else if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}
	if !isOwner {
		return nil, ErrNotProjectOwner
	}

	email, err := normalizeAndValidateEmail(req.Email)
	if err != nil {
		return nil, err
	}

	alreadyMember, err := s.repo.CheckUserIsMemberByEmail(ctx, s.db, projectID, email)
	if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}
	if alreadyMember {
		return nil, ErrAlreadyProjectMember
	}

	hasPending, err := s.repo.CheckPendingInvite(ctx, s.db, projectID, email)
	if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}
	if hasPending {
		return nil, ErrInviteAlreadyPending
	}

	token, err := generateInviteToken()
	if err != nil {
		s.logger.Error("failed to generate invite token", slog.String("error", err.Error()))
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	invite := &models.ProjectInvite{
		ProjectID: projectID,
		Email:     email,
		Token:     token,
	}

	if err := s.repo.CreateInvite(ctx, s.db, invite); err != nil {
		s.logger.Error("failed to create invite in DB", slog.String("error", err.Error()))
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	s.logger.Info("invite created", slog.Int64("project_id", projectID), slog.String("email", email))
	return invite, nil
}

func (s *ProjectService) AcceptInvite(ctx context.Context, userID int64, token string) (*models.ProjectInvite, error) {
	if token == "" {
		return nil, ErrInviteTokenRequired
	}

	invite, err := s.repo.GetInviteByToken(ctx, s.db, token)
	if err == sql.ErrNoRows {
		return nil, ErrInviteNotFound
	} else if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	if invite.Status != "pending" {
		return nil, ErrInviteAlreadyUsed
	}

	userEmail, err := s.repo.GetUserEmail(ctx, s.db, userID)
	if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	if invite.Email != userEmail {
		return nil, ErrInviteEmailMismatch
	}

	var updatedInvite *models.ProjectInvite

	err = s.RunInTx(ctx, func(tx repository.DBTX) error {
		if errTx := s.repo.AddProjectMember(ctx, tx, invite.ProjectID, userID, "member"); errTx != nil {
			return errTx
		}
		var errTx error
		updatedInvite, errTx = s.repo.UpdateInviteStatus(ctx, tx, invite.ID, "accepted")
		return errTx
	})

	if err != nil {
		s.logger.Error("failed to accept invite", slog.String("error", err.Error()))
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	s.logger.Info("invite accepted", slog.Int64("user_id", userID), slog.Int64("project_id", invite.ProjectID))

	ownerID, _ := s.repo.GetProjectOwnerID(ctx, s.db, invite.ProjectID)

	// Отправляем событие о том, что юзер присоединился к проекту
	s.bus.Publish(eventbus.SystemEvent{
		Type:      models.EventTypeProjectMember,
		ProjectID: invite.ProjectID,
		ActorID:   ownerID,
		EntityID:  userID,
		Payload: map[string]interface{}{
			"target_user_id": userID,
			"action":         "joined",
		},
	})

	return updatedInvite, nil
}

func (s *ProjectService) ListProjectMembers(ctx context.Context, userID, projectID int64) ([]models.ProjectMemberWithUser, error) {
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

	members, err := s.repo.ListProjectMembers(ctx, s.db, projectID)
	if err != nil {
		s.logger.Error("failed to list project members", slog.String("error", err.Error()))
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	if members == nil {
		members = []models.ProjectMemberWithUser{}
	}

	return members, nil
}
