package ctxt

import "fmt"

const (
	defaultMaxInjectionBytesPerFile = 32 * 1024
	defaultMaxInjectionBytesPerTurn = 128 * 1024
)

type InjectionPolicy struct {
	MaxBytesPerFile int
	MaxBytesPerTurn int
}

type InjectionResult struct {
	Text      string
	Included  bool
	Truncated bool
	Omitted   bool
}

func DefaultInjectionPolicy() InjectionPolicy {
	return InjectionPolicy{
		MaxBytesPerFile: defaultMaxInjectionBytesPerFile,
		MaxBytesPerTurn: defaultMaxInjectionBytesPerTurn,
	}
}

func (p InjectionPolicy) normalized() InjectionPolicy {
	if p.MaxBytesPerFile <= 0 {
		p.MaxBytesPerFile = defaultMaxInjectionBytesPerFile
	}
	if p.MaxBytesPerTurn <= 0 {
		p.MaxBytesPerTurn = defaultMaxInjectionBytesPerTurn
	}
	return p
}

func RenderInjectedText(path string, data []byte, policy InjectionPolicy, usedBytes int) InjectionResult {
	policy = policy.normalized()
	remaining := policy.MaxBytesPerTurn - usedBytes
	if remaining <= 0 {
		return InjectionResult{Omitted: true}
	}

	total := len(data)
	if total <= policy.MaxBytesPerFile && total <= remaining {
		return InjectionResult{Text: string(data), Included: true}
	}

	marker := fmt.Sprintf(
		"\n... (truncated; %d bytes total; load via filesystem.read_text(%q))",
		total,
		path,
	)
	limit := min(total, policy.MaxBytesPerFile, remaining-len(marker))
	if limit <= 0 {
		return InjectionResult{Omitted: true}
	}
	return InjectionResult{Text: string(data[:limit]) + marker, Included: true, Truncated: true}
}
