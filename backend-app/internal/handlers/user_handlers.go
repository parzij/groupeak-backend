package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"groupeak/internal/dto"

	"github.com/minio/minio-go/v7"
)

func generateRandomFileName(ext string) string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes) + ext
}

var allowedAvatarExts = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
	".heic": "image/heic",
	".heif": "image/heif",
}

// UploadAvatar godoc
// @Summary Загрузка аватара
// @Security Bearer
// @Tags user
// @Accept multipart/form-data
// @Produce json
// @Param avatar formData file true "Файл изображения (jpg, png, heic)"
// @Success 200 {object} map[string]string
// @Router /user/avatar [post]
func (h *AuthHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userProfile, err := h.authService.GetProfile(r.Context(), userID)
	var oldAvatarUrl string
	if err == nil && userProfile.AvatarURL != nil {
		oldAvatarUrl = *userProfile.AvatarURL
	}

	if err := r.ParseMultipartForm(8 << 20); err != nil {
		WriteError(w, http.StatusBadRequest, "file too large or invalid form data (max 8MB)")
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "failed to get avatar file")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	contentType, isAllowed := allowedAvatarExts[ext]
	if !isAllowed {
		WriteError(w, http.StatusBadRequest, "unsupported file format. Allowed: jpg, jpeg, png, webp, heic, heif")
		return
	}

	fileName := generateRandomFileName(ext)

	_, err = h.minioClient.PutObject(r.Context(), h.bucketName, fileName, file, header.Size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to upload image to storage")
		return
	}

	baseURL := h.endpoint
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}

	avatarURL := fmt.Sprintf("%s/%s/%s", baseURL, h.bucketName, fileName)

	err = h.authService.UpdateAvatarURL(r.Context(), userID, avatarURL)
	if err != nil {
		go func() {
			_ = h.minioClient.RemoveObject(context.Background(), h.bucketName, fileName, minio.RemoveObjectOptions{})
		}()
		HandleError(w, err)
		return
	}

	if oldAvatarUrl != "" {
		parts := strings.Split(oldAvatarUrl, "/")
		oldFileName := parts[len(parts)-1]

		if oldFileName != "" {
			go func(name string) {
				_ = h.minioClient.RemoveObject(context.Background(), h.bucketName, name, minio.RemoveObjectOptions{})
			}(oldFileName)
		}
	}

	_ = WriteJSON(w, http.StatusOK, map[string]string{
		"avatar_url": avatarURL,
	})
}

// DeleteAvatar godoc
// @Summary Удаление аватара
// @Security Bearer
// @Tags user
// @Produce json
// @Success 200 {object} map[string]string
// @Router /user/avatar [delete]
func (h *AuthHandler) DeleteAvatar(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userProfile, err := h.authService.GetProfile(r.Context(), userID)
	if err != nil {
		HandleError(w, err)
		return
	}

	if userProfile.AvatarURL == nil || *userProfile.AvatarURL == "" {
		_ = WriteJSON(w, http.StatusOK, map[string]string{
			"message": "avatar already deleted",
		})
		return
	}

	oldAvatarURL := *userProfile.AvatarURL

	err = h.authService.DeleteAvatarURL(r.Context(), userID)
	if err != nil {
		HandleError(w, err)
		return
	}

	parts := strings.Split(oldAvatarURL, "/")
	fileName := parts[len(parts)-1]

	if fileName != "" {
		go func(name string) {
			_ = h.minioClient.RemoveObject(context.Background(), h.bucketName, name, minio.RemoveObjectOptions{})
		}(fileName)
	}

	_ = WriteJSON(w, http.StatusOK, map[string]string{
		"message": "avatar successfully deleted",
	})
}

// GetProfile godoc
// @Summary Получение профиля пользователя
// @Security Bearer
// @Tags user
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /user/profile [get]
func (h *AuthHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.authService.GetProfile(r.Context(), userID)
	if err != nil {
		HandleError(w, err)
		return
	}

	stats, err := h.authService.GetTaskStats(r.Context(), userID)
	if err != nil {
		HandleError(w, err)
		return
	}

	_ = WriteJSON(w, http.StatusOK, map[string]interface{}{
		"user":  user,
		"stats": stats,
	})
}

// UpdateProfile godoc
// @Summary Обновление профиля
// @Security Bearer
// @Tags user
// @Accept json
// @Produce json
// @Param input body dto.UpdateProfileRequest true "Данные профиля"
// @Success 200 {object} map[string]interface{}
// @Router /user/profile [put]
func (h *AuthHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req dto.UpdateProfileRequest
	if err := DecodeJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	resp, err := h.authService.UpdateProfile(r.Context(), userID, req)
	if err != nil {
		HandleError(w, err)
		return
	}

	_ = WriteJSON(w, http.StatusOK, resp)
}
