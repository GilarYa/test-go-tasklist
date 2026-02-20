package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"test-go-tasklist/models"
	"test-go-tasklist/services"
	"test-go-tasklist/utils"
)

type TaskHandler struct {
	taskService       *services.TaskService
	dependencyService *services.DependencyService
}

func NewTaskHandler(ts *services.TaskService, ds *services.DependencyService) *TaskHandler {
	return &TaskHandler{
		taskService:       ts,
		dependencyService: ds,
	}
}

// GetTasks godoc
func (h *TaskHandler) GetTasks(c *gin.Context) {
	var projectID uint
	if pid := c.Query("project_id"); pid != "" {
		id, err := strconv.ParseUint(pid, 10, 64)
		if err != nil {
			utils.ErrorResponse(c, http.StatusBadRequest, "Invalid project_id")
			return
		}
		projectID = uint(id)
	}

	status := c.Query("status")
	search := c.Query("search")

	tasks, err := h.taskService.GetAllTasks(projectID, status, search)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(c, "Tasks retrieved successfully", tasks)
}

// GetTask godoc
func (h *TaskHandler) GetTask(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid task ID")
		return
	}

	task, err := h.taskService.GetTaskByID(uint(id))
	if err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, err.Error())
		return
	}

	utils.SuccessResponse(c, "Task retrieved successfully", task)
}

type CreateTaskInput struct {
	Name      string `json:"name" binding:"required"`
	ProjectID uint   `json:"project_id" binding:"required"`
	ParentID  *uint  `json:"parent_id"`
	Bobot     int    `json:"bobot"`
}

// CreateTask godoc
func (h *TaskHandler) CreateTask(c *gin.Context) {
	var input CreateTaskInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "name and project_id are required")
		return
	}

	task := models.Task{
		Name:      input.Name,
		ProjectID: input.ProjectID,
		ParentID:  input.ParentID,
		Bobot:     input.Bobot,
	}

	if err := h.taskService.CreateTask(&task); err != nil {
		utils.ErrorResponse(c, http.StatusUnprocessableEntity, err.Error())
		return
	}

	result, _ := h.taskService.GetTaskByID(task.ID)
	utils.CreatedResponse(c, "Task created successfully", result)
}

type UpdateTaskInput struct {
	Name      *string `json:"name"`
	Status    *string `json:"status"`
	Bobot     *int    `json:"bobot"`
	ParentID  *uint   `json:"parent_id"`
	ProjectID *uint   `json:"project_id"`
}

// UpdateTask godoc
func (h *TaskHandler) UpdateTask(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid task ID")
		return
	}

	var input UpdateTaskInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid input")
		return
	}

	updates := make(map[string]interface{})

	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.Status != nil {
		// Validate status value
		validStatuses := map[string]bool{"Draft": true, "In Progress": true, "Done": true}
		if !validStatuses[*input.Status] {
			utils.ErrorResponse(c, http.StatusBadRequest, "Invalid status. Must be 'Draft', 'In Progress', or 'Done'")
			return
		}
		updates["status"] = *input.Status
	}
	if input.Bobot != nil {
		updates["bobot"] = *input.Bobot
	}
	if input.ProjectID != nil {
		updates["project_id"] = *input.ProjectID
	}

	task, err := h.taskService.UpdateTask(uint(id), updates)
	if err != nil {
		utils.ErrorResponse(c, http.StatusUnprocessableEntity, err.Error())
		return
	}

	utils.SuccessResponse(c, "Task updated successfully", task)
}

// DeleteTask godoc
func (h *TaskHandler) DeleteTask(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid task ID")
		return
	}

	if err := h.taskService.DeleteTask(uint(id)); err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, err.Error())
		return
	}

	utils.SuccessResponse(c, "Task deleted successfully", nil)
}

// AddTaskDependency godoc
func (h *TaskHandler) AddTaskDependency(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid task ID")
		return
	}

	var input struct {
		DependsOnTaskID uint `json:"depends_on_task_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "depends_on_task_id is required")
		return
	}

	dep, err := h.dependencyService.AddTaskDependency(uint(id), input.DependsOnTaskID)
	if err != nil {
		utils.ErrorResponse(c, http.StatusUnprocessableEntity, err.Error())
		return
	}

	utils.CreatedResponse(c, "Task dependency added successfully", dep)
}

// RemoveTaskDependency godoc
func (h *TaskHandler) RemoveTaskDependency(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid task ID")
		return
	}

	depID, err := strconv.ParseUint(c.Param("depId"), 10, 64)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid dependency ID")
		return
	}

	if err := h.dependencyService.RemoveTaskDependency(uint(id), uint(depID)); err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, err.Error())
		return
	}

	utils.SuccessResponse(c, fmt.Sprintf("Task dependency %d removed successfully", depID), nil)
}
