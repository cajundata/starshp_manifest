// Package extract converts a single Connect HTML document into a types.Question.
// It is pure: no printing, no filesystem, no process exit.
package extract

import (
	"io"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/cajundata/starshp_manifest/classify"
	"github.com/cajundata/starshp_manifest/internal/cells"
	"github.com/cajundata/starshp_manifest/taxonomy"
	"github.com/cajundata/starshp_manifest/types"
)

// ExtractHTML parses r and returns the populated Question envelope. The error is
// non-nil only when the HTML cannot be parsed at all.
func ExtractHTML(r io.Reader, src types.Source) (types.Question, error) {
	derived, taxWarn := taxonomy.Derive(src.Path)
	// Preserve caller-provided path; take derived course/module fields.
	derived.Path = src.Path

	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return types.Question{
			SchemaVersion: types.SchemaVersion,
			Source:        derived,
			Type:          types.TypeUnknown,
			Capture:       types.Capture{},
			Warnings:      mergeWarn(taxWarn, types.WarnParseFailed),
			Tags:          []string{},
			Body:          types.UnknownBody{RawClasses: []string{}, Note: "html parse failed"},
		}, err
	}

	qType := classify.Classify(doc)
	body, title, warns := dispatch(qType, doc)

	q := types.Question{
		SchemaVersion: types.SchemaVersion,
		Source:        derived,
		Type:          qType,
		Title:         title,
		Capture: types.Capture{
			HasCapturedChoices: cells.HasCapturedChoices(doc.Selection),
			CapturedTabCount:   capturedTabCount(doc),
		},
		Warnings: mergeWarn(taxWarn, warns...),
		Tags:     []string{},
		Body:     body,
	}
	return q, nil
}

func dispatch(t types.QuestionType, doc *goquery.Document) (any, *string, []string) {
	switch t {
	case types.TypeWorksheet:
		return extractWorksheet(doc)
	case types.TypeMultipleChoice:
		return extractMultipleChoice(doc)
	case types.TypeMatching:
		return extractMatching(doc)
	case types.TypeFillInTheBlank:
		return extractFillInTheBlank(doc)
	case types.TypeTrueFalse:
		return extractTrueFalse(doc)
	case types.TypeMultipleSelect:
		return extractMultipleSelect(doc)
	default:
		return unknownBody(doc), nil, []string{}
	}
}

func unknownBody(doc *goquery.Document) types.UnknownBody {
	set := map[string]struct{}{}
	doc.Find("[class]").Each(func(_ int, s *goquery.Selection) {
		for _, c := range strings.Fields(s.AttrOr("class", "")) {
			set[c] = struct{}{}
		}
	})
	classes := make([]string, 0, len(set))
	for c := range set {
		classes = append(classes, c)
	}
	sort.Strings(classes)
	return types.UnknownBody{RawClasses: classes, Note: "no discriminator matched"}
}

// mergeWarn combines an optional taxonomy warning ("" = none) with extractor warnings.
func mergeWarn(taxWarn string, rest ...string) []string {
	out := []string{}
	if taxWarn != "" {
		out = append(out, taxWarn)
	}
	out = append(out, rest...)
	if out == nil {
		return []string{}
	}
	return out
}
