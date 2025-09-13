# Memory Enhancement Specification for Aigentic Framework

## Overview

This specification outlines improvements to the memory functionality in the aigentic framework to provide a compartmentalized, robust, and user-friendly memory system for LLM agents working on complex multi-step tasks.

## Goals

### Primary Objectives
1. **Compartmentalized Memory**: Create distinct memory sections for different purposes
2. **Enhanced Memory Operations**: Provide comprehensive CRUD operations for each memory section
3. **Memory Persistence**: Enable memory to persist across agent runs and sessions
4. **Memory Organization**: Support structured memory with categories, tags, and metadata
5. **Memory Retrieval**: Enable intelligent search and retrieval of relevant memory
6. **Memory Management**: Provide tools for memory cleanup, archiving, and lifecycle management

### Secondary Objectives
1. **Performance Optimization**: Efficient memory storage and retrieval
2. **Memory Validation**: Structure validation and content validation
3. **Memory Analytics**: Usage tracking and memory effectiveness metrics
4. **Memory Sharing**: Enable memory sharing between agents and sessions
5. **Simple Developer Experience**: Keep the API simple and intuitive

## Proposed Memory Architecture

### Memory Compartments

#### 1. Run Memory
- **Purpose**: Memory available in every LLM call during a single agent run
- **Scope**: Single agent run (not shared with sub-agents)
- **Use Cases**: 
  - Current task state and progress
  - Intermediate results and calculations
  - Temporary context for the current run
  - Real-time decision making data
- **Persistence**: Cleared at the end of each agent run
- **Access**: Automatically included in every LLM call during the run

#### 2. Session Memory
- **Purpose**: Memory shared across multiple agent calls within a session
- **Scope**: Entire session (shared with sub-agents)
- **Use Cases**:
  - User preferences and settings
  - Session-level context and state
  - Information shared between main agent and sub-agents
  - Session goals and objectives
- **Persistence**: Persists across agent runs within the same session
- **Access**: Available to all agents in the session

#### 3. Plan Memory
- **Purpose**: Memory for storing and tracking complex multi-step plans
- **Scope**: Can be run-level or session-level depending on plan scope
- **Use Cases**:
  - Multi-step task plans and strategies
  - Progress tracking and status updates
  - Plan modifications and adaptations
  - Task dependencies and relationships
- **Persistence**: Configurable (run-level or session-level)
- **Access**: Available to agents working on the plan

### Core Memory Types

#### 1. MemoryEntry
```go
type MemoryEntry struct {
    ID          string            `json:"id"`
    Content     string            `json:"content"`
    Category    string            `json:"category,omitempty"`
    Tags        []string          `json:"tags,omitempty"`
    Metadata    map[string]string `json:"metadata,omitempty"`
    CreatedAt   time.Time         `json:"created_at"`
    UpdatedAt   time.Time         `json:"updated_at"`
    AccessCount int               `json:"access_count"`
    Priority    int               `json:"priority"` // 1-10, higher = more important
}
```

### Memory Storage

#### FileStore Implementation
- **In memory storage with backing file storage** the memory is kept in memory with writting to disk when updates occur; all reads are memory reads
- **JSON-based file storage** for persistence across agent runs
- **Single file organization** one file containing all memory
- **Atomic writes** to prevent data corruption. write the whole file every time.
- **File locking** for concurrent access safety
- **Configurable file location** with default in agent's working directory

### Simplified Memory Tools

#### 1. Core Memory Tools (Only 3 tools)
- **`save_memory`**: Save memory entries to specified compartment
  - Parameters: `compartment` (run/session/plan), `content`, `category`, `tags`, `metadata`
- **`get_memory`**: Retrieve memory entries from specified compartment
  - Parameters: `compartment` (run/session/plan), `id` (optional), `category` (optional), `tags` (optional)
- **`clear_memory`**: Clear memory entries from specified compartment
  - Parameters: `compartment` (run/session/plan), `category` (optional), `tags` (optional)

## Functional Requirements

### Memory Compartment Operations
1. **Create**: Save new memory entries to specific compartments
2. **Read**: Retrieve memory entries by ID or search criteria from compartments
3. **Update**: Modify existing memory entries within compartments
4. **Delete**: Remove memory entries from specific compartments
5. **Search**: Find memory entries using text search, tags, or metadata within compartments
6. **List**: Enumerate memory entries with filtering and pagination by compartment
7. **Clear**: Clear all entries from a specific compartment

### Memory Compartment Behavior
1. **Run Memory**: Automatically included in every LLM call, cleared at run end
2. **Session Memory**: Available to all agents in session, persists across runs
3. **Plan Memory**: Configurable persistence, available to plan-related agents

### Memory Organization
1. **Categories**: Organize memory entries into logical categories within compartments
2. **Tags**: Add multiple tags to memory entries for flexible organization
3. **Metadata**: Store additional key-value metadata with memory entries
5. **Timestamps**: Track creation and modification times
6. **Access Tracking**: Monitor how often memory entries are accessed

### Memory Management
1. **Size Limits**: Configurable limits on memory storage size per compartment
2. **Error reporting**: Report error in llm tool call if memory size exceeded; inform the llm to delete entries 
3. **Context Manager**: Memory operations integrated into existing context managers

## Technical Requirements

### Performance
- Search operations should return results within 100ms for typical datasets
- File I/O should be minimised for read operations
- Default memory size per compartment is 10k characters

### Reliability
- Concurrent access should be handled safely
- Error handling should be comprehensive

### Usability
- Memory tools are automatic inserted into agent context
- Integration with existing agent workflows should be seamless

## Testing Requirements

### Unit Tests
- Test all memory compartment operations
- Test all memory tools individually
- Test memory entry validation and serialization
- Test error handling and edge cases
- Do not use testify; use simple go comparison 

### Integration Tests
- Test memory persistence across agent runs using Dummy Model
- Test memory sharing between agents and compartments
- Test concurrent memory access


## Guidelines
- Keep llm prompts clear and precise
- Keep tool definitions clear and with good description so that llms can understand when and how to use the tool
- No need to maintain backward compatilibity
- Create a README.md file in the package that describes the goals and the current design
- Keep comments to a minimal only commenting important concepts; no trivial comments
- Do not create new interfaces with complex signatures; go interfaces mus tbe simple with no more than one or two methods
- Only the run memory should be included in every LLM call; the other memories have to be explicitly requested by the LLM via tool calls

## Memory package
- I want to keep all memory files in their own package "memory" (i.e. memory folder)
- The memory type should return ai.Tool values so it depends only on the ai package and not on the aigentic AgentTool type. 
- The caller of memory package will need to convert the ai.Tool to an ai.AgentTool if required.
- Note that some the memory package cannot depend on the aigentic package because this will cause cyclic loops.

