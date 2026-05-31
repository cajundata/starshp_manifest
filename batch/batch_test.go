package batch

import (
	"fmt"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/cajundata/starshp_manifest/types"
)

// failOpenFS wraps a MapFS and returns an error when the given path is opened.
// WalkDir (ReadDir) calls are delegated to the embedded MapFS unchanged.
type failOpenFS struct {
	fstest.MapFS
	failPath string
}

func (f failOpenFS) Open(name string) (fs.File, error) {
	if name == f.failPath {
		return nil, fmt.Errorf("simulated open failure")
	}
	return f.MapFS.Open(name)
}

func TestExtractTree(t *testing.T) {
	mc := `<ul class="answers--mc"><li class="answer-wrap--mc"><label class="answer__label--mc"><p>A</p></label></li></ul>`
	ws := `<div class="worksheet-wrap"><div class="worksheet__main"><p>scenario</p></div></div>`
	fsys := fstest.MapFS{
		"acct4421_gov-nonprof-acct/mod01/mod01_001.html": {Data: []byte(mc)},
		"acct4421_gov-nonprof-acct/mod01/mod01_002.html": {Data: []byte(ws)},
		"readme.txt": {Data: []byte("ignore me")},
	}

	var progress [][2]int
	res, err := ExtractTree(fsys, Options{
		Root:     "courseroot",
		Progress: func(done, total int) { progress = append(progress, [2]int{done, total}) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Questions) != 2 {
		t.Fatalf("questions = %d, want 2", len(res.Questions))
	}
	// deterministic order: mod01_001 before mod01_002
	if res.Questions[0].Source.Path != "acct4421_gov-nonprof-acct/mod01/mod01_001.html" {
		t.Errorf("order wrong: %s", res.Questions[0].Source.Path)
	}
	if res.Manifest.Count != 2 || res.Manifest.GeneratedFrom != "courseroot" {
		t.Errorf("manifest = %#v", res.Manifest)
	}
	if res.ByType[types.TypeMultipleChoice] != 1 || res.ByType[types.TypeWorksheet] != 1 {
		t.Errorf("byType = %#v", res.ByType)
	}
	if len(progress) != 2 || progress[1] != [2]int{2, 2} {
		t.Errorf("progress = %v, want final (2,2)", progress)
	}
}

func TestExtractTreeIsolatesFailingFile(t *testing.T) {
	mc := `<ul class="answers--mc"><li class="answer-wrap--mc"><label class="answer__label--mc"><p>A</p></label></li></ul>`
	const badPath = "acct4421_gov-nonprof-acct/mod01/mod01_bad.html"
	const goodPath = "acct4421_gov-nonprof-acct/mod01/mod01_good.html"

	m := fstest.MapFS{
		goodPath: {Data: []byte(mc)},
		badPath:  {Data: []byte(`<p>irrelevant – open will fail</p>`)},
	}
	fsys := failOpenFS{MapFS: m, failPath: badPath}

	res, err := ExtractTree(fsys, Options{Root: "testroot"})
	if err != nil {
		t.Fatalf("ExtractTree returned error: %v", err)
	}
	if len(res.Questions) != 2 {
		t.Fatalf("questions = %d, want 2", len(res.Questions))
	}

	// Locate the two questions by path.
	var badQ, goodQ *types.Question
	for i := range res.Questions {
		q := &res.Questions[i]
		if q.Source.Path == badPath {
			badQ = q
		} else if q.Source.Path == goodPath {
			goodQ = q
		}
	}
	if badQ == nil {
		t.Fatal("no question found for the failing path")
	}
	if goodQ == nil {
		t.Fatal("no question found for the good path")
	}

	// The failing file must degrade to TypeUnknown with WarnParseFailed.
	if badQ.Type != types.TypeUnknown {
		t.Errorf("failing file type = %q, want TypeUnknown", badQ.Type)
	}
	foundWarn := false
	for _, w := range badQ.Warnings {
		if w == types.WarnParseFailed {
			foundWarn = true
			break
		}
	}
	if !foundWarn {
		t.Errorf("failing file warnings = %v, want to contain %q", badQ.Warnings, types.WarnParseFailed)
	}

	// The good file must extract normally.
	if goodQ.Type != types.TypeMultipleChoice {
		t.Errorf("good file type = %q, want TypeMultipleChoice", goodQ.Type)
	}

	// Aggregate counters.
	if res.Unknowns != 1 {
		t.Errorf("Unknowns = %d, want 1", res.Unknowns)
	}
}
