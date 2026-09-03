# Fault-Tolerant Distributed KV Store

## 1. Project Overview
This project implements a Fault-Tolerant Distributed Key-Value (KV) Store from scratch in Go. It relies on the Raft consensus algorithm for distributed coordination and state replication.

## 2. Rationale
The primary purpose of this repository is to serve as a serious, research-grade implementation of a distributed system. It emphasizes rigorous software engineering practices, deterministic testing, and invariant-checking over superficial feature expansion.

## 3. Architecture
The system follows a layered architecture:
```text
Client -> KV Service -> Raft -> State Machine -> Persistent State
```
The Raft module is cleanly separated from the network transport, which allows it to interface with either a real TCP transport layer or a deterministic simulated transport layer for testing and fault injection.

## 4. Raft Protocol Scope
This project implements the core Raft consensus algorithm, including leader election, log replication, and snapshotting mechanisms. 

## 5. Safety Invariants
The system enforces and continuously tests the following critical safety invariants:
- **Election Safety:** At most one leader can be elected in a given term.
- **Leader Append-Only:** A leader never overwrites or deletes entries in its own log.
- **Log Matching:** If two logs contain an entry with the same index and term, the logs are identical in all entries up through that index.
- **Leader Completeness:** If a log entry is committed in a given term, it will be present in the logs of the leaders for all higher-numbered terms.
- **State Machine Safety:** If a server has applied a log entry at a given index to its state machine, no other server will ever apply a different log entry for the same index.

## 6. Client Semantics
The KV store provides strict client-session *at-most-once* semantics to prevent duplicated command execution. Reads are served via the ReadIndex mechanism, ensuring strongly consistent (linearizable) reads without requiring new log entries.

## 7. Durability Model
State transitions involving `currentTerm`, `votedFor`, and the logical `log` are persisted atomically. Snapshots and deduplication state are managed with strict durability boundaries to guarantee safe crash recovery.

## 8. Fault Model
**Included in tests:**
- Process crash and restart
- Message loss, delay, duplication, and reordering
- Symmetric and asymmetric network partitions

**Excluded (Out of scope):**
- Power-loss simulation
- Disk corruption
- Hardware failure

## 9. Testing Strategy
The project follows a staged testing approach:
`Unit Tests -> Deterministic Integration Tests -> Adversarial/Failure Tests -> Race Detector -> Regression Suite`

## 10. Reproducible Failure Testing
Distributed testing relies on a deterministic simulated transport and seeded scheduling. All randomness is controlled by seeds. A failing seed from CI can be used to locally reproduce the exact sequence of interleavings and failures.

## 11. Benchmarks
(Performance benchmarks for single-node and replicated throughput will be recorded here once the core implementation achieves correctness and stability.)

## 12. Limitations
The system assumes static cluster membership. Dynamic membership changes are not supported.

## 13. Out-of-Scope Functionality
To maintain engineering rigor on the core consensus logic, the following are explicitly out of scope:
- Dynamic membership
- Sharding / partitioning of the KV space
- Multi-datacenter awareness
- Production-grade LSM/B-tree storage engine optimizations

## 14. Development Workflow
The repository enforces a strict, protected workflow:
`Issue -> Feature Branch -> Implementation -> Tests -> Pull Request -> CI Gate -> Auto-Merge`

## 15. Reproducing Seeded Failures
To reproduce a failure discovered by the fault injector in CI, execute the test locally using the provided seed:
```bash
go test -run <TestName> -args -seed <SeedValue>
```

## 16. Continuous Integration (CI)
CI gates all pull requests. The pipeline requires all formatting (`go fmt`), static analysis (`go vet`), deterministic tests, and race detection (`go test -race`) to pass cleanly before any code can be merged. CI enforces the project invariants on every commit.
