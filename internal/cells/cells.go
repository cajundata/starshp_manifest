// Package cells parses jSheet worksheet tables (and their dropdown choices)
// into the types.Table schema. Lossless: it never invents data.
package cells

import (
	"regexp"
	"sort"
	"strconv"

	"github.com/PuerkitoBio/goquery"
	"github.com/cajundata/starshp_manifest/internal/text"
	"github.com/cajundata/starshp_manifest/types"
)

var cellIDRe = regexp.MustCompile(`_cell_c(\d+)_r(\d+)`)

type parsedCell struct {
	col, row int
	sel      *goquery.Selection
	internal string // colHeader | rowHeader | dropdown | formula | input
}

// ParseTable converts one div.jSheetParent into a types.Table. root is the
// document-level selection used to resolve dropdown choices that newer captures
// store outside the cell in a shared #codex-captured-choices-library; pass nil
// when choices are inline (or absent).
func ParseTable(parent *goquery.Selection, root *goquery.Selection) types.Table {
	var pcs []parsedCell
	parent.Find("td[id*='_cell_c']").Each(func(_ int, s *goquery.Selection) {
		id, _ := s.Attr("id")
		m := cellIDRe.FindStringSubmatch(id)
		if m == nil {
			return
		}
		col, _ := strconv.Atoi(m[1])
		row, _ := strconv.Atoi(m[2])
		pcs = append(pcs, parsedCell{col: col, row: row, sel: s, internal: internalType(s)})
	})

	// Group by row.
	byRow := map[int][]parsedCell{}
	var rowOrder []int
	for _, pc := range pcs {
		if _, ok := byRow[pc.row]; !ok {
			rowOrder = append(rowOrder, pc.row)
		}
		byRow[pc.row] = append(byRow[pc.row], pc)
	}
	sort.Ints(rowOrder)

	tbl := types.Table{Headers: []string{}, Rows: []types.Row{}}
	for _, r := range rowOrder {
		group := byRow[r]
		sort.Slice(group, func(i, j int) bool { return group[i].col < group[j].col })

		if allColHeaders(group) {
			for _, pc := range group {
				tbl.Headers = append(tbl.Headers, text.Normalize(pc.sel.Text()))
			}
			continue
		}

		row := types.Row{Cells: []types.Cell{}}
		labelTaken := false
		for _, pc := range group {
			if pc.internal == "rowHeader" && !labelTaken {
				row.Label = text.Normalize(pc.sel.Text())
				labelTaken = true
				continue
			}
			row.Cells = append(row.Cells, buildCell(pc, root))
		}
		tbl.Rows = append(tbl.Rows, row)
	}
	return tbl
}

func allColHeaders(group []parsedCell) bool {
	for _, pc := range group {
		if pc.internal != "colHeader" {
			return false
		}
	}
	return len(group) > 0
}

func internalType(s *goquery.Selection) string {
	switch {
	case s.HasClass("colHeader"):
		return "colHeader"
	case s.HasClass("rowHeader"):
		return "rowHeader"
	case s.HasClass("dropDownList") || s.AttrOr("dropdowntype", "") == "dropDown":
		return "dropdown"
	case s.AttrOr("formula", "") != "":
		return "formula"
	default:
		return "input"
	}
}

func buildCell(pc parsedCell, root *goquery.Selection) types.Cell {
	id, _ := pc.sel.Attr("id")
	c := types.Cell{
		ID:        id,
		AriaLabel: text.Normalize(pc.sel.AttrOr("aria-label", "")),
		Options:   []types.DropdownOption{},
	}
	if f := pc.sel.AttrOr("formula", ""); f != "" {
		c.Formula = &f
	}
	switch pc.internal {
	case "dropdown":
		c.CellType = "dropdown"
		c.Options = parseOptions(pc.sel, root)
		// dropdown cell .Text() would include option text; value stays nil.
	case "formula":
		c.CellType = "formula"
		c.Value = textOrNil(pc.sel)
	case "colHeader", "rowHeader":
		c.CellType = "readonly"
		c.Value = textOrNil(pc.sel)
	default:
		c.CellType = "input"
		c.Value = textOrNil(pc.sel)
	}
	return c
}

func textOrNil(s *goquery.Selection) *string {
	v := text.Normalize(s.Text())
	if v == "" {
		return nil
	}
	return &v
}

// parseOptions reads the cell's captured dropdown choices. It prefers the cell's
// own div.codex-captured-choices (older inline captures), ignoring any duplicate
// live listbox (div.listContainer). When the cell has no inline choices, it falls
// back to the shared #codex-captured-choices-library entry whose data-dropdownid
// matches the cell's dropdownid (newer captures store choices there). Many cells
// may share one dropdownid, so the same library entry can feed several cells.
func parseOptions(cell, root *goquery.Selection) []types.DropdownOption {
	choices := cell.Find(".codex-captured-choices")
	if choices.Length() == 0 && root != nil {
		if id := cell.AttrOr("dropdownid", ""); id != "" {
			choices = root.Find(".codex-captured-choices[data-dropdownid='" + id + "']")
		}
	}

	opts := []types.DropdownOption{}
	choices.Find("li[role='option']").Each(func(i int, li *goquery.Selection) {
		opts = append(opts, types.DropdownOption{
			Index:   i,
			Text:    text.Normalize(li.Find("a.list_content").Text()),
			Correct: li.Find("span.answer_holder").HasClass("k"),
		})
	})
	return opts
}

// HasCapturedChoices reports whether any dropdown choices were captured anywhere in the doc.
func HasCapturedChoices(root *goquery.Selection) bool {
	return root.Find(".codex-captured-choices").Length() > 0
}
