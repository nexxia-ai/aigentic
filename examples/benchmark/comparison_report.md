# Model Comparison Report

Generated on: 2025-08-18 23:00:04

| Capability | qwen3:4b | gpt-4o-mini | gemini-2.0-flash-001 | qwen3:14b | qwen3:8b |
|---|---|---|---|---|---|
| ToolIntegration | ✅ Success | ✅ Success | ✅ Success | ✅ Success | ✅ Success |
| ToolIntegration (timing) | 2.6s | 1.6s | 2.4s | 9.7s | 2.6s |
| TeamCoordination | ✅ Success | ❌ Failure | ❌ Failure | ✅ Success | ❌ Failure |
| TeamCoordination (timing) | 26.1s | 58.1s | 3.7s | 75.5s | 44.5s |
| MultiAgentChain | ❌ Failure | ✅ Success | ✅ Success | ❌ Failure | ❌ Failure |
| MultiAgentChain (timing) | 106.3s | 4.9s | 6.5s | 254.4s | 42.8s |
| ConcurrentRuns | ✅ Success | ✅ Success | ✅ Success | ✅ Success | ✅ Success |
| ConcurrentRuns (timing) | 6.2s | 4.3s | 3.6s | 22.8s | 7.0s |
| StreamingWithTools | ✅ Success | ✅ Success | ✅ Success | ✅ Success | ✅ Success |
| StreamingWithTools (timing) | 3.9s | 2.5s | 3.0s | 12.7s | 2.9s |
| SimpleAgent | ✅ Success | ✅ Success | ✅ Success | ✅ Success | ✅ Success |
| SimpleAgent (timing) | 3.6s | 0.5s | 1.2s | 7.6s | 5.8s |
| FileAttachments | ❌ Failure | ✅ Success | ❌ Failure | ❌ Failure | ❌ Failure |
| FileAttachments (timing) | 5.2s | 1.6s | 1.0s | 9.4s | 6.1s |
| Streaming | ✅ Success | ✅ Success | ✅ Success | ✅ Success | ✅ Success |
| Streaming (timing) | 4.8s | 4.3s | 1.7s | 12.0s | 5.3s |
| MemoryPersistence | ✅ Success | ❌ Failure | ❌ Failure | ✅ Success | ✅ Success |
| MemoryPersistence (timing) | 22.7s | 46.6s | 7.6s | 80.7s | 29.1s |
