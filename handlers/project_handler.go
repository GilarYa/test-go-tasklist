package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"test-go-tasklist/models"
	"test-go-tasklist/services"
	"test-go-tasklist/utils"
)

type ProjectHandler struct {
	projectService    *services.ProjectService
	dependencyService *services.DependencyService
}

func NewProjectHandler(ps *services.ProjectService, ds *services.DependencyService) *ProjectHandler {
	return &ProjectHandler{
		projectService:    ps,
		dependencyService: ds,
	}
}

// GetProjects godoc
func (h *ProjectHandler) GetProjects(c *gin.Context) {
	status := c.Query("status")
	search := c.Query("search")

	projects, err := h.projectService.GetAllProjects(status, search)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(c, "Projects retrieved successfully", projects)
}

// GetProject godoc
func (h *ProjectHandler) GetProject(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid project ID")
		return
	}

	project, err := h.projectService.GetProjectByID(uint(id))
	if err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, err.Error())
		return
	}

	utils.SuccessResponse(c, "Project retrieved successfully", project)
}

type CreateProjectInput struct {
	Name      string `json:"name" binding:"required"`
	StartDate string `json:"start_date" binding:"required"`
	EndDate   string `json:"end_date" binding:"required"`
}

// CreateProject godoc
func (h *ProjectHandler) CreateProject(c *gin.Context) {
	var input CreateProjectInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "name, start_date, and end_date are required")
		return
	}

	project := models.Project{
		Name: input.Name,
	}

	// Parse dates (required)
	startTime, err := time.Parse("2006-01-02", input.StartDate)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid start_date format (use YYYY-MM-DD)")
		return
	}
	project.StartDate = &startTime

	endTime, err := time.Parse("2006-01-02", input.EndDate)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid end_date format (use YYYY-MM-DD)")
		return
	}
	project.EndDate = &endTime

	if err := h.projectService.CreateProject(&project); err != nil {
		utils.ErrorResponse(c, http.StatusUnprocessableEntity, err.Error())
		return
	}

	// Reload with associations
	result, _ := h.projectService.GetProjectByID(project.ID)
	utils.CreatedResponse(c, "Project created successfully", result)
}

type UpdateProjectInput struct {
	Name      *string `json:"name"`
	StartDate *string `json:"start_date"`
	EndDate   *string `json:"end_date"`
}

// UpdateProject godoc
func (h *ProjectHandler) UpdateProject(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid project ID")
		return
	}

	var input UpdateProjectInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid input")
		return
	}

	updates := make(map[string]interface{})

	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.StartDate != nil {
		if *input.StartDate == "" {
			updates["start_date"] = nil
		} else {
			t, err := time.Parse("2006-01-02", *input.StartDate)
			if err != nil {
				utils.ErrorResponse(c, http.StatusBadRequest, "Invalid start_date format (use YYYY-MM-DD)")
				return
			}
			updates["start_date"] = &t
		}
	}
	if input.EndDate != nil {
		if *input.EndDate == "" {
			updates["end_date"] = nil
		} else {
			t, err := time.Parse("2006-01-02", *input.EndDate)
			if err != nil {
				utils.ErrorResponse(c, http.StatusBadRequest, "Invalid end_date format (use YYYY-MM-DD)")
				return
			}
			updates["end_date"] = &t
		}
	}

	project, err := h.projectService.UpdateProject(uint(id), updates)
	if err != nil {
		utils.ErrorResponse(c, http.StatusUnprocessableEntity, err.Error())
		return
	}

	utils.SuccessResponse(c, "Project updated successfully", project)
}

// DeleteProject godoc
func (h *ProjectHandler) DeleteProject(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid project ID")
		return
	}

	if err := h.projectService.DeleteProject(uint(id)); err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, err.Error())
		return
	}

	utils.SuccessResponse(c, "Project deleted successfully", nil)
}

// AddProjectDependency godoc
func (h *ProjectHandler) AddProjectDependency(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid project ID")
		return
	}

	var input struct {
		DependsOnProjectID uint `json:"depends_on_project_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "depends_on_project_id is required")
		return
	}

	dep, err := h.dependencyService.AddProjectDependency(uint(id), input.DependsOnProjectID)
	if err != nil {
		utils.ErrorResponse(c, http.StatusUnprocessableEntity, err.Error())
		return
	}

	utils.CreatedResponse(c, "Project dependency added successfully", dep)
}

// RemoveProjectDependency godoc
func (h *ProjectHandler) RemoveProjectDependency(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid project ID")
		return
	}

	depID, err := strconv.ParseUint(c.Param("depId"), 10, 64)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid dependency ID")
		return
	}

	if err := h.dependencyService.RemoveProjectDependency(uint(id), uint(depID)); err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, err.Error())
		return
	}

	utils.SuccessResponse(c, "Project dependency removed successfully", nil)
}
