package ai

import (
	"reflect"
	"testing"
)

func TestNewMCPHost(t *testing.T) {
	tests := []struct {
		name    string
		want    *MCPHost
		wantErr bool
	}{
		{name: "test1", want: &MCPHost{}, wantErr: false},
	}

	config := &MCPConfig{
		MCPServers: map[string]ServerConfig{
			"test": {
				Command: "go",
				// Command: "C:\\Users\\User\\src\\rag\\langchaingo\\testserver\\testserver.exe",
				Args: []string{"run", "C:\\Users\\User\\src\\rag\\langchaingo\\testserver\\main.go"},
				// Env:  map[string]string{},
			},
			"mcp-server": {
				Command: "mcp-filesystem-server",
				Args: []string{
					"C:\\Users\\User\\Downloads",
					"c:"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewMCPHost(config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewMCPHost() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewMCPHost() = %v, want %v", got, tt.want)
			}
		})
	}
}
