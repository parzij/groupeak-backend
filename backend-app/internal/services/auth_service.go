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
	"groupeak/internal/models"
	"groupeak/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailAlreadyTaken      = apperror.New(http.StatusConflict, "email already taken")
	ErrInvalidPosition        = apperror.New(http.StatusBadRequest, "invalid position")
	ErrMissingFields          = apperror.New(http.StatusBadRequest, "full_name, email and password are required")
	ErrWeakPassword           = apperror.New(http.StatusBadRequest, "password must be at least 8 characters long")
	ErrUnsupportedEmailDomain = apperror.New(http.StatusBadRequest, "email domain is not supported")
	ErrInvalidCredentials     = apperror.New(http.StatusUnauthorized, "invalid credentials")
	ErrInvalidBirthDate       = apperror.New(http.StatusBadRequest, "invalid birth date, expected format dd.mm.yyyy")
	ErrUserNotFound           = apperror.New(http.StatusNotFound, "user not found")
	ErrPasswordDoNotMatch     = apperror.New(http.StatusBadRequest, "confirm_new does not match new_password")
	ErrNewPasswordSameAsOld   = apperror.New(http.StatusBadRequest, "new password must not match old password")
	ErrEmailSameAsCurrent     = apperror.New(http.StatusBadRequest, "new email must not match current email")
	ErrPasswordIsRequired     = apperror.New(http.StatusBadRequest, "password is required")
	ErrNewEmailIsRequired     = apperror.New(http.StatusBadRequest, "new_email is required")
	ErrNothingToUpdate        = apperror.New(http.StatusBadRequest, "nothing to update")
)

type JWTClaims struct {
	UserID   int64  `json:"user_id"`
	Position string `json:"position"`
	jwt.RegisteredClaims
}

type AuthService struct {
	db        *sql.DB
	repo      repository.UserRepository
	jwtSecret []byte
	logger    *slog.Logger
}

func NewAuthService(db *sql.DB, repo repository.UserRepository, jwtSecret string, logger *slog.Logger) *AuthService {
	return &AuthService{
		db:        db,
		repo:      repo,
		jwtSecret: []byte(jwtSecret),
		logger:    logger,
	}
}

func (s *AuthService) RunInTx(ctx context.Context, fn func(tx repository.DBTX) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		s.logger.Error("failed to begin transaction", slog.String("error", err.Error()))
		return apperror.New(http.StatusInternalServerError, "failed to start transaction")
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		s.logger.Error("failed to commit transaction", slog.String("error", err.Error()))
		return apperror.New(http.StatusInternalServerError, "failed to commit transaction")
	}
	return nil
}

