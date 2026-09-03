# Go Engineering

* idiomatic modern Go
* explicit error handling
* context cancellation where appropriate
* no unnecessary dependencies
* deterministic tests
* `go vet`
* formatter
* race detector
* static analysis
* benchmarks where useful
* avoid global mutable state
* documented concurrency ownership
* clear lock boundaries
* avoid holding locks across blocking network/storage operations unless demonstrably safe
* avoid goroutine leaks
* graceful shutdown
* test cleanup
