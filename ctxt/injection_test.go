package ctxt

import (
	"strings"
	"testing"
)

func TestRenderInjectedTextTruncatesAndOmitsAfterBudget(t *testing.T) {
	policy := InjectionPolicy{MaxBytesPerFile: 4, MaxBytesPerTurn: 128}

	truncated := RenderInjectedText("large.txt", []byte("abcdefghij"), policy, 0)
	if !truncated.Included || !truncated.Truncated || truncated.Omitted {
		t.Fatalf("expected truncated included result: %+v", truncated)
	}
	if !strings.Contains(truncated.Text, "abcd") {
		t.Fatalf("expected rendered text to include prefix: %q", truncated.Text)
	}
	if !strings.Contains(truncated.Text, "truncated; 10 bytes total") {
		t.Fatalf("expected truncated marker: %q", truncated.Text)
	}

	omitted := RenderInjectedText("later.txt", []byte("abc"), policy, 128)
	if !omitted.Omitted || omitted.Included {
		t.Fatalf("expected omitted result after budget: %+v", omitted)
	}
}
