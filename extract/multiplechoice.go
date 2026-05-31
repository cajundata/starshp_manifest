package extract

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/cajundata/starshp_manifest/internal/text"
	"github.com/cajundata/starshp_manifest/types"
)

// extractMultipleChoice returns (body, title, warnings).
func extractMultipleChoice(doc *goquery.Document) (any, *string, []string) {
	warns := []string{}

	// The stem is in p.question; when the source HTML wraps an inner <p>,
	// browsers/parsers hoist the inner <p> as the next sibling, leaving
	// p.question empty. Fall back to the next sibling <p> in that case.
	stemSel := doc.Find("p.question").First()
	stem := text.Normalize(stemSel.Text())
	if stem == "" {
		stem = text.Normalize(stemSel.Next().Text())
	}

	body := types.MultipleChoiceBody{
		Stem:    stem,
		Choices: []types.MCChoice{},
		// Stays nil for ungraded snapshots. Graded snapshots tag the correct
		// option's .answer__span--mc with the "is-correct" class (see loop).
		CorrectIndex: nil,
	}

	doc.Find("li.answer-wrap--mc").Each(func(i int, li *goquery.Selection) {
		t := text.Normalize(li.Find(".answer__label--mc p").First().Text())
		if t == "" {
			t = text.Normalize(li.Text())
		}
		body.Choices = append(body.Choices, types.MCChoice{Index: i, Text: t})

		// Graded MC snapshots mark the correct option's span with "is-correct"
		// (independent of which option the student selected). First match wins.
		if body.CorrectIndex == nil && li.Find(".answer__span--mc").First().HasClass("is-correct") {
			idx := i
			body.CorrectIndex = &idx
		}
	})

	title := titleOrWarn(doc, &warns)
	return body, title, warns
}

// titleOrWarn resolves the MC title from #question-info-holder ("Item N"),
// appending WarnMissingTitle when absent.
func titleOrWarn(doc *goquery.Document, warns *[]string) *string {
	t := text.Normalize(doc.Find("#question-info-holder").First().Text())
	if t == "" {
		*warns = append(*warns, types.WarnMissingTitle)
		return nil
	}
	return &t
}
