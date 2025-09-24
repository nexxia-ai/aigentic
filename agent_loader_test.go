package aigentic

import (
	"strings"
	"testing"

	"github.com/nexxia-ai/aigentic/ai"
)

func TestDecodeConfigYAML_Valid(t *testing.T) {
	yaml := `
tools:
  customer_service:
    command: "uvx"
    args: ["mcp-server-customer-service"]
  billing:
    command: "go"
    args: ["run", "github.com/company/mcp-billing-server@latest"]
  technical:
    command: "uvx"
    args: ["mcp-server-technical"]

agents:
  - name: "customer_service_agent"
    model_name: "gpt-4"
    description: "Handles customer inquiries and support requests"
    instructions: "Be polite and professional, escalate complex issues"
    include_history: true
    retries: 2
    stream: true
    log_level: "info"
    max_llm_calls: 30
    enable_evaluation: true
    tools: ["customer_service", "billing", "technical"]
    agents: ["billing_specialist", "technical_support"]

  - name: "billing_specialist"
    model_name: "gpt-3.5-turbo"
    description: "Specialized agent for billing inquiries"
    instructions: "Handle billing questions and payment issues"
    include_history: false
    retries: 1
    stream: false
    log_level: "warn"
    max_llm_calls: 20
    enable_evaluation: false
    tools: ["billing"]

  - name: "technical_support"
    model_name: "gpt-4"
    description: "Handles technical support requests"
    instructions: "Provide technical solutions and troubleshooting steps"
    include_history: true
    retries: 2
    stream: false
    log_level: "info"
    max_llm_calls: 40
    enable_evaluation: true
    tools: ["technical"]
`

	cfg, err := DecodeConfigYAML(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(cfg.Tools))
	}
	if len(cfg.Agents) != 3 {
		t.Fatalf("expected 3 agents, got %d", len(cfg.Agents))
	}

	// spot check a few fields
	if cfg.Agents[0].Name != "customer_service_agent" {
		t.Errorf("unexpected first agent name: %s", cfg.Agents[0].Name)
	}
	if cfg.Agents[0].LogLevel != "info" {
		t.Errorf("expected info, got %s", cfg.Agents[0].LogLevel)
	}
}

func TestInstantiateAgents_TwoPass(t *testing.T) {
	yaml := `
tools:
  t:
    command: x
    args: [y]
agents:
  - name: parent
    model_name: gpt-4
    tools: ["t"]
    agents: ["child"]
  - name: child
    model_name: gpt-4
    tools: ["t"]
`
	cfg, err := DecodeConfigYAML(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	modelResolver := func(s string) (*ai.Model, error) { return &ai.Model{ModelName: s}, nil }
	toolResolver := func(name string, sc ai.ServerConfig) ([]AgentTool, error) { return []AgentTool{}, nil }

	agents, err := cfg.InstantiateAgents(modelResolver, toolResolver)
	if err != nil {
		t.Fatalf("unexpected instantiate error: %v", err)
	}
	p, ok := agents["parent"]
	if !ok {
		t.Fatalf("parent not found")
	}
	if len(p.Agents) != 1 || p.Agents[0].Name != "child" {
		t.Fatalf("expected parent to have child wired, got %+v", p.Agents)
	}
}

func TestDecodeConfigYAML_UnknownTool(t *testing.T) {
	yaml := `
tools: {}
agents:
  - name: a
    model_name: gpt-4
    tools: ["missing"]
`
	_, err := DecodeConfigYAML(strings.NewReader(yaml))
	if err == nil {
		t.Fatalf("expected error for unknown tool, got nil")
	}
}

func TestDecodeConfigYAML_UnknownAgentRef(t *testing.T) {
	yaml := `
tools:
  t:
    command: x
    args: [y]
agents:
  - name: a
    model_name: gpt-4
    tools: ["t"]
    agents: ["b"]
`
	_, err := DecodeConfigYAML(strings.NewReader(yaml))
	if err == nil {
		t.Fatalf("expected error for unknown agent ref, got nil")
	}
}

func TestDecodeConfigYAML_InvalidLogLevel(t *testing.T) {
	yaml := `
tools:
  t:
    command: x
    args: [y]
agents:
  - name: a
    model_name: gpt-4
    log_level: noisy
    tools: ["t"]
`
	_, err := DecodeConfigYAML(strings.NewReader(yaml))
	if err == nil {
		t.Fatalf("expected error for invalid log level, got nil")
	}
}
