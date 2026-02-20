package models

import (
	"time"
)

type Project struct {
	ID                 uint               `json:"id" gorm:"primaryKey"`
	Name               string             `json:"name" gorm:"type:varchar(255);not null"`
	Status             string             `json:"status" gorm:"type:enum('Draft','In Progress','Done');default:'Draft'"`
	CompletionProgress float64            `json:"completion_progress" gorm:"type:decimal(5,2);default:0"`
	StartDate          *time.Time         `json:"start_date" gorm:"type:date"`
	EndDate            *time.Time         `json:"end_date" gorm:"type:date"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
	Tasks              []Task             `json:"tasks,omitempty" gorm:"foreignKey:ProjectID;constraint:OnDelete:CASCADE"`
	Dependencies       []ProjectDependency `json:"dependencies,omitempty" gorm:"foreignKey:ProjectID"`
}

func (Project) TableName() string {
	return "projects"
}

type ProjectDependency struct {
	ID                 uint      `json:"id" gorm:"primaryKey"`
	ProjectID          uint      `json:"project_id" gorm:"not null"`
	DependsOnProjectID uint      `json:"depends_on_project_id" gorm:"not null"`
	DependsOnProject   *Project  `json:"depends_on_project,omitempty" gorm:"foreignKey:DependsOnProjectID"`
	CreatedAt          time.Time `json:"created_at"`
}

func (ProjectDependency) TableName() string {
	return "project_dependencies"
}
