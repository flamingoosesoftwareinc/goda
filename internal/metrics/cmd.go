package metrics

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/google/subcommands"

	"github.com/flamingoosesoftwareinc/goda/internal/pkggraph"
	"github.com/flamingoosesoftwareinc/goda/internal/pkgset"
	"github.com/flamingoosesoftwareinc/goda/internal/templates"
)

var (
	defaultHeader = "ID\tCa\tCe\tA\tI\tD"
	defaultFormat = "{{.ID}}\t{{.Ca}}\t{{.Ce}}\t{{printf \"%.2f\" .A}}\t{{printf \"%.2f\" .I}}\t{{printf \"%.2f\" .D}}"

	typesHeader = "ID\tCa\tCe\tA\tI\tD\tSCa\tSCe"
	typesFormat = "{{.ID}}\t{{.Ca}}\t{{.Ce}}\t{{printf \"%.2f\" .A}}\t{{printf \"%.2f\" .I}}\t{{printf \"%.2f\" .D}}\t{{.SCa}}\t{{.SCe}}"
)

type Command struct {
	printStandard bool
	typesMode     bool

	noAlign bool
	header  string
	format  string
	sortBy  string
}

func (*Command) Name() string     { return "metrics" }
func (*Command) Synopsis() string { return "Print Robert Martin's package metrics." }
func (*Command) Usage() string {
	return `metrics <expr>:
	Print Robert Martin's package metrics for the given packages.

	Metrics computed:
	  Ca   Afferent couplings: packages that depend on this package.
	  Ce   Efferent couplings: packages this package depends on.
	  A    Abstractness: ratio of interfaces to total type declarations (0..1).
	  I    Instability: Ce / (Ce + Ca) (0..1).
	  D    Distance from the main sequence: |A + I - 1| (0..1).

	With -types flag, additional structural coupling metrics are computed:
	  SCa  Structural afferent coupling: packages whose types satisfy this
	       package's interfaces without importing it.
	  SCe  Structural efferent coupling: packages whose interfaces are
	       satisfied by this package's types without importing them.

	See "help expr" for further information about expressions.
	See "help format" for further information about formatting.
`
}

func (cmd *Command) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&cmd.printStandard, "std", false, "print std packages")
	f.BoolVar(&cmd.typesMode, "types", false, "enable structural coupling analysis (SCa/SCe)")

	f.BoolVar(&cmd.noAlign, "noalign", false, "disable aligning tabs")
	f.StringVar(&cmd.header, "h", "", "header for the table, use \"-\" to skip")
	f.StringVar(&cmd.format, "f", "", "output format")
	f.StringVar(&cmd.sortBy, "sort", "d", "sort by: d (distance), ca, ce, a, i, sca, sce, id")
}

func (cmd *Command) Execute(ctx context.Context, f *flag.FlagSet, _ ...any) subcommands.ExitStatus {
	// Apply defaults based on -types flag.
	if cmd.header == "" {
		if cmd.typesMode {
			cmd.header = typesHeader
		} else {
			cmd.header = defaultHeader
		}
	}
	if cmd.format == "" {
		if cmd.typesMode {
			cmd.format = typesFormat
		} else {
			cmd.format = defaultFormat
		}
	}

	t, err := templates.Parse(cmd.format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid format string: %v\n", err)
		return subcommands.ExitFailure
	}

	if !cmd.printStandard {
		go pkgset.LoadStd()
	}

	result, err := pkgset.CalcWithOpts(ctx, f.Args(), pkgset.CalcOpts{
		TypesMode: cmd.typesMode,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return subcommands.ExitFailure
	}

	// Build graph from the full result for coupling calculations.
	allPkgs := result

	if !cmd.printStandard {
		result = pkgset.Subtract(result, pkgset.Std())
	}

	graph := pkggraph.From(result)
	graph.ComputeMetrics(allPkgs)

	if cmd.typesMode {
		graph.ComputeStructuralCoupling()
	}

	sorted := make([]*pkggraph.Node, len(graph.Sorted))
	copy(sorted, graph.Sorted)

	switch cmd.sortBy {
	case "d":
		sort.Slice(sorted, func(i, k int) bool { return sorted[i].D > sorted[k].D })
	case "ca":
		sort.Slice(sorted, func(i, k int) bool { return sorted[i].Ca > sorted[k].Ca })
	case "ce":
		sort.Slice(sorted, func(i, k int) bool { return sorted[i].Ce > sorted[k].Ce })
	case "a":
		sort.Slice(sorted, func(i, k int) bool { return sorted[i].A > sorted[k].A })
	case "i":
		sort.Slice(sorted, func(i, k int) bool { return sorted[i].I > sorted[k].I })
	case "sca":
		sort.Slice(sorted, func(i, k int) bool { return sorted[i].SCa > sorted[k].SCa })
	case "sce":
		sort.Slice(sorted, func(i, k int) bool { return sorted[i].SCe > sorted[k].SCe })
	case "id":
		// already sorted by ID
	}

	var w io.Writer = os.Stdout
	if !cmd.noAlign {
		w = tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	}
	if cmd.header != "-" {
		fmt.Fprintln(w, cmd.header)
	}
	for _, p := range sorted {
		err := t.Execute(w, p)
		fmt.Fprintln(w)
		if err != nil {
			fmt.Fprintf(os.Stderr, "template error: %v\n", err)
		}
	}
	if w, ok := w.(interface{ Flush() error }); ok {
		w.Flush()
	}

	return subcommands.ExitSuccess
}
