package extract

import (
	"strconv"

	"github.com/PuerkitoBio/goquery"
	"github.com/cajundata/starshp_manifest/internal/text"
	"github.com/cajundata/starshp_manifest/types"
)

func extractMatching(doc *goquery.Document) (any, *string, []string) {
	body := types.MatchingBody{Terms: []string{}, Choices: []string{}}
	doc.Find(".match-row .match-prompt-label .content").Each(func(_ int, s *goquery.Selection) {
		body.Terms = append(body.Terms, text.Normalize(s.Text()))
	})
	doc.Find(".choices-container .choice-item-wrapper .choice-item .content").Each(func(_ int, s *goquery.Selection) {
		body.Choices = append(body.Choices, text.Normalize(s.Text()))
	})
	return body, nil, []string{types.WarnDLCUnverified}
}

func extractFillInTheBlank(doc *goquery.Document) (any, *string, []string) {
	// Replace each .fitb-input with a [BLANK n] marker, then normalize.
	prompt := doc.Find(".prompt").First()
	n := 0
	prompt.Find(".fitb-input").Each(func(_ int, s *goquery.Selection) {
		n++
		s.ReplaceWithHtml(" [BLANK " + strconv.Itoa(n) + "] ")
	})
	body := types.FillInTheBlankBody{PromptWithBlanks: text.Normalize(prompt.Text())}
	return body, nil, []string{types.WarnDLCUnverified}
}

func extractTrueFalse(doc *goquery.Document) (any, *string, []string) {
	body := types.TrueFalseBody{
		Stem:         text.Normalize(doc.Find(".prompt").First().Text()),
		Choices:      choiceTexts(doc),
		CorrectIndex: nil,
	}
	return body, nil, []string{types.WarnDLCUnverified}
}

func extractMultipleSelect(doc *goquery.Document) (any, *string, []string) {
	body := types.MultipleSelectBody{
		Stem:           text.Normalize(doc.Find(".prompt").First().Text()),
		Choices:        []types.MCChoice{},
		CorrectIndices: nil,
	}
	for i, t := range choiceTexts(doc) {
		body.Choices = append(body.Choices, types.MCChoice{Index: i, Text: t})
	}
	return body, nil, []string{types.WarnDLCUnverified}
}

func choiceTexts(doc *goquery.Document) []string {
	out := []string{}
	doc.Find(".choiceText").Each(func(_ int, s *goquery.Selection) {
		out = append(out, text.Normalize(s.Text()))
	})
	return out
}
