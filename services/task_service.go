package services

import (
	"fmt"
	"strings"

	"gorm.io/gorm"

	"test-go-tasklist/models"
)

type TaskService struct {
	db                *gorm.DB
	dependencyService *DependencyService
	projectService    *ProjectService
}

func NewTaskService(db *gorm.DB, depSvc *DependencyService, projSvc *ProjectService) *TaskService {
	return &TaskService{
		db:                db,
		dependencyService: depSvc,
		projectService:    projSvc,
	}
}

// GetAllTasks returns tasks with filtering support including subtask hierarchy
func (s *TaskService) GetAllTasks(projectID uint, status, search string) ([]models.Task, error) {
	// Step 1: Find matching task IDs (including subtask-aware search)
	matchingIDs := make(map[uint]bool)
	parentIDsToInclude := make(map[uint]bool)

	var allTasks []models.Task
	query := s.db.Preload("Children.Dependencies.DependsOnTask").Preload("Dependencies.DependsOnTask")

	if projectID > 0 {
		query = query.Where("project_id = ?", projectID)
	}

	query.Find(&allTasks)

	// Build lookup map
	taskMap := make(map[uint]*models.Task)
	for i := range allTasks {
		taskMap[allTasks[i].ID] = &allTasks[i]
	}

	// Apply filters
	hasFilter := status != "" || search != ""

	if hasFilter {
		for i := range allTasks {
			task := &allTasks[i]
			matches := true

			if status != "" {
				// Check if task or any of its subtasks match the status
				if !s.taskOrSubtaskHasStatus(task, status, taskMap) {
					matches = false
				}
			}

			if search != "" {
				// Check if task or any of its subtasks match the search
				if !s.taskOrSubtaskMatchesSearch(task, search, taskMap) {
					matches = false
				}
			}

			if matches {
				matchingIDs[task.ID] = true
				// If this is a subtask, ensure parent is included
				if task.ParentID != nil {
					parentIDsToInclude[*task.ParentID] = true
				}
			}
		}

		// Include parents of matching subtasks (hierarchical consistency)
		for parentID := range parentIDsToInclude {
			matchingIDs[parentID] = true
			// Walk up the parent chain
			if parent, ok := taskMap[parentID]; ok && parent.ParentID != nil {
				matchingIDs[*parent.ParentID] = true
			}
		}
	}

	// Step 2: Return only top-level tasks (parent_id IS NULL) that match
	var result []models.Task
	resultQuery := s.db.
		Preload("Children", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("Children.Dependencies.DependsOnTask").
		Preload("Dependencies.DependsOnTask").
		Where("parent_id IS NULL")

	if projectID > 0 {
		resultQuery = resultQuery.Where("project_id = ?", projectID)
	}

	resultQuery.Order("created_at ASC").Find(&result)

	// Filter results if we have active filters
	if hasFilter {
		var filtered []models.Task
		for _, task := range result {
			if matchingIDs[task.ID] {
				filtered = append(filtered, task)
			}
		}
		return filtered, nil
	}

	return result, nil
}

// taskOrSubtaskHasStatus checks if the task or any of its subtasks have the given status
func (s *TaskService) taskOrSubtaskHasStatus(task *models.Task, status string, taskMap map[uint]*models.Task) bool {
	if task.Status == status {
		return true
	}

	// Check subtasks
	var children []models.Task
	s.db.Where("parent_id = ?", task.ID).Find(&children)

	for i := range children {
		if s.taskOrSubtaskHasStatus(&children[i], status, taskMap) {
			return true
		}
	}

	return false
}

// taskOrSubtaskMatchesSearch checks if the task or any of its subtasks match the search
func (s *TaskService) taskOrSubtaskMatchesSearch(task *models.Task, search string, taskMap map[uint]*models.Task) bool {
	if strings.Contains(strings.ToLower(task.Name), strings.ToLower(search)) {
		return true
	}

	// Check subtasks
	var children []models.Task
	s.db.Where("parent_id = ?", task.ID).Find(&children)

	for i := range children {
		if s.taskOrSubtaskMatchesSearch(&children[i], search, taskMap) {
			return true
		}
	}

	return false
}

// GetTaskByID returns a single task with all associations
func (s *TaskService) GetTaskByID(id uint) (*models.Task, error) {
	var task models.Task
	err := s.db.
		Preload("Children", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("Children.Dependencies.DependsOnTask").
		Preload("Dependencies.DependsOnTask").
		Preload("Project").
		First(&task, id).Error

	if err != nil {
		return nil, fmt.Errorf("task not found")
	}
	return &task, nil
}

// CreateTask creates a new task and recalculates project status
func (s *TaskService) CreateTask(task *models.Task) error {
	// Validate project exists
	var project models.Project
	if err := s.db.First(&project, task.ProjectID).Error; err != nil {
		return fmt.Errorf("project not found")
	}

	// Validate parent task if provided
	if task.ParentID != nil {
		var parent models.Task
		if err := s.db.First(&parent, *task.ParentID).Error; err != nil {
			return fmt.Errorf("parent task not found")
		}
		// Parent must belong to the same project
		if parent.ProjectID != task.ProjectID {
			return fmt.Errorf("parent task must belong to the same project")
		}
	}

	// Default values
	if task.Status == "" {
		task.Status = "Draft"
	}
	if task.Bobot <= 0 {
		task.Bobot = 1
	}

	if err := s.db.Create(task).Error; err != nil {
		return err
	}

	// Recalculate project status
	return s.projectService.RecalculateProjectStatus(task.ProjectID)
}

// UpdateTask updates a task with dependency validation
func (s *TaskService) UpdateTask(id uint, input map[string]interface{}) (*models.Task, error) {
	var task models.Task
	if err := s.db.First(&task, id).Error; err != nil {
		return nil, fmt.Errorf("task not found")
	}

	oldStatus := task.Status

	// If status is being changed, validate dependencies
	if newStatus, ok := input["status"]; ok {
		statusStr := fmt.Sprintf("%v", newStatus)
		if err := s.dependencyService.ValidateTaskStatusChange(id, statusStr); err != nil {
			return nil, err
		}
	}

	// Apply updates
	if err := s.db.Model(&task).Updates(input).Error; err != nil {
		return nil, err
	}

	// Reload task
	s.db.First(&task, id)

	// If status changed, revalidate dependent tasks
	if oldStatus != task.Status {
		s.dependencyService.RevalidateDependentTasks(id, task.Status)
	}

	// Always recalculate project progress (status or bobot may have changed)
	s.projectService.RecalculateProjectStatus(task.ProjectID)

	return s.GetTaskByID(id)
}

// DeleteTask deletes a task and its subtasks, then recalculates project
func (s *TaskService) DeleteTask(id uint) error {
	var task models.Task
	if err := s.db.First(&task, id).Error; err != nil {
		return fmt.Errorf("task not found")
	}

	projectID := task.ProjectID

	// Delete dependencies referencing this task or its children
	s.deleteTaskDependenciesRecursive(id)

	// Delete the task (cascade deletes children via FK)
	if err := s.db.Delete(&task).Error; err != nil {
		return err
	}

	// Recalculate project
	return s.projectService.RecalculateProjectStatus(projectID)
}

func (s *TaskService) deleteTaskDependenciesRecursive(taskID uint) {
	// Delete deps for this task
	s.db.Where("task_id = ? OR depends_on_task_id = ?", taskID, taskID).Delete(&models.TaskDependency{})

	// Find children and delete their deps too
	var children []models.Task
	s.db.Where("parent_id = ?", taskID).Find(&children)
	for _, child := range children {
		s.deleteTaskDependenciesRecursive(child.ID)
	}
}
