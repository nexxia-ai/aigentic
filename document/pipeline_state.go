package document

import (
	"time"
)

type PipelineStatus string

const (
	StatusPending   PipelineStatus = "pending"
	StatusRunning   PipelineStatus = "running"
	StatusPaused    PipelineStatus = "paused"
	StatusCompleted PipelineStatus = "completed"
	StatusFailed    PipelineStatus = "failed"
)

type StageState struct {
	Name         string         `json:"name"`
	Status       PipelineStatus `json:"status"`
	OutputDocIDs []string       `json:"output_doc_ids"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
	Error        string         `json:"error,omitempty"`
}

type PipelineState struct {
	ID           string       `json:"id"`
	Status       PipelineStatus `json:"status"`
	CurrentStage int          `json:"current_stage"`
	Stages       []StageState `json:"stages"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

