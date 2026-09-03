# Fault-Tolerant Distributed KV Store

1. What this project is: A distributed KV store using Raft.
2. Why it exists: Educational/research-grade system.
3. Architecture: Client -> KV -> Raft -> State Machine.
4. Raft protocol scope: Full consensus with snapshots.
5. Safety invariants: Election safety, Log matching, etc.
6. Client semantics: At-most-once.
7. Durability model: Atomic persistence.
8. Fault model: Network faults, crash/restart.
9. Testing strategy: Deterministic simulation.
10. Reproducible failure testing: Seed based.
11. Benchmarks: TBD.
12. Limitations: No dynamic membership.
13. Out-of-scope functionality: Sharding.
14. Development workflow: Strict PR/Issue model.
15. How to reproduce a seeded failure: Use `go test -run TestX -args -seed Y`.
16. How CI works: Gates all PRs.
