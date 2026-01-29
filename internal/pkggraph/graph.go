package pkggraph

import (
	"encoding/json"
	"go/types"
	"math"
	"sort"

	"golang.org/x/tools/go/packages"

	"github.com/flamingoosesoftwareinc/goda/internal/stat"
)

type Graph struct {
	Packages map[string]*Node
	Sorted   []*Node
	stat.Stat
}

func (g *Graph) AddNode(n *Node) {
	g.Packages[n.ID] = n
	n.Graph = g
}

type Node struct {
	*packages.Package
	Color string

	ImportsNodes []*Node

	// Stats about the current node.
	stat.Stat
	// Stats about upstream nodes.
	Up stat.Stat
	// Stats about downstream nodes.
	Down stat.Stat

	// Robert Martin package metrics.
	Ca float64 // Afferent couplings: number of packages that depend on this package.
	Ce float64 // Efferent couplings: number of packages this package depends on.
	A  float64 // Abstractness: ratio of interfaces to total types.
	I  float64 // Instability: Ce / (Ce + Ca).
	D  float64 // Distance from the main sequence: |A + I - 1|.

	// Structural coupling metrics (requires type analysis via -types flag).
	SCa float64 // Structural afferent coupling: packages with types satisfying this package's interfaces, excluding importers.
	SCe float64 // Structural efferent coupling: packages with interfaces satisfied by this package's types, excluding imports.

	Errors []error
	Graph  *Graph
}

func (n *Node) Pkg() *packages.Package { return n.Package }

// From creates a new graph from a map of packages.
func From(pkgs map[string]*packages.Package) *Graph {
	g := &Graph{Packages: map[string]*Node{}}

	// Create the graph nodes.
	for _, p := range pkgs {
		n := LoadNode(p)
		g.Sorted = append(g.Sorted, n)
		g.AddNode(n)
		g.Stat.Add(n.Stat)
	}
	SortNodes(g.Sorted)

	// TODO: find ways to improve performance.

	cache := allImportsCache(pkgs)

	// Populate the graph's Up and Down stats.
	for _, n := range g.Packages {
		importsIDs := cache[n.ID]
		for _, id := range importsIDs {
			imported, ok := g.Packages[id]
			if !ok {
				// we may not want to print info about every package
				continue
			}

			n.Down.Add(imported.Stat)
			imported.Up.Add(n.Stat)
		}
	}

	// Build node imports from package imports.
	for _, n := range g.Packages {
		for id := range n.Package.Imports {
			direct, ok := g.Packages[id]
			if !ok {
				// TODO:
				//  should we include dependencies where Y is hidden?
				//  X -> [Y] -> Z
				continue
			}

			n.ImportsNodes = append(n.ImportsNodes, direct)
		}
	}

	for _, n := range g.Packages {
		SortNodes(n.ImportsNodes)
	}

	return g
}

// ComputeMetrics calculates Robert Martin's package metrics (Ca, Ce, A, I, D)
// for every node in the graph. allPkgs is the full set of loaded packages used
// to determine afferent couplings from packages that may be outside the graph.
func (g *Graph) ComputeMetrics(allPkgs map[string]*packages.Package) {
	// Compute Ca: for each package in the graph, count how many packages
	// across allPkgs import it.
	for _, n := range g.Packages {
		var ca int
		for _, p := range allPkgs {
			if p.ID == n.ID {
				continue
			}
			if _, imports := p.Imports[n.PkgPath]; imports {
				ca++
			}
		}
		n.Ca = float64(ca)
	}

	// Compute Ce: number of direct imports for each node.
	for _, n := range g.Packages {
		n.Ce = float64(len(n.Package.Imports))
	}

	// Compute A, I, D.
	for _, n := range g.Packages {
		totalTypes := n.Stat.Decls.Type
		interfaces := n.Stat.Decls.Interface
		if totalTypes > 0 {
			n.A = float64(interfaces) / float64(totalTypes)
		}

		total := n.Ca + n.Ce
		if total > 0 {
			n.I = n.Ce / total
		}

		n.D = math.Abs(n.A + n.I - 1)
	}
}

