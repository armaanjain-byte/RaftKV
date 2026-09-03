---
name: raft-correctness
description: Instructions for modifying Raft protocol implementations safely.
---

1. identify the relevant Raft rule/invariant,
2. locate the implementation,
3. inspect existing tests,
4. make the smallest coherent change,
5. add/update targeted tests,
6. run focused tests,
7. run race detection,
8. run the broader regression suite,
9. document any important design decision.

Forbid changing protocol behavior merely because a test is inconvenient.
