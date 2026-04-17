package dto

import "groupeak/internal/models"

type RegisterRequest struct {
	FullName  string  `json:"full_name"`
	Email     string  `json:"email"`
	Password  string  `json:"password"`
	Position  *string `json:"position"`
	AvatarURL *string `json:"avatar_url"`
	BirthDate *string `json:"birth_date"`
	About     *string `json:"about"`
}

type RegisterResponse struct {
	User  models.User `json:"user"`
	Token string      `json:"token"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	User  models.User `json:"user"`
	Token string      `json:"token"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
	ConfirmNew  string `json:"confirm_new"`
}

type ChangePasswordResponse struct {
	User  models.User `json:"user"`
	Token string      `json:"token"`
}

type ChangeEmailRequest struct {
	NewEmail string `json:"new_email"`
	Password string `json:"password"`
}

type ChangeEmailResponse struct {
	User  models.User `json:"user"`
	Token string      `json:"token"`
}

type UpdateProfileRequest struct {
	FullName  *string `json:"full_name,omitempty"`
	Position  *string `json:"position"`
	AvatarURL *string `json:"avatar_url,omitempty"`
	BirthDate *string `json:"birth_date,omitempty"`
	About     *string `json:"about,omitempty"`
}

type UpdateProfileResponse struct {
	User  models.User `json:"user"`
	Token string      `json:"token,omitempty"`
}
