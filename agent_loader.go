package aigentic

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/run"
	"gopkg.in/yaml.v3"
)

// AgentConfig is a flat, serializable definition of an Agent for YAML/JSON.
// Keep top-level only: no nested structs; references by name.
type AgentConfig struct {
	Name               string   `yaml:"name" json:"name"`
	ModelName          string   `yaml:"model_name" json:"model_name"`
	Description        string   `yaml:"description" json:"description"`
	Instructions       string   `yaml:"instructions" json:"instructions"`
	OutputInstructions string   `yaml:"output_instructions" json:"output_instructions"`
	IncludeHistory     bool     `yaml:"conversation_history" json:"conversation_history"`
	Retries            int      `yaml:"retries" json:"retries"`
	Stream             bool     `yaml:"stream" json:"stream"`
	LogLevel           string   `yaml:"log_level" json:"log_level"`
	MaxLLMCalls        int      `yaml:"max_llm_calls" json:"max_llm_calls"`
	EnableEvaluation   bool     `yaml:"enable_evaluation" json:"enable_evaluation"`
	EnableTrace        bool     `yaml:"enable_trace" json:"enable_trace"`
	Tools              []string `yaml:"tools" json:"tools"`
	Agents             []string `yaml:"agents" json:"agents"`
}

// ConfigFile is the root document for config serialization.
type ConfigFile struct {
	Tools  map[string]ai.ServerConfig `yaml:"tools" json:"tools"`
	Agents []AgentConfig              `yaml:"agents" json:"agents"`
}

// LoadConfigFile parses a YAML config file into ConfigFile and validates basic constraints.
func LoadConfigFile(path string) (*ConfigFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return DecodeConfigYAML(f)
}

// DecodeConfigYAML decodes YAML from an io.Reader for tests and programmatic use.
func DecodeConfigYAML(r io.Reader) (*ConfigFile, error) {
	var cfg ConfigFile
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, err
	}
	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func validateConfig(cfg *ConfigFile) error {
	const defaultRetries = 2
	const defaultMaxLLMCalls = 20

	validAgents := make([]AgentConfig, 0, len(cfg.Agents))
	names := map[string]struct{}{}

	for _, a := range cfg.Agents {
		if a.Name == "" {
			slog.Error("skipping agent: missing name")
			continue
		}

		if _, ok := names[a.Name]; ok {
			slog.Error("skipping agent: duplicate name", "name", a.Name)
			continue
		}

		names[a.Name] = struct{}{}
		validAgent := a

		if a.Retries < 0 {
			slog.Error("agent has negative retries, using default", "agent", a.Name, "retries", a.Retries, "default", defaultRetries)
			validAgent.Retries = defaultRetries
		}

		if a.MaxLLMCalls < 0 {
			slog.Error("agent has negative max_llm_calls, using default", "agent", a.Name, "max_llm_calls", a.MaxLLMCalls, "default", defaultMaxLLMCalls)
			validAgent.MaxLLMCalls = defaultMaxLLMCalls
		}

		switch a.LogLevel {
		case "", "debug", "info", "warn", "error":
		default:
			slog.Error("agent has invalid log_level, using info as default", "agent", a.Name, "log_level", a.LogLevel)
			validAgent.LogLevel = "info"
		}

		if a.ModelName == "" {
			slog.Warn("agent missing model_name, will use default model", "agent", a.Name)
		}

		validTools := make([]string, 0, len(a.Tools))
		for _, t := range a.Tools {
			if _, ok := cfg.Tools[t]; !ok {
				slog.Error("agent references unknown tool, skipping", "agent", a.Name, "tool", t)
				continue
			}
			validTools = append(validTools, t)
		}
		validAgent.Tools = validTools

		validChildAgents := make([]string, 0, len(a.Agents))
		for _, child := range a.Agents {
			if child == a.Name {
				slog.Error("agent references itself, skipping reference", "agent", a.Name)
				continue
			}
			validChildAgents = append(validChildAgents, child)
		}
		validAgent.Agents = validChildAgents

		validAgents = append(validAgents, validAgent)
	}

	names = make(map[string]struct{})
	for _, a := range validAgents {
		names[a.Name] = struct{}{}
	}

	finalAgents := make([]AgentConfig, 0, len(validAgents))
	for _, a := range validAgents {
		validChildAgents := make([]string, 0, len(a.Agents))
		for _, child := range a.Agents {
			if _, ok := names[child]; !ok {
				slog.Error("agent references unknown agent, skipping reference", "agent", a.Name, "child", child)
				continue
			}
			validChildAgents = append(validChildAgents, child)
		}
		a.Agents = validChildAgents
		finalAgents = append(finalAgents, a)
	}

	cfg.Agents = finalAgents
	return nil
}

// InstantiateAgents builds runtime Agents from the ConfigFile using a two-pass approach.
// Pass 1: create Agent instances with basic fields and map by name.
// Pass 2: wire sub-agents and attach tools (by name) via the provided toolResolver.
// - modelResolver: given a model name string, return *ai.Model (or error)
// - toolResolver: given a tool server name and its ai.ServerConfig, return []AgentTool to attach
func (cfg *ConfigFile) InstantiateAgents(
	modelResolver func(string) (*ai.Model, error),
	toolResolver func(name string, sc ai.ServerConfig) ([]run.AgentTool, error),
) (map[string]Agent, error) {
	// Pass 1: instantiate top-level agents without sub-agents
	out := make(map[string]Agent)
	for _, ac := range cfg.Agents {
		m, err := modelResolver(ac.ModelName)
		if err != nil {
			return nil, fmt.Errorf("agent %s model resolve failed: %w", ac.Name, err)
		}
		a := Agent{
			Model:              m,
			Name:               ac.Name,
			Description:        ac.Description,
			Instructions:       ac.Instructions,
			OutputInstructions: ac.OutputInstructions,
			Retries:            ac.Retries,
			Stream:             ac.Stream,
			MaxLLMCalls:        ac.MaxLLMCalls,
			EnableEvaluation:   ac.EnableEvaluation,
			EnableTrace:        ac.EnableTrace,
			IncludeHistory:     ac.IncludeHistory,
		}
		// Map log level strings to slog.Level
		switch ac.LogLevel {
		case "debug":
			a.LogLevel = slog.LevelDebug
		case "info":
			a.LogLevel = slog.LevelInfo
		case "warn":
			a.LogLevel = slog.LevelWarn
		case "error":
			a.LogLevel = slog.LevelError
		default:
			// leave zero value if empty/unknown; validateConfig covers invalid
		}

		// Attach tools from tool servers listed by name using resolver
		for _, tname := range ac.Tools {
			sc := cfg.Tools[tname]
			ats, err := toolResolver(tname, sc)
			if err != nil {
				return nil, fmt.Errorf("agent %s tool resolve failed for %s: %w", ac.Name, tname, err)
			}
			a.AgentTools = append(a.AgentTools, ats...)
		}
		out[ac.Name] = a
	}
	// Pass 2: wire sub-agents by name
	for _, ac := range cfg.Agents {
		parent := out[ac.Name]
		parent.Agents = nil
		for _, child := range ac.Agents {
			ca, ok := out[child]
			if !ok {
				return nil, fmt.Errorf("agent %s references unknown agent: %s", ac.Name, child)
			}
			parent.Agents = append(parent.Agents, ca)
		}
		out[ac.Name] = parent
	}
	return out, nil
}