func (s *AuthService) generateToken(userID int64, position *string) (string, error) {
	pos := ""
	if position != nil {
		pos = *position
	}

	claims := JWTClaims{
		UserID:   userID,
		Position: pos,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(72 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *AuthService) Register(ctx context.Context, req dto.RegisterRequest) (*dto.RegisterResponse, error) {
	req.FullName = strings.TrimSpace(req.FullName)
	req.Email = strings.TrimSpace(req.Email)
	req.Password = strings.TrimSpace(req.Password)

	if req.FullName == "" || req.Email == "" || req.Password == "" {
		return nil, ErrMissingFields
	}
	if len(req.Password) < 8 {
		return nil, ErrWeakPassword
	}
	if !ValidatePosition(req.Position) {
		return nil, ErrInvalidPosition
	}

	email, err := normalizeAndValidateEmail(req.Email)
	if err != nil {
		return nil, err
	}

	if req.BirthDate != nil && *req.BirthDate != "" {
		bd, err := time.Parse("02.01.2006", *req.BirthDate)
		if err != nil {
			return nil, ErrInvalidBirthDate
		}
		if err := ValidateBirthDateAge(bd); err != nil {
			return nil, err
		}
		formattedBD := bd.Format("2006-01-02")
		req.BirthDate = &formattedBD
	} else {
		req.BirthDate = nil
	}

	passwordHash, err := HashPassword(req.Password)
	if err != nil {
		s.logger.Error("failed to hash password", slog.String("error", err.Error()))
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	user := &models.User{
		FullName:  req.FullName,
		Email:     email,
		Position:  req.Position,
		AvatarURL: req.AvatarURL,
		BirthDate: req.BirthDate,
		About:     req.About,
	}

	err = s.RunInTx(ctx, func(tx repository.DBTX) error {
		exists, errTx := s.repo.CheckEmailExists(ctx, tx, email)
		if errTx != nil {
			return fmt.Errorf("check email: %w", errTx)
		}
		if exists {
			return ErrEmailAlreadyTaken
		}

		if errTx := s.repo.CreateUser(ctx, tx, user, passwordHash); errTx != nil {
			return fmt.Errorf("create user: %w", errTx)
		}
		return nil
	})

	if err != nil {
		s.logger.Error("failed to register user", slog.String("email", email), slog.String("error", err.Error()))
		return nil, err
	}

	token, err := s.generateToken(user.ID, user.Position)
	if err != nil {
		s.logger.Error("failed to generate token during registration", slog.Int64("user_id", user.ID), slog.String("error", err.Error()))
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	if req.BirthDate != nil {
		bd, _ := time.Parse("2006-01-02", *req.BirthDate)
		formatted := bd.Format("02.01.2006")
		user.BirthDate = &formatted
	}

	s.logger.Info("user successfully registered", slog.Int64("user_id", user.ID), slog.String("email", user.Email))

	return &dto.RegisterResponse{User: *user, Token: token}, nil
}

func (s *AuthService) Login(ctx context.Context, req dto.LoginRequest) (*dto.LoginResponse, error) {
	email, err := normalizeAndValidateEmail(req.Email)
	if err != nil {
		return nil, err
	}

	user, hash, err := s.repo.GetUserByEmail(ctx, s.db, email)
	if err == sql.ErrNoRows {
		s.logger.Warn("failed login attempt: user not found", slog.String("email", email))
		return nil, ErrInvalidCredentials
	} else if err != nil {
		s.logger.Error("db error during login", slog.String("error", err.Error()))
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		s.logger.Warn("failed login attempt: incorrect password", slog.String("email", email))
		return nil, ErrInvalidCredentials
	}

	token, err := s.generateToken(user.ID, user.Position)
	if err != nil {
		s.logger.Error("failed to generate token during login", slog.Int64("user_id", user.ID), slog.String("error", err.Error()))
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	s.logger.Info("user successfully logged in", slog.Int64("user_id", user.ID))

	return &dto.LoginResponse{User: *user, Token: token}, nil
}

func (s *AuthService) ChangePassword(ctx context.Context, userID int64, req dto.ChangePasswordRequest) (*dto.ChangePasswordResponse, error) {
	if req.OldPassword == "" || req.NewPassword == "" || req.ConfirmNew == "" {
		return nil, ErrMissingFields
	}
	if req.NewPassword != req.ConfirmNew {
		return nil, ErrPasswordDoNotMatch
	}
	if req.OldPassword == req.NewPassword {
		return nil, ErrNewPasswordSameAsOld
	}
	if len(req.NewPassword) < 8 {
		return nil, ErrWeakPassword
	}

	user, currentHash, err := s.repo.GetUserByID(ctx, s.db, userID)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	} else if err != nil {
		s.logger.Error("failed to get user for password change", slog.Int64("user_id", userID), slog.String("error", err.Error()))
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(req.OldPassword)); err != nil {
		return nil, ErrInvalidCredentials
	}

	newHash, err := HashPassword(req.NewPassword)
	if err != nil {
		s.logger.Error("failed to hash new password", slog.String("error", err.Error()))
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	err = s.RunInTx(ctx, func(tx repository.DBTX) error {
		return s.repo.UpdatePassword(ctx, tx, userID, newHash)
	})
	if err != nil {
		s.logger.Error("failed to update password in DB", slog.Int64("user_id", userID), slog.String("error", err.Error()))
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	token, err := s.generateToken(user.ID, user.Position)
	if err != nil {
		s.logger.Error("failed to generate new token after password change", slog.Int64("user_id", userID), slog.String("error", err.Error()))
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	s.logger.Info("user successfully changed password", slog.Int64("user_id", userID))

	return &dto.ChangePasswordResponse{User: *user, Token: token}, nil
}

func (s *AuthService) ChangeEmail(ctx context.Context, userID int64, req dto.ChangeEmailRequest) (*dto.ChangeEmailResponse, error) {
	req.NewEmail = strings.TrimSpace(req.NewEmail)
	req.Password = strings.TrimSpace(req.Password)

	if req.NewEmail == "" {
		return nil, ErrNewEmailIsRequired
	}
	if req.Password == "" {
		return nil, ErrPasswordIsRequired
	}

	newEmail, err := normalizeAndValidateEmail(req.NewEmail)
	if err != nil {
		return nil, err
	}

	user, hash, err := s.repo.GetUserByID(ctx, s.db, userID)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	} else if err != nil {
		s.logger.Error("failed to get user for email change", slog.Int64("user_id", userID), slog.String("error", err.Error()))
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	if strings.EqualFold(user.Email, newEmail) {
		return nil, ErrEmailSameAsCurrent
	}

	err = s.RunInTx(ctx, func(tx repository.DBTX) error {
		exists, errTx := s.repo.CheckEmailExists(ctx, tx, newEmail)
		if errTx != nil {
			return errTx
		}
		if exists {
			return ErrEmailAlreadyTaken
		}
		return s.repo.UpdateEmail(ctx, tx, userID, newEmail)
	})

	if err != nil {
		s.logger.Error("failed to change email in DB", slog.Int64("user_id", userID), slog.String("error", err.Error()))
		return nil, err
	}

	user.Email = newEmail
	token, err := s.generateToken(user.ID, user.Position)
	if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, "internal server error")
	}

	s.logger.Info("user successfully changed email", slog.Int64("user_id", userID), slog.String("new_email", newEmail))

	return &dto.ChangeEmailResponse{User: *user, Token: token}, nil
}
