# Project Rules

* The frozen PRD is authoritative.
* Never silently change requirements.
* Never weaken a guarantee merely to make a test pass.
* Never delete a failing test to make CI green.
* Never suppress race detector failures.
* Never hide flaky tests.
* Never bypass CI.
* Never directly push feature work to `main`.
* Every feature or meaningful bug fix gets its own branch.
* Every feature or meaningful bug fix gets a GitHub issue.
* Every feature or meaningful bug fix gets a pull request.
* PRs are merged only after required automated checks pass.
* Feature branches must remain after merge; do not delete them.
* Keep the repository reproducible from a clean checkout.
* Prefer explicit, understandable implementation over clever abstractions.
* Do not add out-of-scope features.
