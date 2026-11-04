# Contributing

Development is milestone-gated according to `docs/planning/Firedoor_Master_Build_Plan_v1.1.pdf`.

1. Branch from clean `main` using `feat/vNN-topic`.
2. Prefer one verifiable vertical slice per pull request.
3. Keep Terraform state and auth data out of commits; never mutate arbitrary Kubernetes contexts.
4. Run `gofmt`, `go test ./...`, `go vet ./...`, and the active Terraform checks before PR review.
5. Update `docs/progress.md` with the actual passing commands, observed output, commit, caveats, and next task.
6. Merge only after tests and CI pass; verify `main` is clean.

Do not claim a scenario is behaviourally proven based solely on the presence of policies, status text, or mock data.
