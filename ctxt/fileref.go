package ctxt

import (
	"encoding/json"
	"time"
)

const (
	FileRoleUserUpload   = "user_upload"
	FileRoleToolArtifact = "tool_artifact"
	FileRoleAgentOutput  = "agent_output"
	FileRoleReference    = "reference"
)

// FileRef is the canonical attachment type persisted on the turn.
// Caller metadata (e.g. visible_to_user, source, derived_from) lives in Meta().
type FileRef struct {
	BasePath        string    `json:"base_path,omitempty"`
	Path            string    `json:"path"`
	MimeType        string    `json:"mime_type,omitempty"`
	Role            string    `json:"role,omitempty"`
	SizeBytes       int64     `json:"size_bytes,omitempty"`
	AddedAt         time.Time `json:"added_at,omitempty"`
	ToolID          string    `json:"tool_id,omitempty"`
	IncludeInPrompt bool      `json:"include_in_prompt"`
	Ephemeral       bool      `json:"ephemeral"`
	metadata        map[string]string
}

func (f FileRef) IsUserUpload() bool {
	return f.Role == FileRoleUserUpload
}

func (f FileRef) IsToolArtifact() bool {
	return f.Role == FileRoleToolArtifact
}

func (f FileRef) IsAgentOutput() bool {
	return f.Role == FileRoleAgentOutput
}

func (f FileRef) IsReference() bool {
	return f.Role == FileRoleReference
}

func (f FileRef) IsArtifact() bool {
	return f.IsToolArtifact() || f.IsAgentOutput()
}

func (f *FileRef) SetMeta(meta map[string]string) {
	if f.metadata == nil {
		f.metadata = make(map[string]string)
	}
	for k, v := range meta {
		if v == "" {
			delete(f.metadata, k)
		} else {
			f.metadata[k] = v
		}
	}
	if len(f.metadata) == 0 {
		f.metadata = nil
	}
}

func (f *FileRef) Meta() map[string]string {
	if len(f.metadata) == 0 {
		return nil
	}
	out := make(map[string]string, len(f.metadata))
	for k, v := range f.metadata {
		out[k] = v
	}
	return out
}

func (f *FileRef) GetMeta(key string) string {
	if f.metadata == nil {
		return ""
	}
	return f.metadata[key]
}

func (f *FileRef) MarshalJSON() ([]byte, error) {
	type Alias FileRef
	aux := &struct {
		*Alias
		Metadata map[string]string `json:"metadata,omitempty"`
	}{
		Alias: (*Alias)(f),
	}
	if len(f.metadata) > 0 {
		aux.Metadata = f.metadata
	}
	return json.Marshal(aux)
}

func (f *FileRef) UnmarshalJSON(data []byte) error {
	type Alias FileRef
	aux := &struct {
		*Alias
		Metadata map[string]string `json:"metadata,omitempty"`
	}{
		Alias: (*Alias)(f),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if len(aux.Metadata) > 0 {
		f.metadata = aux.Metadata
	}
	return nil
}
