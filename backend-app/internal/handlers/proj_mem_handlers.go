package handlers

import (
	"net/http"

	"groupeak/internal/dto"
)

// InviteMember godoc
// @Summary Приглашение в проект
// @Security Bearer
// @Tags projects
// @Accept json
// @Produce json
// @Param projectID path int true "ID Проекта"
// @Param input body dto.InviteMemberRequest true "Данные инвайта"
// @Success 201 {object} map[string]interface{}
// @Router /projects/{projectID}/invites [post]
func (h *ProjectHandler) InviteMember(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	projectID, ok := parseProjectID(r)
	if !ok {
		WriteError(w, http.StatusBadRequest, "invalid projectID")
		return
	}

	var req dto.InviteMemberRequest
	if err := DecodeJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	invite, err := h.projectService.InviteMember(r.Context(), userID, projectID, req)
	if err != nil {
		HandleError(w, err)
		return
	}

	_ = WriteJSON(w, http.StatusCreated, map[string]interface{}{"invite": invite})
}

// ListMembers godoc
// @Summary Список участников проекта
// @Security Bearer
// @Tags projects
// @Produce json
// @Param projectID path int true "ID Проекта"
// @Success 200 {object} map[string]interface{}
// @Router /projects/{projectID}/members [get]
func (h *ProjectHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	projectID, ok := parseProjectID(r)
	if !ok {
		WriteError(w, http.StatusBadRequest, "invalid projectID")
		return
	}

	members, err := h.projectService.ListProjectMembers(r.Context(), userID, projectID)
	if err != nil {
		HandleError(w, err)
		return
	}

	_ = WriteJSON(w, http.StatusOK, map[string]interface{}{"members": members})
}

type acceptInviteRequest struct {
	Token string `json:"token"`
}

// AcceptInvite godoc
// @Summary Принять приглашение
// @Security Bearer
// @Tags projects
// @Accept json
// @Produce json
// @Param input body acceptInviteRequest true "Токен"
// @Success 200 {object} map[string]interface{}
// @Router /projects/invites/accept [post]
func (h *ProjectHandler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req acceptInviteRequest
	if err := DecodeJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	invite, err := h.projectService.AcceptInvite(r.Context(), userID, req.Token)
	if err != nil {
		HandleError(w, err)
		return
	}

	_ = WriteJSON(w, http.StatusOK, map[string]interface{}{"invite": invite})
}

// RemoveMember godoc
// @Summary Удаление участника
// @Security Bearer
// @Tags projects
// @Param projectID path int true "ID Проекта"
// @Param userID path int true "ID Пользователя"
// @Success 204
// @Router /projects/{projectID}/members/{userID} [delete]
func (h *ProjectHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	requesterID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	projectID, ok := parseProjectID(r)
	if !ok {
		WriteError(w, http.StatusBadRequest, "invalid projectID")
		return
	}

	targetUserID, err := ParseIDParam(r, "userID")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid userID")
		return
	}

	err = h.projectService.RemoveMember(r.Context(), requesterID, projectID, targetUserID)
	if err != nil {
		HandleError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
