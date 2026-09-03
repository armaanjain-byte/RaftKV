# Distributed Systems Rules

* Compare implementation behavior against the project's Raft protocol specification.
* Treat persistent state transitions as correctness-critical.
* Never reason about concurrency only from source-code appearance; reason about interleavings and failure timing.
* Every safety-sensitive behavior needs a targeted test.
* Every discovered correctness bug should produce a regression test.
* Never fix only the observed symptom if the underlying invariant remains violated.
* Clearly distinguish safety from liveness.
* Never label a test as proving a stronger property than it actually establishes.
