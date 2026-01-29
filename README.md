# Goda

Goda is a Go dependency analysis toolkit. It contains tools to figure out what your program is using.

_Note: the exact syntax of the command line arguments has not yet been finalized. So expect some changes to it._

_This is a fork of [github.com/loov/goda](https://github.com/loov/goda) with additional package quality metrics._

### Installation

To install, you'll need a recent version of Go and then you can install via:

```
go install github.com/flamingoosesoftwareinc/goda@latest
```

The commands assume that your GOBIN is reachable on path.

The graph visualizations require [GraphViz](https://graphviz.org/) for rendering the graph.

## Cool things it can do

```
# All of the commands should be run in the cloned repository.
git clone https://github.com/flamingoosesoftwareinc/goda && cd goda

# draw a graph of packages in github.com/flamingoosesoftwareinc/goda
goda graph "github.com/flamingoosesoftwareinc/goda/..." | dot -Tsvg -o graph.svg

# draw a graph of command github.com/flamingoosesoftwareinc/goda, within the module
goda graph "github.com/flamingoosesoftwareinc/goda:mod" | dot -Tsvg -o graph.svg

# draw a dependency graph of github.com/flamingoosesoftwareinc/goda and dependencies
goda graph -cluster -short "github.com/flamingoosesoftwareinc/goda:all" | dot -Tsvg -o graph.svg

# list direct dependencies of github.com/flamingoosesoftwareinc/goda
goda list "github.com/flamingoosesoftwareinc/goda/...:import"

# list dependency graph that reaches flag package, including std
goda graph -std "reach(github.com/flamingoosesoftwareinc/goda/...:all, flag)" | dot -Tsvg -o graph.svg

# list packages shared by two subpackages
goda list "shared(github.com/flamingoosesoftwareinc/goda/internal/pkgset:all, github.com/flamingoosesoftwareinc/goda/internal/cut:all)"

# list packages that are only imported for tests
goda list "github.com/flamingoosesoftwareinc/goda/...:+test:all - github.com/flamingoosesoftwareinc/goda/...:all"

# list packages that are imported with `purego` tag
goda list -std "purego=1(github.com/flamingoosesoftwareinc/goda/...:all)"

# list packages that are imported for windows and not linux
goda list "goos=windows(github.com/flamingoosesoftwareinc/goda/...:all) - goos=linux(github.com/flamingoosesoftwareinc/goda/...:all)"

# list how much memory each symbol in the final binary is taking
goda weight -h $GOPATH/bin/goda

# show the impact of cutting a package
goda cut ./...:all

# print dependency tree of all sub-packages
goda tree ./...:all

# print stats while building a go program
go build -a --toolexec "goda exec" .

# list dependency graph in same format as "go mod graph"
goda graph -type edges -f '{{.ID}}{{if .Module}}{{with .Module.Version}}@{{.}}{{end}}{{end}}' ./...:all
```

Maybe you noticed that it's using some weird symbols on the command-line while specifying packages. They allow for more complex scenarios.

The basic syntax is that you can specify multiple packages:

```
goda list github.com/flamingoosesoftwareinc/goda/... github.com/loov/qloc
```

By default it will select all the specific packages. You can select the package's direct dependencies with `:import`, direct and indirect dependencies with `:import:all`, the package and all of its direct and indirect dependencies with `:all`:

```
goda list github.com/flamingoosesoftwareinc/goda/...:import
goda list github.com/flamingoosesoftwareinc/goda/...:import:all
goda list github.com/flamingoosesoftwareinc/goda/...:all
```

You can also do basic arithmetic with these sets. For example, if you wish to ignore all `golang.org/x/tools` dependencies:

```
goda list github.com/flamingoosesoftwareinc/goda/...:all - golang.org/x/tools/...
```

To get more help about expressions or formatting:

```
goda help expr
goda help format
```

## Package Metrics

The `goda metrics` command computes Robert Martin's package quality metrics for each package in the analyzed set:

| Metric | Name | Description |
|--------|------|-------------|
| **Ca** | Afferent Coupling | Number of packages that depend on this package via imports |
| **Ce** | Efferent Coupling | Number of packages this package directly imports |
| **A** | Abstractness | Ratio of exported interface declarations to total type declarations (0 = fully concrete, 1 = fully abstract) |
| **I** | Instability | Ce / (Ce + Ca). 0 = fully stable, 1 = fully unstable |
| **D** | Distance from Main Sequence | \|A + I - 1\|. 0 = ideal balance, 1 = worst position |

```
# show metrics for all packages, sorted by distance (worst first)
goda metrics ./...

# sort by afferent coupling (most depended-on first)
goda metrics -sort ca ./...

# sort by package name
goda metrics -sort id ./...

# access metrics via list command templates
goda list -f '{{.ID}}  D={{printf "%.2f" .D}}  Ca={{.Ca}}' ./...
```

### Structural Coupling (SCa/SCe)

Go uses structural typing — a concrete type satisfies an interface without an `implements` keyword. This means real coupling can exist between packages with no import edge. Two additional metrics measure this implicit coupling:

| Metric | Name | Description |
|--------|------|-------------|
| **SCa** | Structural Afferent Coupling | Packages whose concrete types satisfy this package's interfaces WITHOUT importing it |
| **SCe** | Structural Efferent Coupling | Packages whose interfaces are satisfied by this package's concrete types WITHOUT this package importing them |

These require heavier type analysis and are opt-in via the `-types` flag:

```
# show all metrics including structural coupling
goda metrics -types ./...

# structural coupling via list templates
goda list -types -f '{{.ID}}  SCa={{.SCa}} SCe={{.SCe}}' ./...
```

### Using Metrics in Code Review

The metrics are most useful as a before/after comparison on a PR branch. Here's what to look for:

**D increased** — the change moved a package away from the main sequence. A concrete type was added to a stable package (zone of pain) or an abstract type to an unstable leaf (zone of uselessness). Ask whether an interface should be extracted or whether the type belongs in a different package.

**Ca went up significantly** — more packages now depend on this one. It's becoming a hub. If A is low (mostly concrete), this package is becoming rigid and hard to change. Consider whether dependents should program against interfaces instead.

**Ce is climbing** — the package is accumulating knowledge of the system. Fine for `main` or `cmd` packages, a red flag for `util` or `service` packages. Ask whether the package is doing too much.

**SCa > 0** — some package is silently satisfying your interfaces via structural typing. This is common for simple signatures like `Read([]byte) (int, error)` that mirror stdlib conventions. Worth knowing about, since a method signature change could silently break an implicit contract with no compiler error in the interface-defining package.

**Practical thresholds:**
- For core packages (Ca > 5), flag any PR that pushes D above 0.3
- For leaf packages (Ca = 0), D is mostly noise — the package has no dependents to hurt
- SCa/SCe are informational — they tell you where implicit contracts exist, not that something is wrong

```
# quick check: what are the most problematic packages?
goda metrics -sort d ./...

# what are the most depended-on packages? (review changes here carefully)
goda metrics -sort ca ./...

# where does implicit coupling exist?
goda metrics -types -sort sca ./...
```

## Graph example

Here's an example output for:

```
git clone https://github.com/flamingoosesoftwareinc/goda && cd goda
goda graph github.com/flamingoosesoftwareinc/goda:mod | dot -Tsvg -o graph.svg
```

![github.com/flamingoosesoftwareinc/goda dependency graph](./graph.svg)

## How it differs from `go list` or `go mod`

`go list` and `go mod` are tightly integrated with Go and can answer simple queries with compatibility. They also serves as good building blocks for other tools.

`goda` is intended for more complicated queries and analysis. Some of the features can be reproduced by format flags and scripts. However, this library aims to make even complicated analysis fast.

Also, `goda` can be used together with `go list` and `go mod`.
