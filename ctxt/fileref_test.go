package ctxt

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFileRefRoleHelpers(t *testing.T) {
	tests := []struct {
		name         string
		ref          FileRef
		userUpload   bool
		toolArtifact bool
		agentOutput  bool
		reference    bool
		artifact     bool
	}{
		{
			name:       "user upload",
			ref:        FileRef{Role: FileRoleUserUpload},
			userUpload: true,
		},
		{
			name:         "tool artifact",
			ref:          FileRef{Role: FileRoleToolArtifact},
			toolArtifact: true,
			artifact:     true,
		},
		{
			name:        "agent output",
			ref:         FileRef{Role: FileRoleAgentOutput},
			agentOutput: true,
			artifact:    true,
		},
		{
			name:      "reference",
			ref:       FileRef{Role: FileRoleReference},
			reference: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ref.IsUserUpload(); got != tt.userUpload {
				t.Fatalf("IsUserUpload() = %v, want %v", got, tt.userUpload)
			}
			if got := tt.ref.IsToolArtifact(); got != tt.toolArtifact {
				t.Fatalf("IsToolArtifact() = %v, want %v", got, tt.toolArtifact)
			}
			if got := tt.ref.IsAgentOutput(); got != tt.agentOutput {
				t.Fatalf("IsAgentOutput() = %v, want %v", got, tt.agentOutput)
			}
			if got := tt.ref.IsReference(); got != tt.reference {
				t.Fatalf("IsReference() = %v, want %v", got, tt.reference)
			}
			if got := tt.ref.IsArtifact(); got != tt.artifact {
				t.Fatalf("IsArtifact() = %v, want %v", got, tt.artifact)
			}
		})
	}
}

func TestFileRefRoleJSONRoundTripAndOmitEmpty(t *testing.T) {
	withRole := FileRef{Path: "uploads/a.txt", Role: FileRoleUserUpload}
	dataWithRole, err := json.Marshal(withRole)
	if err != nil {
		t.Fatalf("Marshal(withRole): %v", err)
	}
	if !strings.Contains(string(dataWithRole), `"role":"user_upload"`) {
		t.Fatalf("expected role field in JSON, got %s", string(dataWithRole))
	}
	var decoded FileRef
	if err := json.Unmarshal(dataWithRole, &decoded); err != nil {
		t.Fatalf("Unmarshal(withRole): %v", err)
	}
	if decoded.Role != FileRoleUserUpload {
		t.Fatalf("decoded role = %q, want %q", decoded.Role, FileRoleUserUpload)
	}

	withoutRole := FileRef{Path: "uploads/b.txt"}
	dataWithoutRole, err := json.Marshal(withoutRole)
	if err != nil {
		t.Fatalf("Marshal(withoutRole): %v", err)
	}
	if strings.Contains(string(dataWithoutRole), `"role":`) {
		t.Fatalf("expected role to be omitted, got %s", string(dataWithoutRole))
	}
}
