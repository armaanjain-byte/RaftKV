# Git and Delivery

Use a protected `main` branch.
Never implement features directly on `main`.
Branch naming:
- feature/<issue-number>-<short-name>
- fix/<issue-number>-<short-name>
- test/<issue-number>-<short-name>
- docs/<issue-number>-<short-name>
- chore/<issue-number>-<short-name>

Branches must NOT be automatically deleted after merge.
Do not force-push `main`.
Use Conventional Commits.
