package handlers

import (
	"net/http"

	"groupeak/internal/dto"
	"groupeak/internal/services"
)

type EventHandler struct {
	service *services.EventService
}

func NewEventHandler(s *services.EventService) *EventHandler {
	return &EventHandler{service: s}
}

// CreateEvent godoc
// @Summary Создать событие/встречу
// @Security Bearer
// @Tags events
// @Accept json
// @Produce json
// @Param projectID path int true "ID Проекта"
// @Param input body dto.CreateEventRequest true "Данные события"
// @Success 201 {object} map[string]interface{}
// @Router /projects/{projectID}/events [post]
func (h *EventHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	projectID, err := ParseIDParam(r, "projectID")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid projectID")
		return
	}

	var req dto.CreateEventRequest
	if err := DecodeJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	event, err := h.service.CreateEvent(r.Context(), int(userID), int(projectID), req)
	if err != nil {
		HandleError(w, err)
		return
	}

	_ = WriteJSON(w, http.StatusCreated, event)
}

// UpdateEvent godoc
// @Summary Обновить событие
// @Security Bearer
// @Tags events
// @Accept json
// @Produce json
// @Param projectID path int true "ID Проекта"
// @Param eventID path int true "ID События"
// @Param input body dto.UpdateEventRequest true "Обновляемые поля"
// @Success 200 {object} map[string]string
// @Router /projects/{projectID}/events/{eventID} [patch]
func (h *EventHandler) UpdateEvent(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	projectID, err := ParseIDParam(r, "projectID")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid projectID")
		return
	}

	eventID, err := ParseIDParam(r, "eventID")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid eventID")
		return
	}

	var req dto.UpdateEventRequest
	if err := DecodeJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	err = h.service.UpdateEvent(r.Context(), int(userID), int(projectID), int(eventID), req)
	if err != nil {
		HandleError(w, err)
		return
	}

	_ = WriteJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// DeleteEvent godoc
// @Summary Удалить событие
// @Security Bearer
// @Tags events
// @Param projectID path int true "ID Проекта"
// @Param eventID path int true "ID События"
// @Success 200 {object} map[string]string
// @Router /projects/{projectID}/events/{eventID} [delete]1
func (h *EventHandler) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	projectID, err := ParseIDParam(r, "projectID")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid projectID")
		return
	}

	eventID, err := ParseIDParam(r, "eventID")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid eventID")
		return
	}

	err = h.service.DeleteEvent(r.Context(), int(userID), int(projectID), int(eventID))
	if err != nil {
		HandleError(w, err)
		return
	}

	_ = WriteJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// GetUserEvents godoc
// @Summary Мои события
// @Security Bearer
// @Tags events
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /events/my [get]
func (h *EventHandler) GetUserEvents(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	events, err := h.service.GetUserEvents(r.Context(), int(userID))
	if err != nil {
		HandleError(w, err)
		return
	}

	_ = WriteJSON(w, http.StatusOK, map[string]interface{}{"events": events})
}
