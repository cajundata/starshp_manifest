package extract

import (
	"strconv"

	"github.com/PuerkitoBio/goquery"
	"github.com/cajundata/starshp_manifest/internal/cells"
	"github.com/cajundata/starshp_manifest/internal/text"
	"github.com/cajundata/starshp_manifest/types"
)

func extractWorksheet(doc *goquery.Document) (any, *string, []string) {
	warns := []string{}

	body := types.WorksheetBody{
		Scenario: text.Normalize(doc.Find(".worksheet__main > p").First().Text()),
		Required: []string{},
		Tabs:     []types.Tab{},
	}

	doc.Find(".worksheet__main ol").First().Find("li").Each(func(_ int, li *goquery.Selection) {
		if t := text.Normalize(li.Text()); t != "" {
			body.Required = append(body.Required, t)
		}
	})

	panels := doc.Find("#codex-captured-tab-panels section.captured-tab-panel")
	if panels.Length() > 0 {
		panels.Each(func(_ int, panel *goquery.Selection) {
			label := text.Normalize(panel.AttrOr("data-tab-label", ""))
			tab := types.Tab{Label: &label, Tables: parseTables(panel, &warns)}
			body.Tabs = append(body.Tabs, tab)
		})
	} else {
		src := doc.Find(".captured-iframe-content").First()
		if src.Length() == 0 {
			src = doc.Find(".worksheet-wrap").First()
		}
		body.Tabs = append(body.Tabs, types.Tab{Label: nil, Tables: parseTables(src, &warns)})
	}

	// Worksheets normally have no title; only set when an explicit one exists.
	var title *string
	if t := text.Normalize(doc.Find(".worksheet-wrap .question__title").First().Text()); t != "" {
		title = &t
	}
	return body, title, warns
}

// parseTables parses every div.jSheetParent under root, accumulating a
// dropdown-not-captured warning when a dropdown cell has no options.
//
// Two complementary checks cover both capture states:
//  1. Cells that cells.ParseTable classifies as "dropdown" (has dropDownList
//     class or dropdowntype attr) but have zero parsed options.
//  2. Raw responseCell td elements that carry NO dropdown marker at all —
//     these are dropdowns whose type annotation was never written because the
//     entire cell was left uncaptured; they are identified by the absence of
//     codex-captured-choices content.
func parseTables(root *goquery.Selection, warns *[]string) []types.Table {
	tables := []types.Table{}
	root.Find(".jSheetParent").Each(func(_ int, p *goquery.Selection) {
		tbl := cells.ParseTable(p)
		for _, row := range tbl.Rows {
			for _, c := range row.Cells {
				if c.CellType == "dropdown" && len(c.Options) == 0 {
					*warns = appendUnique(*warns, types.WarnDropdownNotCaptured)
				}
			}
		}
		// Also flag responseCell tds that have no dropdown markers at all (fully
		// uncaptured dropdowns the typed path above never sees as "dropdown").
		p.Find("td.responseCell").Each(func(_ int, td *goquery.Selection) {
			isMarkedDropdown := td.HasClass("dropDownList") || td.AttrOr("dropdowntype", "") == "dropDown"
			hasCapturedChoices := td.Find(".codex-captured-choices").Length() > 0
			if !isMarkedDropdown && !hasCapturedChoices {
				*warns = appendUnique(*warns, types.WarnDropdownNotCaptured)
			}
		})
		tables = append(tables, tbl)
	})
	return tables
}

func appendUnique(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}

// capturedTabCount reads the data-captured-tab-count attribute (0 if absent).
func capturedTabCount(doc *goquery.Document) int {
	v := doc.Find("#codex-captured-tab-panels").AttrOr("data-captured-tab-count", "")
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}
