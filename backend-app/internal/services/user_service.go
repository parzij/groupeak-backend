package services

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"groupeak/internal/apperror"
	"groupeak/internal/dto"
	"groupeak/internal/models"
)

func (s *AuthService) GetProfile(ctx context.Context, userID int64) (*models.User, error) {
	user, _, err := s.repo.GetUserByID(ctx, s.db, userID)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, fmt.Sprintf("get profile: %v", err))
	}
	return user, nil
}

func (s *AuthService) UpdateProfile(ctx context.Context, userID int64, req dto.UpdateProfileRequest) (*dto.UpdateProfileResponse, error) {
	setParts := make([]string, 0, 5)
	args := make([]interface{}, 0, 5)
	idx := 1
	changed := false
	tokenNeedsRefresh := false
	var newPosition *string

	if req.FullName != nil {
		fn := strings.TrimSpace(*req.FullName)
		if fn == "" {
			return nil, apperror.New(http.StatusBadRequest, "full name cannot be empty")
		}
		setParts = append(setParts, fmt.Sprintf("full_name = $%d", idx))
		args = append(args, fn)
		idx++
		changed = true
	}

	if req.Position != nil {
		if !ValidatePosition(req.Position) {
			return nil, ErrInvalidPosition
		}
		setParts = append(setParts, fmt.Sprintf("position = $%d", idx))
		args = append(args, *req.Position)
		idx++
		changed = true
		tokenNeedsRefresh = true
		newPosition = req.Position
	}

	if req.BirthDate != nil {
		bdStr := strings.TrimSpace(*req.BirthDate)
		if bdStr == "" {
			setParts = append(setParts, "birth_date = NULL")
			changed = true
		} else {
			bd, err := time.Parse("02.01.2006", bdStr)
			if err != nil {
				return nil, ErrInvalidBirthDate
			}
			if err := ValidateBirthDateAge(bd); err != nil {
				return nil, err
			}
			setParts = append(setParts, fmt.Sprintf("birth_date = $%d", idx))
			args = append(args, bd.Format("2006-01-02"))
			idx++
			changed = true
		}
	}

	if req.About != nil {
		abt := strings.TrimSpace(*req.About)
		if abt == "" {
			setParts = append(setParts, "about = NULL")
		} else {
			setParts = append(setParts, fmt.Sprintf("about = $%d", idx))
			args = append(args, abt)
			idx++
		}
		changed = true
	}

	if !changed {
		return nil, ErrNothingToUpdate
	}

	user, err := s.repo.UpdateProfileDynamic(ctx, s.db, userID, setParts, args)
	if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, fmt.Sprintf("update profile: %v", err))
	}

	resp := &dto.UpdateProfileResponse{User: *user}

	if tokenNeedsRefresh {
		token, err := s.generateToken(userID, newPosition)
		if err == nil {
			resp.Token = token
		}
	}

	return resp, nil
}

func (s *AuthService) GetTaskStats(ctx context.Context, userID int64) (models.TaskStats, error) {
	return s.repo.GetTaskStats(ctx, s.db, userID)
}

func (s *AuthService) UpdateAvatarURL(ctx context.Context, userID int64, avatarURL string) error {
	err := s.repo.UpdateAvatarURL(ctx, s.db, userID, &avatarURL)
	if err == sql.ErrNoRows {
		return ErrUserNotFound
	}
	return err
}

func (s *AuthService) DeleteAvatarURL(ctx context.Context, userID int64) error {
	err := s.repo.UpdateAvatarURL(ctx, s.db, userID, nil)
	if err == sql.ErrNoRows {
		return ErrUserNotFound
	}
	return err
}
