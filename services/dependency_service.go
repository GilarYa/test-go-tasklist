package services

import (
	"fmt"

	"gorm.io/gorm"

	"test-go-tasklist/models"
)

type DependencyService struct {
	db              *gorm.DB
	promoteCallback func(projectID uint)
}

func NewDependencyService(db *gorm.DB) *DependencyService {
	return &DependencyService{db: db}
}

// =============================================
// Task Dependency
// =============================================

// AddTaskDependency adds a dependency and checks for circular references
func (s *DependencyService) AddTaskDependency(taskID, dependsOnTaskID uint) (*models.TaskDependency, error) {
	if taskID == dependsOnTaskID {
		return nil, fmt.Errorf("task cannot depend on itself")
	}

	// Check both tasks exist
	var task, depTask models.Task
	if err := s.db.First(&task, taskID).Error; err != nil {
		return nil, fmt.Errorf("task with ID %d not found", taskID)
	}
	if err := s.db.First(&depTask, dependsOnTaskID).Error; err != nil {
		return nil, fmt.Errorf("dependency task with ID %d not found", dependsOnTaskID)
	}

	// Check for existing dependency
	var existing models.TaskDependency
	if err := s.db.Where("task_id = ? AND depends_on_task_id = ?", taskID, dependsOnTaskID).First(&existing).Error; err == nil {
		return nil, fmt.Errorf("dependency already exists")
	}

	// Check for circular dependency using DFS
	if s.wouldCreateTaskCycle(taskID, dependsOnTaskID) {
		return nil, fmt.Errorf("circular dependency detected: adding this dependency would create a cycle")
	}

	dep := models.TaskDependency{
		TaskID:          taskID,
		DependsOnTaskID: dependsOnTaskID,
	}

	if err := s.db.Create(&dep).Error; err != nil {
		return nil, fmt.Errorf("failed to create dependency: %v", err)
	}

	s.db.Preload("DependsOnTask").First(&dep, dep.ID)
	return &dep, nil
}

// wouldCreateTaskCycle checks if adding an edge taskID -> dependsOnTaskID would create a cycle
// This means checking if there's already a path from dependsOnTaskID to taskID
func (s *DependencyService) wouldCreateTaskCycle(taskID, dependsOnTaskID uint) bool {
	visited := make(map[uint]bool)
	return s.dfsTaskCycle(dependsOnTaskID, taskID, visited)
}

// dfsTaskCycle performs DFS from 'current' node to see if it can reach 'target'
func (s *DependencyService) dfsTaskCycle(current, target uint, visited map[uint]bool) bool {
	if current == target {
		return true
	}
	if visited[current] {
		return false
	}
	visited[current] = true

	var deps []models.TaskDependency
	s.db.Where("task_id = ?", current).Find(&deps)

	for _, dep := range deps {
		if s.dfsTaskCycle(dep.DependsOnTaskID, target, visited) {
			return true
		}
	}
	return false
}

// RemoveTaskDependency removes a task dependency
func (s *DependencyService) RemoveTaskDependency(taskID, depID uint) error {
	result := s.db.Where("id = ? AND task_id = ?", depID, taskID).Delete(&models.TaskDependency{})
	if result.RowsAffected == 0 {
		return fmt.Errorf("dependency not found")
	}
	return result.Error
}

// ValidateTaskStatusChange checks if a task can change to the given status
func (s *DependencyService) ValidateTaskStatusChange(taskID uint, newStatus string) error {
	if newStatus != "Done" {
		return nil
	}

	// Check all dependencies are Done
	var deps []models.TaskDependency
	s.db.Preload("DependsOnTask").Where("task_id = ?", taskID).Find(&deps)

	for _, dep := range deps {
		if dep.DependsOnTask != nil && dep.DependsOnTask.Status != "Done" {
			return fmt.Errorf("cannot mark task as Done: dependency task '%s' (ID: %d) is not Done (current: %s)",
				dep.DependsOnTask.Name, dep.DependsOnTask.ID, dep.DependsOnTask.Status)
		}
	}

	return nil
}

// RevalidateDependentTasks revalidates tasks that depend on a changed task
// If a dependency is no longer Done, dependent tasks that are Done must be reverted
func (s *DependencyService) RevalidateDependentTasks(changedTaskID uint, newStatus string) error {
	if newStatus == "Done" {
		return nil // No need to cascade when marking as Done
	}

	// Find all tasks that depend on the changed task
	var dependentDeps []models.TaskDependency
	s.db.Where("depends_on_task_id = ?", changedTaskID).Find(&dependentDeps)

	for _, dep := range dependentDeps {
		var dependentTask models.Task
		if err := s.db.First(&dependentTask, dep.TaskID).Error; err != nil {
			continue
		}

		// If the dependent task is Done but its dependency is no longer Done, revert it
		if dependentTask.Status == "Done" {
			dependentTask.Status = "In Progress"
			s.db.Save(&dependentTask)
			// Recursively revalidate tasks that depend on this one
			s.RevalidateDependentTasks(dependentTask.ID, "In Progress")
		}
	}

	return nil
}

// GetTaskDependencies returns all dependencies for a task
func (s *DependencyService) GetTaskDependencies(taskID uint) ([]models.TaskDependency, error) {
	var deps []models.TaskDependency
	err := s.db.Preload("DependsOnTask").Where("task_id = ?", taskID).Find(&deps).Error
	return deps, err
}

