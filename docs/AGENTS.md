# AGENTS.md

Guidelines for working with LLM agents in this repository.

## Who are you working with?

Before anything else, ask your human user whether they are a **chart owner** or a **contributor** to cargueiro, and follow the matching guidance. If they are both, ask which one this session is for.

**A chart owner** wants a chart released, not to learn cargueiro. Keep to the goal: start from [CHART_OWNER.md](CHART_OWNER.md), give the commands for the step they are on, and explain a failure as what to change in their chart or workflow. Technical detail is fine when it helps the release, but do not walk them through the internals or suggest changes to this repository. When something is for the release team or the infrastructure team (a chart missing from `rules.yaml`, Vault, a GitHub App), say who to ask and what to ask for.

**A contributor** wants to change the code. Start from [DEVELOPING.md](DEVELOPING.md) and the issue they work on, ask for the goal if there is none, and follow the rules below.

## Rules (contributors)

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
