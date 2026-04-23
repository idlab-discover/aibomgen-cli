# Contributing to AIBoMGen CLI

Thanks for contributing.

## Before You Start

- Check existing issues and pull requests before starting work.
- For larger changes, open an issue first so the approach can be discussed before implementation.
- Keep changes focused. Avoid mixing refactors with feature work unless they are directly related.

## Development Setup

See the [Installation](https://github.com/idlab-discover/aibomgen-cli#from-source) section in the README for requirements and build steps.

## Project Structure

- `cmd/` contains Cobra command wiring
- `internal/` contains internal application logic and UI components
- `pkg/aibomgen/` contains importable public packages
- `targets/` contains sample targets used by tests and examples

## Contribution Guidelines

- Add or update tests for behavior changes when practical.
- Keep public APIs and CLI flags backwards compatible unless a breaking change is intentional and clearly documented.
- Update `README.md` when user-facing behavior changes.
- Follow existing naming, package layout, and error handling patterns in the repository.
- Prefer small pull requests that are easy to review.

## Pull Request Checklist

Before opening a pull request, make sure you have run:

```bash
go test ./...
go build ./...
golangci-lint run
```

The repository enforces [golangci-lint](https://golangci-lint.run/) in CI. All lint issues must be resolved before a pull request can be merged. To install it locally:

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
```

The active linter configuration is in [`.golangci.yml`](../.golangci.yml) at the repository root. Key rules in use include `errcheck`, `staticcheck`, `godot` (top-level comments must end with a period), `noctx`, `bodyclose`, and `unused`. Running the linter locally before pushing saves CI round-trips.

If your change affects docs, examples, or command output, update the relevant files in the same pull request.

## Commit Messages

Clear, descriptive commit messages are preferred. Conventional commits are welcome but not required.

## Code Review

Maintainers may request changes before merging. Reviews focus on correctness, maintainability, tests, compatibility, and documentation quality.
