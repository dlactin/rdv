# AI Agent Repository Standards

This file provides foundational mandates for any AI agent interacting with the `rdv` repository.

## Engineering Standards

### Validation & Quality
- **Mandatory Commands:** ALWAYS run the following commands in order after any code modification:
  1. `make fmt` - To ensure consistent formatting.
  2. `make lint` - To check for code quality issues.
  3. `make test` - To ensure no regressions.
  4. `make docs` - To ensure documentation is up to date.
  5. `make` - To verify the project still builds.
- **Testing:** 
  - New functionality MUST be covered by new test cases in the relevant package's `_test.go` file.
  - The task is not considered complete until all tests pass.
- **Dependencies:** If `go.mod` or `go.sum` are affected, run `go mod tidy`.

### Git Workflow
- **Commit Policy:** AI Agents MUST NOT commit changes. All modifications must be vetted and committed by a human developer.
- **Signing:** Commits MUST be signed using the configured SSH/GPG key. Use `git commit -S` or `git cherry-pick -S`.
- **Branching:** Work should be performed on feature branches and rebased onto `main` cleanly.

### Documentation & References
- **Helm/Kustomize:** If changes affect how Helm charts or Kustomize manifests are processed, ensure that any relevant examples in `examples/` or documentation in `docs/` are updated.
- **README:** If flags or features are added, ensure `README.md` (and `README.md.tmpl` if applicable) is updated.
