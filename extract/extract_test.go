package extract

import (
	"strings"
	"testing"

	"github.com/cajundata/starshp_manifest/types"
)

func TestExtractHTMLMultipleChoice(t *testing.T) {
	q, err := ExtractHTML(strings.NewReader(mcHTML), types.Source{Path: "acct4421_gov-nonprof-acct/mod02/mod02_001.html"})
	if err != nil {
		t.Fatal(err)
	}
	if q.Type != types.TypeMultipleChoice {
		t.Errorf("type = %q", q.Type)
	}
	if q.SchemaVersion != types.SchemaVersion {
		t.Errorf("schemaVersion = %d", q.SchemaVersion)
	}
	if q.Source.CourseCode == nil || *q.Source.CourseCode != "acct4421" {
		t.Errorf("taxonomy not derived: %#v", q.Source)
	}
	if q.Tags == nil || len(q.Tags) != 0 {
		t.Errorf("tags must be empty slice, got %#v", q.Tags)
	}
	if q.Capture.HasCapturedChoices {
		t.Errorf("MC should not report captured choices")
	}
	if _, ok := q.Body.(types.MultipleChoiceBody); !ok {
		t.Errorf("body type = %T", q.Body)
	}
}

func TestExtractHTMLWorksheetCaptureBlock(t *testing.T) {
	q, _ := ExtractHTML(strings.NewReader(wsCapturedPanels), types.Source{Path: "x/004.html"})
	if q.Type != types.TypeWorksheet {
		t.Fatalf("type = %q", q.Type)
	}
	if !q.Capture.HasCapturedChoices {
		t.Errorf("should report captured choices")
	}
	if q.Capture.CapturedTabCount != 2 {
		t.Errorf("capturedTabCount = %d, want 2", q.Capture.CapturedTabCount)
	}
	// path has no acct#### -> course-not-derivable warning present
	found := false
	for _, w := range q.Warnings {
		if w == types.WarnCourseNotDerivable {
			found = true
		}
	}
	if !found {
		t.Errorf("expected course-not-derivable warning, got %v", q.Warnings)
	}
}

func TestExtractHTMLUnknown(t *testing.T) {
	q, _ := ExtractHTML(strings.NewReader(`<div class="weird-thing other"></div>`), types.Source{Path: "001.html"})
	if q.Type != types.TypeUnknown {
		t.Fatalf("type = %q", q.Type)
	}
	ub := q.Body.(types.UnknownBody)
	if len(ub.RawClasses) == 0 {
		t.Errorf("unknown body should list rawClasses")
	}
}
