package handlers

import (
	"net/http"

	"groupeak/internal/dto"
	"groupeak/internal/services"

	"github.com/minio/minio-go/v7"
)

type AuthHandler struct {
	authService *services.AuthService
	minioClient *minio.Client
	bucketName  string
	endpoint    string
}

func NewAuthHandler(authService *services.AuthService, minioClient *minio.Client, bucketName string, endpoint string) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		minioClient: minioClient,
		bucketName:  bucketName,
		endpoint:    endpoint,
	}
}

// Register godoc
// @Summary Регистрация пользователя
// @Tags auth
// @Accept json
// @Produce json
// @Param input body dto.RegisterRequest true "Данные регистрации"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]string "Неверный формат запроса"
// @Router /auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest

	if err := DecodeJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	resp, err := h.authService.Register(r.Context(), req)
	if err != nil {
		HandleError(w, err)
		return
	}

	_ = WriteJSON(w, http.StatusCreated, resp)
}

// Login godoc
// @Summary Авторизация пользователя
// @Tags auth
// @Accept json
// @Produce json
// @Param input body dto.LoginRequest true "Учетные данные"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string "Неверные данные"
// @Router /auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if err := DecodeJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Email == "" || req.Password == "" {
		WriteError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	resp, err := h.authService.Login(r.Context(), req)
	if err != nil {
		HandleError(w, err)
		return
	}

	_ = WriteJSON(w, http.StatusOK, resp)
}

// ChangePassword godoc
// @Summary Смена пароля
// @Security Bearer
// @Tags user
// @Accept json
// @Produce json
// @Param input body dto.ChangePasswordRequest true "Данные для смены пароля"
// @Success 200 {object} map[string]interface{}
// @Router /user/change-password [post]
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req dto.ChangePasswordRequest
	if err := DecodeJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	resp, err := h.authService.ChangePassword(r.Context(), userID, req)
	if err != nil {
		HandleError(w, err)
		return
	}

	_ = WriteJSON(w, http.StatusOK, resp)
}

// ChangeEmail godoc
// @Summary Смена email
// @Security Bearer
// @Tags user
// @Accept json
// @Produce json
// @Param input body dto.ChangeEmailRequest true "Данные для смены email"
// @Success 200 {object} map[string]interface{}
// @Router /user/change-email [post]
func (h *AuthHandler) ChangeEmail(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req dto.ChangeEmailRequest
	if err := DecodeJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	resp, err := h.authService.ChangeEmail(r.Context(), userID, req)
	if err != nil {
		HandleError(w, err)
		return
	}

	_ = WriteJSON(w, http.StatusOK, resp)
}
