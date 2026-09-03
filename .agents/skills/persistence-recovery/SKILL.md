---
name: persistence-recovery
description: Instructions for state persistence and crash recovery.
---

Require careful handling of:
* currentTerm
* votedFor
* log
* snapshots
* dedup state
* atomic persistence
* recovery
* crash timing
* snapshot replacement
* log/snapshot boundaries
