# Memory Package Design Specification

## Overview

The memory package provides a compartmentalized, persistent memory system for LLM agents working on complex multi-step tasks. It implements three distinct memory compartments (Run, Session, Plan) with comprehensive CRUD operations, file-based persistence, and seamless integration with the aigentic framework.

## Architecture

### Package Structure

```
aigentic/memory/
├── types.go          # Core types and interfaces
├── entry.go          # MemoryEntry operations and utilities
├── store.go          # FileStore implementation
├── tools.go          # Memory tool implementations
├── memory.go         # Main Memory type and methods
└── memory_test.go    # Comprehensive test suite
```

### Core Components

#### 1. Memory Compartments

**Run Memory**
- **Scope**: Single agent run (not shared with sub-agents)
- **Lifecycle**: Cleared automatically at the end of each agent run
- **Use Cases**: Current task state, progress tracking, intermediate results
- **Access**: Automatically included in every LLM call during the run

**Session Memory**
- **Scope**: Entire session (shared with sub-agents)
- **Lifecycle**: Persists across agent runs within the same session
- **Use Cases**: User preferences, session context, shared information
- **Access**: Must be requested using get_memory tool

**Plan Memory**
- **Scope**: Configurable (run-level or session-level)
- **Lifecycle**: Depends on plan scope configuration
- **Use Cases**: Multi-step plans, progress tracking, task dependencies
- **Access**: Must be requested using get_memory tool

#### 2. MemoryEntry Structure

```go
type MemoryEntry struct {
    ID          string            `json:"id"`           // Unique identifier
    Content     string            `json:"content"`      // Main content
    Category    string            `json:"category,omitempty"`    // Organization category
    Tags        []string          `json:"tags,omitempty"`        // Flexible tags
    Metadata    map[string]string `json:"metadata,omitempty"`     // Key-value metadata
    CreatedAt   time.Time         `json:"created_at"`   // Creation timestamp
    UpdatedAt   time.Time         `json:"updated_at"`   // Last modification timestamp
    AccessCount int               `json:"access_count"` // Usage tracking
    Priority    int               `json:"priority"`     // Priority level (1-10)
}
```

## Implementation Details

### FileStore Implementation

**Key Features:**
- **In-Memory Storage**: All reads are from memory for performance
- **File Persistence**: Writes to disk when updates occur
- **Atomic Writes**: Complete file rewrite on each update to prevent corruption
- **Concurrent Safety**: RWMutex for thread-safe operations
- **JSON Format**: Human-readable storage format

**Storage Strategy:**
1. Load data from file on initialization
2. Keep all data in memory for fast access
3. Write entire dataset to temporary file
4. Atomically rename temporary file to replace original
5. Handle file creation and directory creation automatically

**Error Handling:**
- Graceful handling of missing files (start with empty data)
- Corrupted file recovery (fallback to empty data)
- Directory creation with proper permissions
- Size limit enforcement with clear error messages

### Memory Tools

#### 1. save_memory Tool

**Purpose**: Save memory entries to specified compartments

**Parameters:**
- `compartment` (required): "run", "session", or "plan"
- `content` (required): The information to save
- `category` (optional): Organization category
- `tags` (optional): Array of tags for flexible organization
- `metadata` (optional): Key-value metadata object
- `priority` (optional): Priority level 1-10 (default: 5)

**Validation:**
- Compartment must be valid enum value
- Content cannot be empty
- Priority must be between 1-10
- Size limits enforced per compartment

#### 2. get_memory Tool

**Purpose**: Retrieve memory entries from specified compartments

**Parameters:**
- `compartment` (required): "run", "session", or "plan"
- `id` (optional): Specific entry ID
- `category` (optional): Category filter
- `tags` (optional): Tags filter

**Behavior:**
- If ID provided: return specific entry
- If filters provided: return matching entries
- If no filters: return all entries from compartment
- Increments access count for retrieved entries

#### 3. clear_memory Tool

**Purpose**: Clear memory entries from specified compartments

**Parameters:**
- `compartment` (required): "run", "session", or "plan"
- `category` (optional): Category filter
- `tags` (optional): Tags filter

**Behavior:**
- If no filters: clear all entries from compartment
- If filters provided: clear only matching entries
- Irreversible operation with confirmation message

### Integration with Aigentic Framework

#### Agent Integration

**Memory Field**: Agents have a `Memory *memory.Memory` field
**Tool Registration**: Memory tools automatically added to agent tool list
**Context Integration**: Memory content included in LLM prompts

#### Context Manager Integration

**System Prompt**: Memory system description and tool usage guidance
**User Prompt**: Only Run memory content formatted and included in context
**Template Variables**: `HasMemory`, `MemoryContent` for template rendering
**Access Control**: Session and Plan memory require explicit tool requests

#### Lifecycle Management

**Run Start**: Memory tools registered, run memory available
**Run End**: Run memory automatically cleared
**Session Persistence**: Session and plan memory persist across runs

## Configuration

### MemoryConfig Structure

```go
type MemoryConfig struct {
    MaxSizePerCompartment int    // Character limit per compartment (default: 10000)
    StoragePath           string // File path for persistence (default: "memory.json")
}
```

### Default Configuration

- **Max Size**: 10,000 characters per compartment
- **Storage Path**: "memory.json" in agent working directory
- **File Permissions**: 0755 for directories, 0644 for files

