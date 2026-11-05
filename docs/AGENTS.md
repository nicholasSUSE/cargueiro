# AGENTS.md

Guidelines for working with LLM agents in this repository.

## Rules

1. **No AI-written code without human review.** Every new feature or modification must be reviewed by a human before it is committed.
2. **Follow the [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md).** No `init()`, no mutable globals, table-driven tests. Errors follow [Error handling in DEVELOPING.md](DEVELOPING.md#error-handling).
3. **Work in small steps.** Plan, review, implement one small chunk, review, test. Do not mix structural changes with new functionality.

## Commits

Use [Conventional Commits](https://www.conventionalcommits.org/): `<type>[scope]: <description>`.

A `Co-Authored-By` trailer is not allowed, and a commit that has one will be blocked. A change that would need one has not been reviewed by a human, so it is not accepted.

## Before handing off

```
make test
make vet
make lint
```
