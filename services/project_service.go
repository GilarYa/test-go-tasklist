package services

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"test-go-tasklist/models"
)

type ProjectService struct {
	db                *gorm.DB
	dependencyService *DependencyService
	scheduleService   *ScheduleService
}

func NewProjectService(db *gorm.DB, depSvc *DependencyService, schSvc *ScheduleService) *ProjectService {
	return &ProjectService{
		db:                db,
		dependencyService: depSvc,
		scheduleService:   schSvc,
	}
}

// GetAllProjects returns all projects with optional filters
func (s *ProjectService) GetAllProjects(status, search string) ([]models.Project, error) {
	query := s.db.Preload("Tasks", func(db *gorm.DB) *gorm.DB {
		return db.Where("parent_id IS NULL").Preload("Children").Preload("Dependencies.DependsOnTask")
	}).Preload("Dependencies.DependsOnProject")

	if status != "" {
		// Filter by project status OR by task status (including subtasks)
		query = query.Where(
			"projects.status = ? OR projects.id IN (?)",
			status,
			s.db.Model(&models.Task{}).Select("DISTINCT project_id").Where("status = ?", status),
		)
	}

	if search != "" {
		// Search in project name or in tasks/subtasks
		query = query.Where(
			"projects.name LIKE ? OR projects.id IN (?)",
			"%"+search+"%",
			s.db.Model(&models.Task{}).Select("DISTINCT project_id").Where("name LIKE ?", "%"+search+"%"),
		)
	}

	var projects []models.Project
	err := query.Order("created_at DESC").Find(&projects).Error
	return projects, err
}

// GetProjectByID returns a project with all related data
func (s *ProjectService) GetProjectByID(id uint) (*models.Project, error) {
	var project models.Project
	err := s.db.
		Preload("Tasks", func(db *gorm.DB) *gorm.DB {
			return db.Where("parent_id IS NULL").Order("created_at ASC")
		}).
		Preload("Tasks.Children", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("Tasks.Dependencies.DependsOnTask").
		Preload("Tasks.Children.Dependencies.DependsOnTask").
		Preload("Dependencies.DependsOnProject").
		First(&project, id).Error

	if err != nil {
		return nil, fmt.Errorf("project not found")
	}
	return &project, nil
}

// CreateProject creates a new project with schedule validation
func (s *ProjectService) CreateProject(project *models.Project) error {
	// Validate schedule
	if err := s.scheduleService.ValidateSchedule(0, project.StartDate, project.EndDate); err != nil {
		return err
	}

	project.Status = "Draft"
	project.CompletionProgress = 0

	return s.db.Create(project).Error
}

// UpdateProject updates a project with validations
func (s *ProjectService) UpdateProject(id uint, input map[string]interface{}) (*models.Project, error) {
	var project models.Project
	if err := s.db.First(&project, id).Error; err != nil {
		return nil, fmt.Errorf("project not found")
	}

	// Handle schedule update — extract new dates from input for validation
	startDate := project.StartDate
	endDate := project.EndDate

	if v, ok := input["start_date"]; ok {
		if v == nil {
			startDate = nil
		} else if t, ok := v.(*time.Time); ok {
			startDate = t
		}
	}
	if v, ok := input["end_date"]; ok {
		if v == nil {
			endDate = nil
		} else if t, ok := v.(*time.Time); ok {
			endDate = t
		}
	}

	// Validate schedule with new dates
	if err := s.scheduleService.ValidateSchedule(id, startDate, endDate); err != nil {
		return nil, err
	}

	// Apply updates
	if err := s.db.Model(&project).Updates(input).Error; err != nil {
		return nil, err
	}

	// Reload with associations
	return s.GetProjectByID(id)
}

// DeleteProject deletes a project and all related data
func (s *ProjectService) DeleteProject(id uint) error {
	var project models.Project
	if err := s.db.First(&project, id).Error; err != nil {
		return fmt.Errorf("project not found")
	}

	// Delete all related task dependencies first
	s.db.Where("task_id IN (SELECT id FROM tasks WHERE project_id = ?)", id).Delete(&models.TaskDependency{})
	s.db.Where("depends_on_task_id IN (SELECT id FROM tasks WHERE project_id = ?)", id).Delete(&models.TaskDependency{})

	// Delete project dependencies
	s.db.Where("project_id = ? OR depends_on_project_id = ?", id, id).Delete(&models.ProjectDependency{})

	return s.db.Delete(&project).Error
}

// RecalculateProjectStatus calculates project status based on its tasks
func (s *ProjectService) RecalculateProjectStatus(projectID uint) error {
	var project models.Project
	if err := s.db.First(&project, projectID).Error; err != nil {
		return err
	}

	var tasks []models.Task
	s.db.Where("project_id = ?", projectID).Find(&tasks)

	if len(tasks) == 0 {
		project.Status = "Draft"
		project.CompletionProgress = 0
		return s.db.Save(&project).Error
	}

	// Calculate completion progress: (sum of done bobot / total bobot) * 100
	var totalBobot, doneBobot int
	allDraft := true
	anyInProgress := false
	allDone := true

	for _, task := range tasks {
		totalBobot += task.Bobot
		if task.Status == "Done" {
			doneBobot += task.Bobot
		}
		if task.Status != "Draft" {
			allDraft = false
		}
		if task.Status == "In Progress" {
			anyInProgress = true
		}
		if task.Status != "Done" {
			allDone = false
		}
	}

	// Calculate progress
	if totalBobot > 0 {
		project.CompletionProgress = float64(doneBobot) / float64(totalBobot) * 100
	} else {
		project.CompletionProgress = 0
	}

	// Determine status
	oldStatus := project.Status
	if allDone {
		project.Status = "Done"
	} else if anyInProgress || doneBobot > 0 {
		project.Status = "In Progress"
	} else if allDraft {
		project.Status = "Draft"
	}

	// Validate project dependency constraints
	if project.Status != "Draft" {
		if err := s.dependencyService.ValidateProjectStatusChange(projectID, project.Status); err != nil {
			// Cannot auto-advance because dependency not met, keep lower status
			if project.Status == "Done" {
				project.Status = "In Progress"
			}
			// If still fails, go to Draft
			if project.Status == "In Progress" {
				if err2 := s.dependencyService.ValidateProjectStatusChange(projectID, "In Progress"); err2 != nil {
					project.Status = "Draft"
				}
			}
		}
	}

	if err := s.db.Save(&project).Error; err != nil {
		return err
	}

	// If status changed, cascade to dependent projects
	if oldStatus != project.Status {
		s.dependencyService.CascadeProjectStatusChange(projectID, project.Status)
	}

	return nil
}
