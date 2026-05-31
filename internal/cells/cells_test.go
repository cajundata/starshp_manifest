package cells

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

const sample = `
<div class="jSheetParent">
  <table class="jSheet">
    <tbody>
      <tr>
        <td class="colHeader td-readOnly" id="0_table0_cell_c0_r0"></td>
        <td class="colHeader td-readOnly" id="0_table0_cell_c1_r0">Amount</td>
      </tr>
      <tr>
        <td class="rowHeader td-readOnly" id="0_table0_cell_c0_r1">Program revenue</td>
        <td class="dropDownList responseCell" id="0_table0_cell_c1_r1" dropdowntype="dropDown" aria-label="  ">
          <div class="codex-captured-choices">
            <ul role="listbox" aria-labelledby="0_table0_cell_c1_r1">
              <li role="option"><span class="answer_holder k"></span><a class="list_content"></a></li>
              <li role="option"><span class="answer_holder"></span><a class="list_content">Program revenue</a></li>
              <li role="option"><span class="answer_holder"></span><a class="list_content">Service revenue</a></li>
            </ul>
          </div>
        </td>
      </tr>
    </tbody>
  </table>
  <div class="listContainer"><ul id="listbox-id"><li role="option"><a class="list_content">DUPLICATE</a></li></ul></div>
</div>`

func parentSel(t *testing.T, html string) *goquery.Selection {
	t.Helper()
	d, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	return d.Find(".jSheetParent").First()
}

func TestParseTableHeadersAndRows(t *testing.T) {
	tbl := ParseTable(parentSel(t, sample))

	if len(tbl.Headers) != 2 || tbl.Headers[0] != "" || tbl.Headers[1] != "Amount" {
		t.Fatalf("headers = %#v, want [\"\" \"Amount\"]", tbl.Headers)
	}
	if len(tbl.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(tbl.Rows))
	}
	row := tbl.Rows[0]
	if row.Label != "Program revenue" {
		t.Errorf("row label = %q, want \"Program revenue\"", row.Label)
	}
	if len(row.Cells) != 1 {
		t.Fatalf("cells = %d, want 1", len(row.Cells))
	}
	cell := row.Cells[0]
	if cell.CellType != "dropdown" {
		t.Errorf("cellType = %q, want dropdown", cell.CellType)
	}
	if cell.ID != "0_table0_cell_c1_r1" {
		t.Errorf("id = %q", cell.ID)
	}
	if cell.AriaLabel != "" { // "  " normalized to empty
		t.Errorf("ariaLabel = %q, want empty", cell.AriaLabel)
	}
	if cell.Value != nil {
		t.Errorf("dropdown value should be nil, got %v", *cell.Value)
	}
	if len(cell.Options) != 3 {
		t.Fatalf("options = %d, want 3 (incl. leading blank; duplicate listContainer must be ignored)", len(cell.Options))
	}
	if cell.Options[0].Text != "" || !cell.Options[0].Correct {
		t.Errorf("option0 = %#v, want empty text + correct (k on blank, captured losslessly)", cell.Options[0])
	}
	if cell.Options[1].Text != "Program revenue" || cell.Options[1].Correct {
		t.Errorf("option1 = %#v", cell.Options[1])
	}
	for _, o := range cell.Options {
		if o.Text == "DUPLICATE" {
			t.Fatal("leaked duplicate option from div.listContainer")
		}
	}
}
