package handlers

import (
	"net/http"
	"strconv"

	"groupeak/internal/dto"
	"groupeak/internal/services"

	"github.com/go-chi/chi/v5"
)

type ProjectHandler struct {
	projectService *services.ProjectService
}

func NewProjectHandler(projectService *services.ProjectService) *ProjectHandler {
	return &ProjectHandler{
		projectService: projectService,
	}
}

func parseProjectID(r *http.Request) (int64, bool) {
	projectIDStr := chi.URLParam(r, "projectID")
	projectID, err := strconv.ParseInt(projectIDStr, 10, 64)
	if err != nil || projectID <= 0 {
		return 0, false
	}
	return projectID, true
}

// CreateProject godoc
// @Summary Создание проекта
// @Security Bearer
// @Tags projects
// @Accept json
// @Produce json
// @Param input body dto.CreateProjectRequest true "Данные проекта"
// @Success 201 {object} map[string]interface{}
// @Router /projects [post]
func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req dto.CreateProjectRequest
	if err := DecodeJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	project, err := h.projectService.CreateProject(r.Context(), userID, req)
	if err != nil {
		HandleError(w, err)
		return
	}

	_ = WriteJSON(w, http.StatusCreated, map[string]interface{}{"project": project})
}

// ListProjects godoc
// @Summary Список проектов пользователя
// @Security Bearer
// @Tags projects
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /projects [get]
func (h *ProjectHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	projects, err := h.projectService.ListUserProjects(r.Context(), userID)
	if err != nil {
		HandleError(w, err)
		return
	}

	_ = WriteJSON(w, http.StatusOK, map[string]interface{}{"projects": projects})
}

// UpdateProject godoc
// @Summary Обновление проекта
// @Security Bearer
// @Tags projects
// @Accept json
// @Produce json
// @Param projectID path int true "ID Проекта"
// @Param input body dto.PatchProjectRequest true "Обновляемые поля"
// @Success 200 {object} map[string]interface{}
// @Router /projects/{projectID} [patch]
func (h *ProjectHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
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

	var req dto.PatchProjectRequest
	if err := DecodeJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	project, err := h.projectService.UpdateProject(r.Context(), userID, projectID, req)
	if err != nil {
		HandleError(w, err)
		return
	}

	_ = WriteJSON(w, http.StatusOK, map[string]interface{}{"project": project})
}

// DeleteProject godoc
// @Summary Удаление проекта
// @Security Bearer
// @Tags projects
// @Param projectID path int true "ID Проекта"
// @Success 204
// @Router /projects/{projectID} [delete]
func (h *ProjectHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
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

	err := h.projectService.DeleteProject(r.Context(), userID, projectID)
	if err != nil {
		HandleError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetProject godoc
// @Summary Получение проекта по ID
// @Security Bearer
// @Tags projects
// @Produce json
// @Param projectID path int true "ID Проекта"
// @Success 200 {object} map[string]interface{}
// @Router /projects/{projectID} [get]
func (h *ProjectHandler) GetProject(w http.ResponseWriter, r *http.Request) {
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

	project, err := h.projectService.GetProjectDetails(r.Context(), userID, projectID)
	if err != nil {
		HandleError(w, err)
		return
	}

	_ = WriteJSON(w, http.StatusOK, map[string]interface{}{"project": project})
}
