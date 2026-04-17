package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"groupeak/internal/apperror"

	"golang.org/x/crypto/bcrypt"
)

const (
	minAgeYears = 8
	maxAgeYears = 100
)

var (
	ErrBirthDateOutOfRange = apperror.New(http.StatusBadRequest, "birth date must make age between 8 and 100 years")
	ErrInvalidEmail        = apperror.New(http.StatusBadRequest, "invalid email format")
	ErrInvalidAssigneeIDs  = apperror.New(http.StatusBadRequest, "invalid assignee_ids")
)

func CalcAgeYears(birthDate, now time.Time) int {
	age := now.Year() - birthDate.Year()

	if now.Month() < birthDate.Month() || (now.Month() == birthDate.Month() && now.Day() < birthDate.Day()) {
		age--
	}
	return age
}

func ValidateBirthDateAge(birthDate time.Time) error {
	now := time.Now()
	if age := CalcAgeYears(birthDate, now); age < minAgeYears || age > maxAgeYears {
		return ErrBirthDateOutOfRange
	}
	return nil
}

func ValidatePosition(position *string) bool {
	if position == nil {
		return true
	}
	val := strings.TrimSpace(*position)
	switch val {
	case "Frontend", "Backend", "Designer", "QA", "DevOps", "Android", "iOS", "Data Analyst", "Product", "HR":
		return true
	default:
		return false
	}
}

func HashPassword(plain string) (string, error) {
	if plain == "" {
		return "", errors.New("password is empty")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("bcrypt.GenerateFromPassword: %w", err)
	}
	return string(hash), nil
}

func generateInviteToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func normalizeAndValidateEmail(email string) (string, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return "", ErrInvalidEmail
	}
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return "", ErrInvalidEmail
	}
	return strings.ToLower(addr.Address), nil
}

func NormalizeAssigneeIDs64(ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return []int64{}, nil
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))

	for _, v := range ids {
		if v <= 0 {
			return nil, ErrInvalidAssigneeIDs
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out, nil
}
