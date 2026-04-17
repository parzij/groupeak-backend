package handlers

import (
	"net/http"
	"strconv"

	"groupeak/internal/dto"
	"groupeak/internal/services"
)

type NotificationHandler struct {
	service *services.NotificationService
}

func NewNotificationHandler(s *services.NotificationService) *NotificationHandler {
	return &NotificationHandler{service: s}
}

// GetNotifications godoc
// @Summary Список уведомлений
// @Security Bearer
// @Tags notifications
// @Produce json
// @Param limit query int false "Лимит"
// @Param offset query int false "Отступ"
// @Success 200 {object} map[string]interface{}
// @Router /notifications [get]
func (h *NotificationHandler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	notifs, err := h.service.GetMyNotifications(r.Context(), userID, limit, offset)
	if err != nil {
		HandleError(w, err)
		return
	}

	_ = WriteJSON(w, http.StatusOK, map[string]interface{}{"notifications": notifs})
}

// GetUnreadCount godoc
// @Summary Количество непрочитанных уведомлений
// @Security Bearer
// @Tags notifications
// @Produce json
// @Success 200 {object} map[string]int
// @Router /notifications/unread-count [get]
func (h *NotificationHandler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	count, err := h.service.GetUnreadCount(r.Context(), userID)
	if err != nil {
		HandleError(w, err)
		return
	}

	_ = WriteJSON(w, http.StatusOK, map[string]int{"unread_count": count})
}

// MarkRead godoc
// @Summary Отметить как прочитанные
// @Security Bearer
// @Tags notifications
// @Accept json
// @Produce json
// @Param input body dto.ReadNotificationRequest true "Массив ID (пусто для всех)"
// @Success 200 {object} map[string]string
// @Router /notifications/read [patch]
func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req dto.ReadNotificationRequest
	if err := DecodeJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid body")
		return
	}

	var errService error
	if len(req.NotificationIDs) > 0 {
		errService = h.service.MarkAsRead(r.Context(), userID, req.NotificationIDs)
	} else {
		errService = h.service.MarkAllAsRead(r.Context(), userID)
	}

	if errService != nil {
		HandleError(w, errService)
		return
	}

	_ = WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
