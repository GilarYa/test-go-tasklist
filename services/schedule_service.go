package services

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"test-go-tasklist/models"
)

type ScheduleService struct {
	db *gorm.DB
}

func NewScheduleService(db *gorm.DB) *ScheduleService {
	return &ScheduleService{db: db}
}

// ValidateSchedule checks that the project's schedule doesn't overlap with any other project
func (s *ScheduleService) ValidateSchedule(projectID uint, startDate, endDate *time.Time) error {
	if startDate == nil || endDate == nil {
		return nil // No schedule set, no conflict possible
	}

	if startDate.After(*endDate) {
		return fmt.Errorf("start_date tidak boleh setelah end_date")
	}

	// Find overlapping projects: startA < endB AND startB < endA
	var conflictingProjects []models.Project
	query := s.db.Where(
		"id != ? AND start_date IS NOT NULL AND end_date IS NOT NULL AND start_date < ? AND end_date > ?",
		projectID, endDate, startDate,
	)

	if err := query.Find(&conflictingProjects).Error; err != nil {
		return fmt.Errorf("gagal mengecek konflik jadwal: %v", err)
	}

	if len(conflictingProjects) > 0 {
		conflictInfo := ""
		for i, p := range conflictingProjects {
			if i > 0 {
				conflictInfo += ", "
			}
			startStr := ""
			endStr := ""
			if p.StartDate != nil {
				startStr = p.StartDate.Format("2006-01-02")
			}
			if p.EndDate != nil {
				endStr = p.EndDate.Format("2006-01-02")
			}
			conflictInfo += fmt.Sprintf("'%s' (%s s/d %s)", p.Name, startStr, endStr)
		}
		return fmt.Errorf("Jadwal bentrok dengan project %s. Silakan pilih tanggal yang tidak beririsan.", conflictInfo)
	}

	return nil
}
