package handlers

import (
	"net/http"
	"time"

	"groupeak/internal/dto"
)

// SubmitForReview godoc
// @Summary Отправить задачу на ревью
// @Security Bearer
// @Tags tasks
// @Produce json
// @Param projectID path int true "ID Проекта"
// @Param taskID path int true "ID Задачи"
// @Success 200 {object} map[string]interface{}
// @Router /projects/{projectID}/tasks/{taskID}/submit [post]
func (h *TaskHandler) SubmitForReview(w http.ResponseWriter, r *http.Request) {
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

	taskID, err := ParseIDParam(r, "taskID")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid taskID")
		return
	}

	err = h.taskService.SubmitForReview(r.Context(), userID, projectID, taskID)
	if err != nil {
		HandleError(w, err)
		return
	}

	_ = WriteJSON(w, http.StatusOK, map[string]interface{}{"status": "ok"})
}

// ReviewTask godoc
// @Summary Проверить задачу (аппрув или реджект)
// @Security Bearer
// @Tags tasks
// @Accept json
// @Produce json
// @Param projectID path int true "ID Проекта"
// @Param taskID path int true "ID Задачи"
// @Param input body dto.ReviewTaskRequest true "Решение"
// @Success 200 {object} map[string]interface{}
// @Router /projects/{projectID}/tasks/{taskID}/review [post]
func (h *TaskHandler) ReviewTask(w http.ResponseWriter, r *http.Request) {
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

	taskID, err := ParseIDParam(r, "taskID")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid taskID")
		return
	}

	var req dto.ReviewTaskRequest
	if err := DecodeJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	var newDueAt *time.Time
	if req.Decision == "reject" {
		if req.DueAt == nil || *req.DueAt == "" {
			WriteError(w, http.StatusBadRequest, "due_at is required for reject")
			return
		}
		t, err := time.Parse(time.RFC3339, *req.DueAt)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid due_at format")
			return
		}
		newDueAt = &t
	}

	err = h.taskService.ReviewTask(r.Context(), userID, projectID, taskID, req.Decision, req.Comment, newDueAt)
	if err != nil {
		HandleError(w, err)
		return
	}

	_ = WriteJSON(w, http.StatusOK, map[string]interface{}{"status": "ok"})
}
