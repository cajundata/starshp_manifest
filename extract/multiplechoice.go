package extract

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/cajundata/starshp_manifest/internal/text"
	"github.com/cajundata/starshp_manifest/types"
)

// extractMultipleChoice returns (body, title, warnings).
func extractMultipleChoice(doc *goquery.Document) (any, *string, []string) {
	warns := []string{}

	stem, stemTable := extractStem(doc)

	body := types.MultipleChoiceBody{
		Stem:      stem,
		StemTable: stemTable,
		Choices:   []types.MCChoice{},
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

// extractStem gathers the full question stem. The stem lives in p.question, but
// HTML5 forbids the block elements (<p>, <table>) that Connect nests inside it,
// so the parser auto-closes p.question and hoists those blocks up as following
// siblings within the same .question-wrap (the answers live in a separate
// container, so they are never captured here). The stem is therefore p.question
// PLUS every following sibling: prose paragraphs are joined into the returned
// string, and the first embedded <table> is captured as a StemTable.
func extractStem(doc *goquery.Document) (string, *types.StemTable) {
	q := doc.Find("p.question").First()
	// Bound the sweep at the answers: in some snapshots .answers-wrap is a direct
	// sibling of p.question, so NextUntil stops before it; when the answers live
	// in a separate container, nothing matches and we take every following
	// sibling within the question container.
	region := q.AddSelection(q.NextUntil(".answers-wrap"))

	var stemTable *types.StemTable
	captureTable := func(sel *goquery.Selection) {
		if stemTable == nil {
			t := parseStemTable(sel)
			stemTable = &t
		}
	}

	var parts []string
	region.Each(func(_ int, s *goquery.Selection) {
		// A hoisted sibling <table>.
		if goquery.NodeName(s) == "table" {
			captureTable(s)
			return
		}
		// A well-formed stem keeps the table nested inside a paragraph; pull it
		// out so its cell text does not bleed into the prose.
		if inner := s.Find("table").First(); inner.Length() > 0 {
			captureTable(inner)
			inner.Remove()
		}
		if txt := text.Normalize(s.Text()); txt != "" {
			parts = append(parts, txt)
		}
	})
	return strings.Join(parts, " "), stemTable
}

// parseStemTable converts a plain HTML <table> into a StemTable. A row whose
// cells are all <th> (no <td>) becomes the column headers; otherwise the first
// <th> is the row label and each <td> contributes a value cell.
func parseStemTable(tbl *goquery.Selection) types.StemTable {
	out := types.StemTable{Headers: []string{}, Rows: []types.StemTableRow{}}
	tbl.Find("tr").Each(func(_ int, tr *goquery.Selection) {
		ths := tr.Find("th")
		tds := tr.Find("td")
		if ths.Length() > 0 && tds.Length() == 0 {
			ths.Each(func(_ int, th *goquery.Selection) {
				out.Headers = append(out.Headers, text.Normalize(th.Text()))
			})
			return
		}
		row := types.StemTableRow{Cells: []types.StemTableCell{}}
		if rh := ths.First(); rh.Length() > 0 {
			row.Label = text.Normalize(rh.Text())
		}
		tds.Each(func(_ int, td *goquery.Selection) {
			row.Cells = append(row.Cells, types.StemTableCell{Value: text.Normalize(td.Text())})
		})
		out.Rows = append(out.Rows, row)
	})
	return out
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
