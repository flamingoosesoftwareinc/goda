# Adding Robert Martin Package Metrics to Goda

## Context

Goda (`github.com/loov/goda`) is a Go dependency analysis and visualization tool. The `golang.org/x/tools` repo is also available locally as a reference. The goal is to extend goda with Robert Martin's package quality metrics and a novel structural coupling analysis for Go's implicit interface satisfaction.

## What Was Built

### Robert Martin's Package Metrics

A new `goda metrics` subcommand and template fields on `goda list` computing five classic metrics per package:

- **Ca** (Afferent Couplings): number of packages in the analyzed set that depend on this package via imports.
- **Ce** (Efferent Couplings): number of packages this package directly imports.
- **A** (Abstractness): ratio of interface type declarations to total type declarations. Range 0..1. A=0 means fully concrete, A=1 means fully abstract.
- **I** (Instability): Ce / (Ce + Ca). Range 0..1. I=0 means fully stable, I=1 means fully unstable.
- **D** (Distance from Main Sequence): |A + I - 1|. Range 0..1. D=0 means the package sits on the ideal A+I=1 line.

Design decisions:
- Abstractness counts ALL interfaces (exported and unexported), not just exported.
- Ca/Ce coupling scope is all loaded packages, not just the query set.
- Both a dedicated `goda metrics` subcommand (with sort, format, header flags) and template fields accessible via `goda list -f`.
- Default sort is by D descending (worst packages first).

### Structural Coupling Metrics (SCa/SCe)

Go uses structural typing: a concrete type satisfies an interface without an `implements` keyword. This means real coupling can exist between packages with no import edge. Two new metrics measure this:

- **SCa** (Structural Afferent Coupling): number of packages whose concrete types satisfy this package's interfaces WITHOUT importing it.
- **SCe** (Structural Efferent Coupling): number of packages whose interfaces are satisfied by this package's concrete types WITHOUT this package importing them.

These are opt-in via `-types` flag because they require heavier loading (`NeedTypes | NeedDeps` in go/packages). The implementation:
- Enumerates all interfaces and concrete types per package via `pkg.Types.Scope()`
- Cross-checks all (concrete, interface) pairs across packages using `types.Implements()`
- Checks both value and pointer receivers
- Skips empty interfaces (every type satisfies `interface{}`)
- Only counts as structural coupling if NO import edge exists (otherwise it's already in Ca/Ce)
- Deduplicates by package (3 types in A satisfying 5 interfaces in B = SCa of 1, not 15)

Critical constraint: all packages must come from a single `packages.Load()` call for `types.Implements()` to work (type identity).

### Infrastructure Changes

- `stat.Decls` gained an `Interface` field. `DeclsFromAst` now iterates individual type specs (fixing undercounting of grouped `type()` blocks) and detects `*ast.InterfaceType`.
- `pkgset.Context` gained `TypesMode bool` to optionally add `NeedTypes | NeedDeps` to the load config.
- `pkgset.CalcWithOpts()` added alongside `Calc()` for backward compatibility.
- `pkggraph.Node` gained Ca, Ce, A, I, D, SCa, SCe fields. `ComputeMetrics()` and `ComputeStructuralCoupling()` methods added to `Graph`.

### Testing

Golden file tests in `internal/metrics/cmd_test.go` with a purpose-built testdata project at `internal/metrics/testdata/testproject/`:

| Package   | Ca | Ce | A    | I    | D    | SCa | SCe | Role                                          |
|-----------|----|----|------|------|------|-----|-----|-----------------------------------------------|
| base      | 3  | 0  | 0.67 | 0.00 | 0.33 | 1   | 0   | Stable+abstract (2 interfaces, no deps)       |
| types     | 1  | 1  | 0.00 | 0.50 | 0.50 | 0   | 0   | Concrete, mid-instability, far from main seq  |
| service   | 2  | 2  | 0.50 | 0.50 | 0.00 | 0   | 0   | On the main sequence                          |
| handler   | 1  | 2  | 0.00 | 0.67 | 0.33 | 0   | 0   | Concrete, somewhat unstable                   |
| app       | 0  | 2  | 0.00 | 1.00 | 0.00 | 0   | 0   | Leaf consumer, on main sequence               |
| compat    | 0  | 0  | 0.00 | 0.00 | 1.00 | 0   | 1   | Has Read/Write methods matching base's ifaces |

Four test cases: metrics sorted by D, metrics sorted by ID, list with metric template fields, and metrics with `-types`.

Tests use `-update` flag to regenerate golden files: `go test ./internal/metrics/ -update`

## Files

Modified:
- `internal/stat/decl.go` — Interface counting in Decls
- `internal/pkggraph/graph.go` — Metric fields, ComputeMetrics, ComputeStructuralCoupling
- `internal/pkgset/context.go` — TypesMode on Context
- `internal/pkgset/calc.go` — CalcWithOpts
- `internal/list/cmd.go` — -types flag, ComputeMetrics/ComputeStructuralCoupling calls
- `main.go` — Register metrics command, format help docs

Created:
- `internal/metrics/cmd.go` — The metrics subcommand
- `internal/metrics/cmd_test.go` — Golden file tests
- `internal/metrics/testdata/` — Test project and golden files

## Usage

```sh
# Import-based metrics (default sort by D descending)
goda metrics ./...

# With structural coupling analysis
goda metrics -types ./...

# Sort by afferent coupling
goda metrics -sort ca ./...

# Via list command templates
goda list -f '{{.ID}}  D={{printf "%.2f" .D}}' ./...

# Structural coupling in list
goda list -types -f '{{.ID}}  SCa={{.SCa}} SCe={{.SCe}}' ./...
```

## On Structural Coupling in Practice

Go's structural typing means `types.Implements()` can detect coupling invisible to the import graph. In practice, most Go interfaces use domain-specific types in their method signatures (e.g., `Write(*pkggraph.Graph) error`), which forces importers to reference the defining package anyway — converting structural coupling into import coupling. Structural coupling is most likely to appear with simple method signatures like `Read([]byte) (int, error)` that mirror stdlib conventions. The current implementation only checks packages within the analyzed set; stdlib interfaces like `io.Reader` or `fmt.Stringer` are not included in the analysis.
