package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cajundata/starshp_manifest/types"
)

func TestRunExtractWritesPerFileJSONAndManifest(t *testing.T) {
	in := t.TempDir()
	out := t.TempDir()
	mc := `<ul class="answers--mc"><li class="answer-wrap--mc"><label class="answer__label--mc"><p>A</p></label></li></ul>`
	if err := os.MkdirAll(filepath.Join(in, "mod01"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(in, "mod01", "q1.html"), []byte(mc), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := runExtract(&buf, in, out); err != nil {
		t.Fatalf("runExtract: %v", err)
	}

	// per-file JSON mirrors the input tree
	jsonPath := filepath.Join(out, "mod01", "q1.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("expected %s: %v", jsonPath, err)
	}
	var q types.Question
	if err := json.Unmarshal(data, &q); err != nil {
		t.Fatalf("per-file JSON invalid: %v", err)
	}
	if q.Type != types.TypeMultipleChoice {
		t.Errorf("type = %q", q.Type)
	}

	// manifest at output root
	if _, err := os.Stat(filepath.Join(out, "manifest.json")); err != nil {
		t.Errorf("manifest.json missing: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("multipleChoice")) {
		t.Errorf("summary should mention type counts; got: %s", buf.String())
	}
}

func TestRunPlanWritesNothing(t *testing.T) {
	in := t.TempDir()
	if err := os.WriteFile(filepath.Join(in, "q.html"), []byte(`<ul class="answers--mc"></ul>`), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := runPlan(&buf, in); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(in)
	if len(entries) != 1 {
		t.Errorf("plan must not write files; dir now has %d entries", len(entries))
	}
}
