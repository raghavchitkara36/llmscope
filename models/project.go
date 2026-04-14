package models

import "time"

// project represents a project in the system, it can be a table name in the database,
// basically done to segregate the traces for different use cases/projects accoding to the requirement
// Project is a logical grouping of traces — stored in the projects table

type Project struct {
	ProjectID   string `json:"project_id" db:"project_id"`
	ProjectName string `json:"project_name" db:"project_name"`
	CreatedAt   int64  `json:"created_at"   db:"created_at"`
}

func NewProject(name string) *Project {
	return &Project{
		ProjectID:   generateID(),
		ProjectName: name,
		CreatedAt:   time.Now().UnixMilli(),
	}
}
