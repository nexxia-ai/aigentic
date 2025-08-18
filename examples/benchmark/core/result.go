package core

import "time"

// BenchResult contains the results of running an example/benchmark
type BenchResult struct {
	TestCase     string                 `json:"test_case"`
	ModelName    string                 `json:"model_name"`
	Success      bool                   `json:"success"`
	Duration     time.Duration          `json:"duration"`
	ResponseSize int                    `json:"response_size,omitempty"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}
