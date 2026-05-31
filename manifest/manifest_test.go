package manifest

import (
	"testing"

	"github.com/cajundata/starshp_manifest/types"
)

func TestBuild(t *testing.T) {
	code := "acct4421"
	mod := "mod02"
	title := "Item 1"
	qs := []types.Question{
		{
			Source:   types.Source{Path: "a/mod02/x.html", CourseCode: &code, Module: &mod},
			Type:     types.TypeMultipleChoice,
			Title:    &title,
			Capture:  types.Capture{HasCapturedChoices: false},
			Warnings: []string{},
		},
		{
			Source:   types.Source{Path: "a/mod02/y.html"},
			Type:     types.TypeWorksheet,
			Capture:  types.Capture{HasCapturedChoices: true},
			Warnings: []string{"dropdown-options-not-captured"},
		},
	}
	m := Build(qs, "a")
	if m.SchemaVersion != types.SchemaVersion || m.GeneratedFrom != "a" || m.Count != 2 {
		t.Fatalf("header wrong: %#v", m)
	}
	if m.Questions[0].CourseCode == nil || *m.Questions[0].CourseCode != "acct4421" {
		t.Errorf("entry0 courseCode = %v", m.Questions[0].CourseCode)
	}
	if m.Questions[1].Warnings != 1 || !m.Questions[1].HasCapturedChoices {
		t.Errorf("entry1 = %#v", m.Questions[1])
	}
}
