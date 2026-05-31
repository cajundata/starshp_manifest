// Package classify determines a Connect page's question type (the discriminator).
package classify

import (
	"regexp"

	"github.com/PuerkitoBio/goquery"
	"github.com/cajundata/starshp_manifest/types"
)

var probeRe = regexp.MustCompile(`awd-probe-type-([a-z_]+)`)

// Classify inspects the document in real-data priority order:
//  1. .worksheet-wrap        -> worksheet
//  2. .dlc_question          -> branch on awd-probe-type-*
//  3. ul.answers--mc         -> multipleChoice
//  4. else                   -> unknown
func Classify(doc *goquery.Document) types.QuestionType {
	if doc.Find(".worksheet-wrap").Length() > 0 {
		return types.TypeWorksheet
	}
	if dlc := doc.Find(".dlc_question"); dlc.Length() > 0 {
		if t := probeType(dlc); t != types.TypeUnknown {
			return t
		}
	}
	if doc.Find("ul.answers--mc").Length() > 0 {
		return types.TypeMultipleChoice
	}
	return types.TypeUnknown
}

func probeType(dlc *goquery.Selection) types.QuestionType {
	class, _ := dlc.First().Attr("class")
	m := probeRe.FindStringSubmatch(class)
	if m == nil {
		return types.TypeUnknown
	}
	switch m[1] {
	case "matching":
		return types.TypeMatching
	case "fill_in_the_blank":
		return types.TypeFillInTheBlank
	case "true_false":
		return types.TypeTrueFalse
	case "multiple_select":
		return types.TypeMultipleSelect
	case "multiple_choice":
		return types.TypeMultipleChoice
	default:
		return types.TypeUnknown
	}
}