// ComputeStructuralCoupling calculates SCa and SCe for each node.
// It finds concrete types that satisfy interfaces in other packages
// without importing them. Requires packages loaded with NeedTypes.
func (g *Graph) ComputeStructuralCoupling() {
	type pkgIfaces struct {
		id    string
		ifaces []*types.Interface
	}
	type pkgConcretes struct {
		id        string
		concretes []types.Type // named types that are not interfaces
	}

	var allIfaces []pkgIfaces
	var allConcretes []pkgConcretes

	for _, n := range g.Sorted {
		if n.Package.Types == nil {
			continue
		}
		scope := n.Package.Types.Scope()
		var ifaces []*types.Interface
		var concretes []types.Type

		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			tn, ok := obj.(*types.TypeName)
			if !ok {
				continue
			}
			if iface, ok := tn.Type().Underlying().(*types.Interface); ok {
				if iface.NumMethods() > 0 {
					ifaces = append(ifaces, iface)
				}
			} else {
				concretes = append(concretes, tn.Type())
			}
		}

		if len(ifaces) > 0 {
			allIfaces = append(allIfaces, pkgIfaces{id: n.ID, ifaces: ifaces})
		}
		if len(concretes) > 0 {
			allConcretes = append(allConcretes, pkgConcretes{id: n.ID, concretes: concretes})
		}
	}

	// For each pair (concrete pkg A, interface pkg B) where A != B,
	// check if any concrete in A implements any interface in B.
	// Only count if A does not import B (for SCe on A)
	// and B does not import A (for SCa on B).
	for _, cp := range allConcretes {
		cn := g.Packages[cp.id]
		if cn == nil {
			continue
		}
		for _, ip := range allIfaces {
			if cp.id == ip.id {
				continue
			}
			in := g.Packages[ip.id]
			if in == nil {
				continue
			}

			// Check if any concrete type in cp satisfies any interface in ip.
			satisfied := false
			for _, ct := range cp.concretes {
				for _, iface := range ip.ifaces {
					if types.Implements(ct, iface) || types.Implements(types.NewPointer(ct), iface) {
						satisfied = true
						break
					}
				}
				if satisfied {
					break
				}
			}
			if !satisfied {
				continue
			}

			// cp's types satisfy ip's interfaces.
			// SCe for cp: only if cp does NOT import ip (otherwise it's already in Ce).
			if _, imports := cn.Package.Imports[in.PkgPath]; !imports {
				cn.SCe++
			}
			// SCa for ip: only if ip does NOT import cp (otherwise it's already in Ca).
			if _, imports := in.Package.Imports[cn.PkgPath]; !imports {
				in.SCa++
			}
		}
	}
}

func LoadNode(p *packages.Package) *Node {
	node := &Node{}
	node.Package = p

	stat, errs := stat.Package(p)
	node.Errors = append(node.Errors, errs...)
	node.Stat = stat

	return node
}

func SortNodes(xs []*Node) {
	sort.Slice(xs, func(i, k int) bool { return xs[i].ID < xs[k].ID })
}

type flatNode struct {
	Package struct {
		ID              string
		Name            string            `json:",omitempty"`
		PkgPath         string            `json:",omitempty"`
		Errors          []packages.Error  `json:",omitempty"`
		GoFiles         []string          `json:",omitempty"`
		CompiledGoFiles []string          `json:",omitempty"`
		OtherFiles      []string          `json:",omitempty"`
		IgnoredFiles    []string          `json:",omitempty"`
		ExportFile      string            `json:",omitempty"`
		Imports         map[string]string `json:",omitempty"`
	}

	ImportsNodes []string `json:",omitempty"`

	Stat stat.Stat
	Up   stat.Stat
	Down stat.Stat

	Errors []error `json:",omitempty"`
}

func (p *Node) MarshalJSON() ([]byte, error) {
	flat := flatNode{
		Stat:   p.Stat,
		Up:     p.Up,
		Down:   p.Down,
		Errors: p.Errors,
	}

	flat.Package.ID = p.Package.ID
	flat.Package.Name = p.Package.Name
	flat.Package.PkgPath = p.Package.PkgPath
	flat.Package.GoFiles = p.Package.GoFiles
	flat.Package.CompiledGoFiles = p.Package.CompiledGoFiles
	flat.Package.OtherFiles = p.Package.OtherFiles
	flat.Package.IgnoredFiles = p.Package.IgnoredFiles
	flat.Package.ExportFile = p.Package.ExportFile

	for _, n := range p.ImportsNodes {
		flat.ImportsNodes = append(flat.ImportsNodes, n.ID)
	}
	if len(p.Package.Imports) > 0 {
		flat.Package.Imports = make(map[string]string, len(p.Imports))
		for path, ipkg := range p.Imports {
			flat.Package.Imports[path] = ipkg.ID
		}
	}

	return json.Marshal(flat)
}
