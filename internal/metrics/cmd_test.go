package metrics_test

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func TestMetrics(t *testing.T) {
	goda := buildGoda(t)

	testdataDir, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	projectDir := filepath.Join(testdataDir, "testproject")

	tests := []struct {
		name   string
		args   []string
		golden string
	}{
		{
			name:   "metrics_sorted_by_distance",
			args:   []string{"metrics", "-std", "./..."},
			golden: "metrics.golden",
		},
		{
			name:   "metrics_sorted_by_id",
			args:   []string{"metrics", "-std", "-sort", "id", "./..."},
			golden: "list_metrics.golden",
		},
		{
			name: "list_with_metrics",
			args: []string{
				"list", "-std",
				"-h", "ID\tCa\tCe\tA\tI\tD",
				"-f", "{{.ID}}\t{{.Ca}}\t{{.Ce}}\t{{printf \"%.2f\" .A}}\t{{printf \"%.2f\" .I}}\t{{printf \"%.2f\" .D}}",
				"./...",
			},
			golden: "list_metrics.golden",
		},
		{
			name:   "metrics_with_types",
			args:   []string{"metrics", "-types", "-std", "-sort", "id", "./..."},
			golden: "metrics_types.golden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(goda, tt.args...)
			cmd.Dir = projectDir
			got, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("goda %v failed: %v\n%s", tt.args, err, got)
			}

			goldenPath := filepath.Join(testdataDir, tt.golden)

			if *update {
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("reading golden file: %v (run with -update to create)", err)
			}

			if string(got) != string(want) {
				t.Errorf("output mismatch for %q\n\ngot:\n%s\nwant:\n%s", tt.name, got, want)
			}
		})
	}
}

func buildGoda(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	bin := filepath.Join(dir, "goda")

	cmd := exec.Command("go", "build", "-o", bin, "github.com/loov/goda")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("building goda: %v\n%s", err, output)
	}

	return bin
}
