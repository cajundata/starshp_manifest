package audit

import (
	"testing"
	"testing/fstest"
)

func TestScanReportsUnknownClasses(t *testing.T) {
	fsys := fstest.MapFS{
		"a.html": {Data: []byte(`<div class="worksheet-wrap"></div><div class="mystery-grid widget-xyz"></div>`)},
		"b.html": {Data: []byte(`<span class="widget-xyz"></span>`)},
		"notes.txt": {Data: []byte("ignored")},
	}
	counts, err := Scan(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if counts["worksheet-wrap"] != 0 {
		t.Errorf("known class worksheet-wrap should be excluded, got %d", counts["worksheet-wrap"])
	}
	if counts["widget-xyz"] != 2 {
		t.Errorf("widget-xyz = %d, want 2", counts["widget-xyz"])
	}
	if counts["mystery-grid"] != 1 {
		t.Errorf("mystery-grid = %d, want 1", counts["mystery-grid"])
	}
}
