package tools

// Example usage of the Python Sandbox Tool
//
// This file demonstrates how to use the Python sandbox tool with an agent.
// The tool allows LLMs to execute Python code dynamically and receive results.
//
// Basic usage in an agent configuration:
//
//	agent := aigentic.Agent{
//	    Name:        "data-analyst",
//	    Description: "You are a data analyst that can run Python code for calculations",
//	    Model:       model,
//	    AgentTools:  []aigentic.AgentTool{tools.NewPythonSandboxTool()},
//	}
//
//	// Now the agent can execute Python code when needed
//	response, err := agent.Run(ctx, "Calculate the fibonacci sequence up to 10")
//
// Example prompts that will trigger the tool:
//
//  1. "Calculate the sum of squares from 1 to 100 using Python"
//     The LLM will generate:
//     {
//       "code": "result = sum(i**2 for i in range(1, 101))\nprint(f'Sum: {result}')"
//     }
//
//  2. "Process this data and find the average: [23, 45, 67, 89, 12]"
//     The LLM will generate:
//     {
//       "code": "data = [23, 45, 67, 89, 12]\naverage = sum(data) / len(data)\nprint(f'Average: {average}')"
//     }
//
//  3. "Write a Python function to check if a number is prime and test it with 17"
//     The LLM will generate:
//     {
//       "code": "def is_prime(n):\n    if n < 2:\n        return False\n    for i in range(2, int(n**0.5) + 1):\n        if n % i == 0:\n            return False\n    return True\n\nresult = is_prime(17)\nprint(f'17 is prime: {result}')",
//       "timeout": 10
//     }
//
// Tool Parameters:
//   - code (required): The Python code to execute
//   - timeout (optional): Execution timeout in seconds (default: 30, max: 300)
//
// Security Considerations:
//   - Code runs in a subprocess with timeout enforcement
//   - No network access restrictions (use with caution)
//   - Isolated temporary directory for each execution
//   - Clean environment (minimal env vars)
//
// Future Enhancements:
//   - Docker-based execution for stronger isolation
//   - Network access controls
//   - Resource limits (CPU, memory)
//   - Pre-installed package support
//
// The tool interface is designed to remain unchanged when migrating to Docker,
// only the internal executePythonCode() function needs to be replaced.
