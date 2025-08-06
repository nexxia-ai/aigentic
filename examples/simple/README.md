# Aigentic Agent Showcase

# this is a private repo for now, you need this:
$env:GOPRIVATE="github.com/nexxia-ai/**"

This showcase demonstrates four different types of agents using the Aigentic framework with the OpenAI provider.

## Agents

1. **Simple Agent** (`simple_agent.go`)
   - Basic agent that responds to user messages
   - No special features, just conversation

2. **Tool Agent** (`tool_agent.go`)
   - Agent with a custom calculator tool
   - Can perform mathematical calculations

3. **Attachment Agent** (`attachment_agent.go`)
   - Agent that can work with file attachments
   - Analyzes text documents

4. **Multi Agent** (`multi_agent.go`)
   - Agent that uses another agent as a sub-agent
   - Demonstrates agent coordination

## Setup

1. Set your OpenAI API key:
   ```bash
   export OPENAI_API_KEY=your_api_key_here
   ```

2. Install dependencies:
   ```bash
   go mod tidy
   ```

3. Run the showcase:
   ```bash
   go run .
   ```

## What You'll See

The showcase will run each agent in sequence and display their responses:

- Simple Agent: Responds to a space-related question
- Tool Agent: Uses a calculator tool to solve a math problem
- Attachment Agent: Analyzes a text file attachment
- Multi Agent: Coordinates with a research sub-agent

## Learning Points

- How to create basic agents
- How to add custom tools to agents
- How to work with file attachments
- How to create multi-agent systems
- How to use the OpenAI provider with Aigentic 