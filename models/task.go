package models

import (
	"time"
)

type Task struct {
	ID           uint             `json:"id" gorm:"primaryKey"`
	Name         string           `json:"name" gorm:"type:varchar(255);not null"`
	Status       string           `json:"status" gorm:"type:enum('Draft','In Progress','Done');default:'Draft'"`
	ProjectID    uint             `json:"project_id" gorm:"not null"`
	ParentID     *uint            `json:"parent_id" gorm:"default:null"`
	Bobot        int              `json:"bobot" gorm:"default:1"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
	Project      *Project         `json:"project,omitempty" gorm:"foreignKey:ProjectID"`
	Parent       *Task            `json:"parent,omitempty" gorm:"foreignKey:ParentID"`
	Children     []Task           `json:"children,omitempty" gorm:"foreignKey:ParentID"`
	Dependencies []TaskDependency `json:"dependencies,omitempty" gorm:"foreignKey:TaskID"`
}

func (Task) TableName() string {
	return "tasks"
}

type TaskDependency struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	TaskID          uint      `json:"task_id" gorm:"not null"`
	DependsOnTaskID uint      `json:"depends_on_task_id" gorm:"not null"`
	DependsOnTask   *Task     `json:"depends_on_task,omitempty" gorm:"foreignKey:DependsOnTaskID"`
	CreatedAt       time.Time `json:"created_at"`
}

func (TaskDependency) TableName() string {
	return "task_dependencies"
}
