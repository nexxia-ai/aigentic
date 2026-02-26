package run

import (
	"context"
	"strings"
	"testing"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/event"
)

func TestExecuteReadFileSuccessAndErrors(t *testing.T) {
	ar, err := NewAgentRun("test-agent", "desc", "inst", t.TempDir())
	if err != nil {
		t.Fatalf("NewAgentRun error: %v", err)
	}

	if err := ar.AgentContext().UploadDocument("skills/skill-one.md", []byte("skill one content"), "text/markdown"); err != nil {
		t.Fatalf("UploadDocument error: %v", err)
	}

	okResult := ar.executeReadFile(map[string]interface{}{"path": "skills/skill-one.md"})
	if okResult == nil || okResult.Result == nil || okResult.Result.Error {
		t.Fatalf("expected success read_file result, got %+v", okResult)
	}
	if got := okResult.Result.Content[0].Content.(string); !strings.Contains(got, "skill one content") {
		t.Fatalf("expected file content, got %q", got)
	}

	missing := ar.executeReadFile(map[string]interface{}{"path": "missing.md"})
	if missing == nil || missing.Result == nil || !missing.Result.Error {
		t.Fatalf("expected missing file to be error result, got %+v", missing)
	}

	abs := ar.executeReadFile(map[string]interface{}{"path": "/tmp/file.md"})
	if abs == nil || abs.Result == nil || !abs.Result.Error {
		t.Fatalf("expected absolute path to be error result, got %+v", abs)
	}

	trav := ar.executeReadFile(map[string]interface{}{"path": "../outside.md"})
	if trav == nil || trav.Result == nil || !trav.Result.Error {
		t.Fatalf("expected traversal path to be error result, got %+v", trav)
	}
}

func TestExecuteReadFileTruncatesLargeContent(t *testing.T) {
	ar, err := NewAgentRun("test-agent", "desc", "inst", t.TempDir())
	if err != nil {
		t.Fatalf("NewAgentRun error: %v", err)
	}

	large := strings.Repeat("a", maxReadFileBytes+200)
	if err := ar.AgentContext().UploadDocument("skills/large.md", []byte(large), "text/markdown"); err != nil {
		t.Fatalf("UploadDocument error: %v", err)
	}

	result := ar.executeReadFile(map[string]interface{}{"path": "skills/large.md"})
	if result == nil || result.Result == nil || result.Result.Error {
		t.Fatalf("expected success read_file result, got %+v", result)
	}
	got := result.Result.Content[0].Content.(string)
	if !strings.Contains(got, "[truncated to") {
		t.Fatalf("expected truncation notice, got %q", got)
	}
}

func TestReadFileToolExposureAndDedup(t *testing.T) {
	ar, err := NewAgentRun("test-agent", "desc", "inst", t.TempDir())
	if err != nil {
		t.Fatalf("NewAgentRun error: %v", err)
	}
	ar.SetModel(ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
		return ai.AIMessage{Role: ai.AssistantRole, Content: "ok"}, nil
	}))

	if err := ar.AgentContext().UploadDocument("skills/skill-one.md", []byte("skill one content"), "text/markdown"); err != nil {
		t.Fatalf("UploadDocument error: %v", err)
	}

	for i := 0; i < 2; i++ {
		ar.Run(context.Background(), "hello")
		foundCount := 0
		for evt := range ar.Next() {
			llm, ok := evt.(*event.LLMCallEvent)
			if !ok {
				continue
			}
			for _, tool := range llm.Tools {
				if tool.Name == readFileToolName {
					foundCount++
				}
			}
		}
		if foundCount != 1 {
			t.Fatalf("expected exactly 1 read_file tool exposure, got %d on run %d", foundCount, i+1)
		}
	}
}

