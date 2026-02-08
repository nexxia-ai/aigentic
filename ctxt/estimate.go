package ctxt

// EstimateTokens estimates token count for a string using a simple heuristic.
// Assumes ~4 characters per token (common for English text).
func EstimateTokens(s string) int {
	if s == "" {
		return 0
	}
	return len(s) / 4
}

// EstimateSystemAndMemoryTokens estimates token counts for system prompt and memory docs.
// Returns (systemTokens, memoryTokens). System excludes memory docs; memory is memory docs only.
func (ac *AgentContext) EstimateSystemAndMemoryTokens() (int, int) {
	memoryTokens := 0
	if ac.workspace != nil {
		docs, err := ac.workspace.MemoryFiles()
		if err == nil && docs != nil {
			for _, doc := range docs {
				memoryTokens += EstimateTokens(doc.Text())
			}
		}
	}

	sysMsg, err := createSystemMsg(ac, nil)
	if err != nil {
		return 0, memoryTokens
	}

	systemTokens := 0
	if sysMsg != nil {
		_, content := sysMsg.Value()
		fullTokens := EstimateTokens(content)
		if fullTokens > memoryTokens {
			systemTokens = fullTokens - memoryTokens
		}
	}

	return systemTokens, memoryTokens
}