// =============================================
// Project Dependency
// =============================================

// AddProjectDependency adds a project dependency with circular detection
func (s *DependencyService) AddProjectDependency(projectID, dependsOnProjectID uint) (*models.ProjectDependency, error) {
	if projectID == dependsOnProjectID {
		return nil, fmt.Errorf("project cannot depend on itself")
	}

	// Check both projects exist
	var project, depProject models.Project
	if err := s.db.First(&project, projectID).Error; err != nil {
		return nil, fmt.Errorf("project with ID %d not found", projectID)
	}
	if err := s.db.First(&depProject, dependsOnProjectID).Error; err != nil {
		return nil, fmt.Errorf("dependency project with ID %d not found", dependsOnProjectID)
	}

	// Check for existing dependency
	var existing models.ProjectDependency
	if err := s.db.Where("project_id = ? AND depends_on_project_id = ?", projectID, dependsOnProjectID).First(&existing).Error; err == nil {
		return nil, fmt.Errorf("dependency already exists")
	}

	// Check for circular dependency
	if s.wouldCreateProjectCycle(projectID, dependsOnProjectID) {
		return nil, fmt.Errorf("circular dependency detected: adding this dependency would create a cycle")
	}

	dep := models.ProjectDependency{
		ProjectID:          projectID,
		DependsOnProjectID: dependsOnProjectID,
	}

	if err := s.db.Create(&dep).Error; err != nil {
		return nil, fmt.Errorf("failed to create dependency: %v", err)
	}

	s.db.Preload("DependsOnProject").First(&dep, dep.ID)
	return &dep, nil
}

func (s *DependencyService) wouldCreateProjectCycle(projectID, dependsOnProjectID uint) bool {
	visited := make(map[uint]bool)
	return s.dfsProjectCycle(dependsOnProjectID, projectID, visited)
}

func (s *DependencyService) dfsProjectCycle(current, target uint, visited map[uint]bool) bool {
	if current == target {
		return true
	}
	if visited[current] {
		return false
	}
	visited[current] = true

	var deps []models.ProjectDependency
	s.db.Where("project_id = ?", current).Find(&deps)

	for _, dep := range deps {
		if s.dfsProjectCycle(dep.DependsOnProjectID, target, visited) {
			return true
		}
	}
	return false
}

// RemoveProjectDependency removes a project dependency
func (s *DependencyService) RemoveProjectDependency(projectID, depID uint) error {
	result := s.db.Where("id = ? AND project_id = ?", depID, projectID).Delete(&models.ProjectDependency{})
	if result.RowsAffected == 0 {
		return fmt.Errorf("dependency not found")
	}
	return result.Error
}

// ValidateProjectStatusChange checks if a project can change to the given status
func (s *DependencyService) ValidateProjectStatusChange(projectID uint, newStatus string) error {
	if newStatus == "Draft" {
		return nil
	}

	// For In Progress or Done, all dependency projects must be Done
	var deps []models.ProjectDependency
	s.db.Preload("DependsOnProject").Where("project_id = ?", projectID).Find(&deps)

	for _, dep := range deps {
		if dep.DependsOnProject != nil && dep.DependsOnProject.Status != "Done" {
			return fmt.Errorf("cannot change project status to '%s': dependency project '%s' (ID: %d) is not Done (current: %s)",
				newStatus, dep.DependsOnProject.Name, dep.DependsOnProject.ID, dep.DependsOnProject.Status)
		}
	}

	return nil
}

// CascadeProjectStatusChange when a dependency project changes status,
// cascade the change to all dependent projects
func (s *DependencyService) CascadeProjectStatusChange(changedProjectID uint, newStatus string) error {
	// Find all projects that depend on the changed project
	var dependentDeps []models.ProjectDependency
	s.db.Where("depends_on_project_id = ?", changedProjectID).Find(&dependentDeps)

	if newStatus == "Done" {
		// Promote: dependency is now Done, re-evaluate dependent projects
		// They might be able to advance to their natural status
		for _, dep := range dependentDeps {
			if s.promoteCallback != nil {
				s.promoteCallback(dep.ProjectID)
			}
		}
		return nil
	}

	// Downgrade: dependency is no longer Done, revert dependent projects
	for _, dep := range dependentDeps {
		var dependentProject models.Project
		if err := s.db.First(&dependentProject, dep.ProjectID).Error; err != nil {
			continue
		}

		// If the dependent project is Done or In Progress but dependency is no longer Done
		if dependentProject.Status == "Done" || dependentProject.Status == "In Progress" {
			// Revert to a safe status
			if newStatus == "Draft" {
				dependentProject.Status = "Draft"
			} else {
				dependentProject.Status = "In Progress"
			}
			s.db.Save(&dependentProject)
			// Recursively cascade
			s.CascadeProjectStatusChange(dependentProject.ID, dependentProject.Status)
		}
	}

	return nil
}

// SetPromoteCallback sets a callback function for re-evaluating project status
// This breaks the circular dependency between DependencyService and ProjectService
func (s *DependencyService) SetPromoteCallback(cb func(projectID uint)) {
	s.promoteCallback = cb
}

// GetProjectDependencies returns all dependencies for a project
func (s *DependencyService) GetProjectDependencies(projectID uint) ([]models.ProjectDependency, error) {
	var deps []models.ProjectDependency
	err := s.db.Preload("DependsOnProject").Where("project_id = ?", projectID).Find(&deps).Error
	return deps, err
}