## Performance Characteristics

### Read Operations
- **Memory Access**: O(1) for direct ID lookup
- **Search Operations**: O(n) where n is number of entries
- **Filtering**: In-memory filtering for fast response
- **Target Performance**: <100ms for typical datasets

### Write Operations
- **Memory Update**: O(1) for direct updates
- **File Write**: Complete file rewrite (atomic operation)
- **Concurrent Access**: RWMutex prevents data corruption
- **Size Validation**: O(1) size check before write

### Storage Efficiency
- **Memory Usage**: All data kept in memory for fast access
- **Disk Usage**: JSON format with minimal overhead
- **Compression**: Not implemented (can be added if needed)

## Error Handling

### Validation Errors
- **Invalid Compartment**: Clear error message with valid options
- **Empty Content**: Explicit error for required content
- **Size Limit Exceeded**: Informative error with current size and limit
- **Invalid Priority**: Range validation with clear bounds

### Storage Errors
- **File Access**: Graceful handling of permission issues
- **Disk Space**: Error propagation with context
- **Corrupted Data**: Fallback to empty data with logging
- **Concurrent Access**: Proper locking prevents corruption

### Tool Execution Errors
- **Parameter Validation**: Clear error messages for invalid inputs
- **Storage Failures**: Detailed error information for debugging
- **Size Limits**: Informative messages about memory constraints

## Testing Strategy

### Unit Tests
- **MemoryEntry**: Validation, utility functions, size calculation
- **FileStore**: CRUD operations, persistence, concurrent access
- **Memory Tools**: Parameter validation, error handling
- **Filtering**: Category and tag-based filtering
- **Search**: Text search functionality

### Integration Tests
- **Agent Integration**: Memory tools with dummy model
- **Lifecycle Management**: Run memory clearing verification
- **Persistence**: Cross-session memory persistence
- **Error Handling**: Invalid inputs and edge cases
- **Size Limits**: Memory constraint enforcement

### Test Coverage
- **Happy Path**: All normal operations work correctly
- **Error Cases**: Invalid inputs handled gracefully
- **Edge Cases**: Empty data, corrupted files, concurrent access
- **Performance**: Size limits and concurrent operations

## Security Considerations

### Data Protection
- **File Permissions**: Appropriate permissions for memory files
- **Directory Creation**: Safe directory creation with proper permissions
- **Input Validation**: All inputs validated before processing
- **Size Limits**: Prevent memory exhaustion attacks

### Access Control
- **Compartment Isolation**: Clear boundaries between memory types
- **Session Scope**: Proper session-based access control
- **Tool Validation**: All tool parameters validated

## Future Enhancements

### Potential Improvements
1. **Compression**: Add optional compression for large datasets
2. **Encryption**: Add encryption for sensitive memory data
3. **Backup**: Automatic backup of memory files
4. **Analytics**: Usage tracking and memory effectiveness metrics
5. **Sharing**: Memory sharing between different sessions
6. **Indexing**: Advanced search capabilities with indexing

### Extension Points
- **Custom Storage**: Implement different storage backends
- **Custom Tools**: Add specialized memory tools
- **Custom Compartments**: Add new memory compartment types
- **Custom Validation**: Extend validation rules

## Maintenance Guidelines

### Code Organization
- **Single Responsibility**: Each file has a clear purpose
- **Simple Interfaces**: Interfaces with minimal methods
- **Clear Naming**: Descriptive names for all components
- **Minimal Comments**: Only essential concepts documented

### Error Handling
- **Fail Fast**: Validate inputs early
- **Clear Messages**: Informative error messages
- **Graceful Degradation**: Fallback behavior for errors
- **Logging**: Appropriate logging for debugging

### Performance Monitoring
- **Size Tracking**: Monitor memory usage per compartment
- **Access Patterns**: Track which memory entries are accessed
- **Performance Metrics**: Monitor operation timing
- **Resource Usage**: Track file I/O and memory consumption

## Dependencies

### Internal Dependencies
- `github.com/google/uuid` - For generating unique IDs
- `time` - For timestamp management
- `sync` - For concurrent access safety
- `encoding/json` - For data serialization
- `os` - For file operations
- `path/filepath` - For path manipulation

### External Dependencies
- `github.com/nexxia-ai/aigentic/ai` - For tool result types

### Circular Dependency Prevention
- Memory package is self-contained and doesn't import aigentic package
- Memory tools return `ai.Tool` types (not `AgentTool`) to avoid dependencies
- Agent package imports memory package and converts tools as needed
- Clean interface boundaries between packages prevent circular imports

## Version History

### v1.1.0 (Current)
- Moved Memory type to memory package for better organization
- Eliminated circular dependencies between packages
- Memory tools return ai.Tool types for clean separation
- Integration tests remain in aigentic package to avoid circular imports
- All memory functionality self-contained in memory package

### v1.0.0 (Previous)
- Initial implementation of compartmentalized memory system
- Three memory compartments (Run, Session, Plan)
- File-based persistence with atomic writes
- Three core memory tools (save_memory, get_memory, clear_memory)
- Comprehensive test suite
- Integration with aigentic framework

---

*This document serves as the authoritative design specification for the memory package. It should be updated whenever significant changes are made to the implementation.*
