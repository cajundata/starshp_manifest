package extract

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cajundata/starshp_manifest/types"
)

var update = flag.Bool("update", false, "regenerate golden files")

func TestGolden(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		golden   string
		wantType types.QuestionType
	}{
		{"mc_001", "../testdata/mc_001.html", "../testdata/mc_001.golden.json", types.TypeMultipleChoice},
		{"mc_graded_001", "../testdata/mc_graded_001.html", "../testdata/mc_graded_001.golden.json", types.TypeMultipleChoice},
		{"ws_005", "../testdata/ws_005.html", "../testdata/ws_005.golden.json", types.TypeWorksheet},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, err := os.Open(c.input)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			q, err := ExtractHTML(f, types.Source{Path: filepath.Base(c.input)})
			if err != nil {
				t.Fatal(err)
			}
			if q.Type != c.wantType {
				t.Fatalf("type = %q, want %q", q.Type, c.wantType)
			}
			got, err := json.MarshalIndent(q, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if *update {
				if err := os.WriteFile(c.golden, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(c.golden)
			if err != nil {
				t.Fatalf("read golden (run `go test ./extract -run TestGolden -update` first): %v", err)
			}
			// Normalize line endings before comparing: core.autocrlf can
			// materialize the golden file with CRLF in the working tree, while
			// json.MarshalIndent always emits LF. The comparison is on content,
			// not line-ending style.
			gotStr := strings.ReplaceAll(string(got), "\r\n", "\n")
			wantStr := strings.ReplaceAll(string(want), "\r\n", "\n")
			if gotStr != wantStr {
				t.Errorf("golden mismatch for %s. Run with -update if the change is intended.\n--- got ---\n%s", c.name, gotStr)
			}
		})
	}
}
