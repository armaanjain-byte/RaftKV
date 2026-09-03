# Testing Strategy

Every feature follows:
implementation -> focused unit tests -> deterministic integration tests -> adversarial/failure tests -> race detector -> regression suite -> CI

A bug is not considered fixed until a regression test exists.
Tests must identify what invariant or behavior they exercise.
Distributed tests must be deterministic whenever possible.
Randomized testing must always support reproducible seeds.
Never introduce uncontrolled randomness into correctness tests.
