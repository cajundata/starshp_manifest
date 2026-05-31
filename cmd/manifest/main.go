// Command manifest is the CLI frontend over the manifest extractor core.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cajundata/starshp_manifest/audit"
	"github.com/cajundata/starshp_manifest/batch"
)

const usage = `manifest — Connect HTML -> JSON extractor

Usage:
  manifest extract <inputDir> [-o outDir]   per-file JSON + manifest index
  manifest plan    <inputDir>               dry run: classify, print breakdown
  manifest audit   <inputDir>               dump unknown CSS classes
`

func main() {
	if err := dispatch(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func dispatch(args []string, w io.Writer) error {
	if len(args) < 2 {
		fmt.Fprint(w, usage)
		if len(args) == 0 {
			return nil
		}
		return fmt.Errorf("missing input directory")
	}
	verb, inputDir := args[0], args[1]
	switch verb {
	case "extract":
		out := defaultOut(inputDir)
		if i := indexOf(args, "-o"); i >= 0 && i+1 < len(args) {
			out = args[i+1]
		}
		return runExtract(w, inputDir, out)
	case "plan":
		return runPlan(w, inputDir)
	case "audit":
		return runAudit(w, inputDir)
	default:
		fmt.Fprint(w, usage)
		return fmt.Errorf("unknown command %q", verb)
	}
}

func defaultOut(inputDir string) string {
	return filepath.Join(inputDir, "_json")
}

func indexOf(args []string, flag string) int {
	for i, a := range args {
		if a == flag {
			return i
		}
	}
	return -1
}

func runExtract(w io.Writer, inputDir, outDir string) error {
	res, err := batch.ExtractTree(os.DirFS(inputDir), batch.Options{
		Root: inputDir,
		Progress: func(done, total int) {
			fmt.Fprintf(w, "\rextracting %d/%d", done, total)
		},
	})
	if err != nil {
		return err
	}
	fmt.Fprintln(w)

	for _, q := range res.Questions {
		rel := strings.TrimSuffix(q.Source.Path, filepath.Ext(q.Source.Path)) + ".json"
		dst := filepath.Join(outDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		data, err := json.MarshalIndent(q, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return err
		}
	}

	mdata, err := json.MarshalIndent(res.Manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "manifest.json"), mdata, 0o644); err != nil {
		return err
	}

	printSummary(w, res)
	return nil
}

func runPlan(w io.Writer, inputDir string) error {
	res, err := batch.ExtractTree(os.DirFS(inputDir), batch.Options{Root: inputDir})
	if err != nil {
		return err
	}
	printSummary(w, res)
	return nil
}

func runAudit(w io.Writer, inputDir string) error {
	counts, err := audit.Scan(os.DirFS(inputDir))
	if err != nil {
		return err
	}
	type kv struct {
		class string
		n     int
	}
	rows := make([]kv, 0, len(counts))
	for c, n := range counts {
		rows = append(rows, kv{c, n})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].n != rows[j].n {
			return rows[i].n > rows[j].n
		}
		return rows[i].class < rows[j].class
	})
	fmt.Fprintf(w, "unknown CSS classes (%d distinct):\n", len(rows))
	for _, r := range rows {
		fmt.Fprintf(w, "%6d  %s\n", r.n, r.class)
	}
	return nil
}

func printSummary(w io.Writer, res batch.Result) {
	fmt.Fprintf(w, "extracted %d question(s)\n", len(res.Questions))
	names := make([]string, 0, len(res.ByType))
	counts := map[string]int{}
	for t, n := range res.ByType {
		names = append(names, string(t))
		counts[string(t)] = n
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(w, "  %-16s %d\n", name, counts[name])
	}
	fmt.Fprintf(w, "files with warnings: %d   unknown: %d\n", res.FilesWithWarning, res.Unknowns)
}
