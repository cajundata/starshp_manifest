package types

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestQuestionMarshalsExpectedShape(t *testing.T) {
	code := "acct4421"
	name := "Governmental & Not-for-Profit Accounting"
	mod := "mod02"
	title := "Item 1"
	q := Question{
		SchemaVersion: 1,
		Source:        Source{Path: "acct4421/mod02/x.html", CourseCode: &code, CourseName: &name, Module: &mod},
		Type:          TypeMultipleChoice,
		Title:         &title,
		Capture:       Capture{HasCapturedChoices: false, CapturedTabCount: 0},
		Warnings:      []string{},
		Tags:          []string{},
		Body:          MultipleChoiceBody{Stem: "Q?", Choices: []MCChoice{{Index: 0, Text: "A"}}, CorrectIndex: nil},
	}
	b, err := json.Marshal(q)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	for _, want := range []string{
		`"schemaVersion":1`, `"courseCode":"acct4421"`, `"type":"multipleChoice"`,
		`"tags":[]`, `"warnings":[]`, `"correctIndex":null`, `"hasCapturedChoices":false`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("marshaled JSON missing %q\n got: %s", want, got)
		}
	}
}

func TestEmptySlicesMarshalAsArraysNotNull(t *testing.T) {
	q := Question{SchemaVersion: 1, Warnings: []string{}, Tags: []string{}, Body: UnknownBody{RawClasses: []string{}, Note: "x"}}
	b, _ := json.Marshal(q)
	if strings.Contains(string(b), `"warnings":null`) || strings.Contains(string(b), `"tags":null`) {
		t.Errorf("nil slice leaked as null: %s", b)
	}
}
