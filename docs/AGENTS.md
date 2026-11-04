# AGENTS.md

Guidelines for working with LLM agents in this repository.

## Rules

1. **Human review is required.** Every new feature or modification must be reviewed by a human before it is considered done.
2. **Follow the [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md).** No `init()`, no mutable globals, wrap errors with `%w`, handle errors once, exit only in `main`, table-driven tests.
3. **Work in small steps.** Plan, review, implement one small chunk, review, test. Do not mix structural changes with new functionality.

## Commits

Use [Conventional Commits](https://www.conventionalcommits.org/): `<type>[scope]: <description>`.

| Situation | Co-author trailer |
|-----------|-------------------|
| Human reviewed the change and it meets the rules above | **No** `Co-Authored-By` |
| LLM-only commit, not reviewed by a human | **Required** `Co-Authored-By` |

LLM-only commits are allowed, but they must be co-authored so they stay identifiable.

## Before handing off

```
make test
make vet
make lint
```
