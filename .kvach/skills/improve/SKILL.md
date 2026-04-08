---
name: improve
description: "Architecture and design critique. Analyzes a component or proposal against Go best practices, SOLID principles, and kvach conventions. Produces a structured verdict: SHIP / IMPROVE / RETHINK / KILL."
---

# /improve — Architecture Critique

Critically evaluate a design, implementation, or proposal.

## USAGE

```
/improve <component or file path>
/improve <description of proposal>
```

## PHILOSOPHY

Never start from zero. Find what's already there, understand the constraints,
then critique with evidence.

## PROCEDURE

### STEP 1: Understand What's Being Evaluated

- If a file path: read the code.
- If a package name: read all files in the package.
- If a proposal: parse the description.

Identify: purpose, scope, dependencies, consumers.

### STEP 2: Research Context

- Check `docs/ai-coding-agent-plan.md` for architectural intent.
- Check `docs/implementation-plan-from-clubtasker.md` for design rationale.
- Check existing interfaces that the component must satisfy.
- Check tests for behavioral contracts.

### STEP 3: Structured Critique

Evaluate on 6 dimensions:

| Dimension              | Question                                              |
|------------------------|-------------------------------------------------------|
| Architecture alignment | Does it fit the package layout and dependency rules?   |
| Correctness            | Are there bugs, race conditions, or edge cases?        |
| Go idioms              | Does it follow Go conventions and stdlib patterns?     |
| Simplicity             | Is it simpler than it needs to be? More complex?       |
| Testability            | Can it be tested without mocking the world?            |
| Security               | Does it respect permission boundaries?                 |

### STEP 4: Improvement Proposals

For each issue found:

```
ISSUE:     <what's wrong>
REFERENCE: <Go standard/pattern/docs that defines the better way>
FIX:       <concrete code change>
IMPACT:    <what improves>
EFFORT:    <trivial / small / medium / large>
```

### STEP 5: Verdict + Score

Rate each dimension 1–10:

| Dimension              | Score |
|------------------------|-------|
| Architecture alignment | ?/10  |
| Correctness            | ?/10  |
| Go idioms              | ?/10  |
| Simplicity             | ?/10  |
| Testability            | ?/10  |
| Security               | ?/10  |
| **Average**            | ?/10  |

Verdict thresholds:
- 8.0+ : **SHIP IT** — good enough, minor polish only
- 6.0–7.9 : **IMPROVE IT** — solid foundation, fixable issues
- 4.0–5.9 : **RETHINK IT** — fundamental problems, needs redesign
- <4.0 : **KILL IT** — start over or drop the feature

### STEP 6: Apply (conditional)

- **SHIP IT**: No changes needed. Optionally note minor improvements.
- **IMPROVE IT**: List the fixes in priority order. Ask if user wants them applied.
- **RETHINK IT**: Write a redesign proposal with concrete interface sketches.
- **KILL IT**: Explain why and suggest an alternative approach.

## RULES

1. Be honest. The user benefits more from accurate criticism than validation.
2. Back every criticism with a concrete reference (Go spec, stdlib pattern, or project convention).
3. Do not critique test files — they test the current behavior, not ideal behavior.
4. If the code is good, say so. Not everything needs improvement.
