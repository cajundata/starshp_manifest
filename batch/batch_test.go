package batch

import (
	"testing"
	"testing/fstest"

	"github.com/cajundata/starshp_manifest/types"
)

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
