# Package Coupling Review

You are reviewing a Go codebase for package coupling problems using Robert Martin's package metrics. Skip generated code (protobuf, connect, etc.).

## Metrics Reference

| Metric | Formula | Meaning |
|--------|---------|---------|
| Ca | count of packages importing this one | afferent coupling (dependents) |
| Ce | count of packages this one imports | efferent coupling (dependencies) |
| A | exported interfaces / total types | abstractness (0=concrete, 1=abstract) |
| I | Ce / (Ce + Ca) | instability (0=stable, 1=unstable) |
| D | \|A + I - 1\| | distance from main sequence (0=ideal, 1=worst) |
| SCa | packages structurally satisfying this package's interfaces without importing it | implicit afferent coupling |
| SCe | packages whose interfaces this package's types satisfy without importing them | implicit efferent coupling |

The main sequence is the line A + I = 1. Packages near it have a healthy balance: stable packages are abstract (dependents program against interfaces), unstable packages are concrete (free to change since nothing depends on them). High D means a package violates this balance.

## Phase 1: Generate Metrics

Run:
```
goda metrics -types -sort d ./...
```

From the output, identify packages of concern using these filters:
- **D > 0.3 AND Ca > 2**: far from main sequence with real dependents — potential zone of pain or uselessness
- **Ca > 5 AND A < 0.2**: heavily depended-on concrete package — rigid, hard to change safely
- **Ce > 10**: importing many packages — possible SRP violation (ignore cmd/main packages)
- **SCa > 0 or SCe > 0**: implicit contracts exist via structural typing — note but don't flag as problems

Ignore packages where Ca = 0 regardless of D — no dependents means no coupling cost.

## Phase 2: Investigate Each Flagged Package

For each package of concern, determine:

1. **Role**: Read the source. Classify as: data definition (structs/constants/enums, no behavior), behavioral (methods with logic/I/O/side effects), or wrapper (thin adapter around external library).

2. **Dependents**: Grep for the import path. For each dependent, note what it actually uses — struct fields, function calls, type in signatures. Count distinct methods/functions called across all dependents.

3. **API surface**: List exported types, functions, and methods. Note whether dependents use the concrete type directly (e.g., `*Client` in constructor params) or could feasibly use an interface subset.

4. **Change likelihood**: Is this package wrapping something external that evolves? Does it contain business logic that changes with requirements? Or is it a stable data definition that rarely changes?

## Phase 3: Render Verdicts

For each investigated package, assign one verdict:

**Leave alone** — the metric is correct but the fix would be over-engineering. Typical for data definition packages: a struct with public fields and lookup functions doesn't benefit from interface extraction regardless of D or Ca. Also applies to any package where dependents only read fields, not call behavioral methods.

**Watch** — not actionable yet but the trend matters. Typical for packages at moderate D with growing Ca, or concrete packages where Ca recently increased. Note what would trigger escalation (e.g., "if Ca exceeds 5, extract interface X").

**Act** — real coupling problem with a concrete fix. Typical indicators:
- Dependents accept the concrete type as a constructor parameter (blocks testing)
- Dependents each use a different subset of a large method surface (interface segregation violation)
- Package wraps an external service and dependents can't be tested without it
- The standard Go fix: each dependent defines its own narrow interface for the 2-3 methods it calls

## Output Format

```
## Metrics Summary

[paste goda metrics output, omit generated packages]

## Packages of Concern

### `internal/example` — D=X.XX, Ca=N, A=X.XX

**Role:** [data definition | behavioral | wrapper]
**Dependents (N):** [list packages and what they use]
**API surface:** [N exported types, N methods]
**Change likelihood:** [low | medium | high] — [why]

**Verdict: [leave alone | watch | act]** — [one sentence justification]
[if act: concrete recommendation]

## Healthy Packages

[one paragraph summarizing packages on or near the main sequence]

## Structural Coupling Notes

[if any SCa/SCe > 0: which packages, what implicit contracts, any risk from signature changes]
```
