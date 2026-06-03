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
	tbl := ParseTable(parentSel(t, sample), nil)

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

// Newer Connect captures externalize dropdown choices into a shared
// #codex-captured-choices-library, keyed by data-dropdownid; the cell only
// carries a dropdownid reference and (at most) a live, empty listContainer.
// Many cells share one dropdownid, so one library entry feeds them all.
const externalLibrary = `
<body>
  <div class="jSheetParent">
    <table class="jSheet">
      <tbody>
        <tr>
          <td class="colHeader td-readOnly" id="0_table0_cell_c0_r0"></td>
          <td class="colHeader td-readOnly" id="0_table0_cell_c1_r0">General Journal</td>
        </tr>
        <tr>
          <td class="rowHeader td-readOnly" id="0_table0_cell_c0_r1">1</td>
          <td class="dropDownList responseCell" id="0_table0_cell_c1_r1" dropdownid="4" dropdowntype="dropDown" aria-label="General Journal blank">
            <div class="listContainer"><ul id="listbox-id"><li role="option"><a class="list_content">LIVE DUP</a></li></ul></div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
  <div id="codex-captured-choices-library">
    <div class="codex-captured-choices" data-dropdownid="4">
      <ul role="listbox">
        <li role="option"><span class="answer_holder k"></span><a class="list_content"></a></li>
        <li role="option"><span class="answer_holder"></span><a class="list_content">Cash</a></li>
        <li role="option"><span class="answer_holder"></span><a class="list_content">Vouchers Payable</a></li>
      </ul>
    </div>
    <div class="codex-captured-choices" data-dropdownid="99">
      <ul role="listbox"><li role="option"><a class="list_content">WRONG LIBRARY ENTRY</a></li></ul>
    </div>
  </div>
</body>`

func TestParseTableExternalChoicesLibrary(t *testing.T) {
	d, err := goquery.NewDocumentFromReader(strings.NewReader(externalLibrary))
	if err != nil {
		t.Fatal(err)
	}
	tbl := ParseTable(d.Find(".jSheetParent").First(), d.Selection)

	if len(tbl.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(tbl.Rows))
	}
	cells := tbl.Rows[0].Cells
	if len(cells) != 1 {
		t.Fatalf("cells = %d, want 1", len(cells))
	}
	cell := cells[0]
	if cell.CellType != "dropdown" {
		t.Fatalf("cellType = %q, want dropdown", cell.CellType)
	}
	if len(cell.Options) != 3 {
		t.Fatalf("options = %d, want 3 (resolved from external library by dropdownid)", len(cell.Options))
	}
	if cell.Options[0].Text != "" || !cell.Options[0].Correct {
		t.Errorf("option0 = %#v, want empty+correct (k on blank, captured losslessly)", cell.Options[0])
	}
	if cell.Options[1].Text != "Cash" || cell.Options[2].Text != "Vouchers Payable" {
		t.Errorf("options = %#v, want [_, Cash, Vouchers Payable]", cell.Options)
	}
	for _, o := range cell.Options {
		if o.Text == "LIVE DUP" {
			t.Fatal("leaked live listContainer option into captured choices")
		}
		if o.Text == "WRONG LIBRARY ENTRY" {
			t.Fatal("matched the wrong library entry (data-dropdownid mismatch)")
		}
	}
}

const sampleInputFormula = `
<div class="jSheetParent">
  <table class="jSheet">
    <tbody>
      <tr>
        <td class="colHeader td-readOnly" id="0_table0_cell_c0_r0"></td>
        <td class="colHeader td-readOnly" id="0_table0_cell_c1_r0">Sales</td>
        <td class="colHeader td-readOnly" id="0_table0_cell_c2_r0">Tax</td>
      </tr>
      <tr>
        <td class="rowHeader td-readOnly" id="0_table0_cell_c0_r1">Q1</td>
        <td class="responseCell" id="0_table0_cell_c1_r1">42</td>
        <td class="responseCell" id="0_table0_cell_c2_r1" formula="=A1+B1">100</td>
      </tr>
      <tr>
        <td class="rowHeader td-readOnly" id="0_table0_cell_c0_r2">Q2</td>
        <td class="responseCell" id="0_table0_cell_c1_r2"></td>
        <td class="responseCell" id="0_table0_cell_c2_r2"></td>
      </tr>
    </tbody>
  </table>
</div>`

func TestParseTableInputFormulaAndMultiColumn(t *testing.T) {
	tbl := ParseTable(parentSel(t, sampleInputFormula), nil)

	// --- headers (3 columns) ---
	if len(tbl.Headers) != 3 {
		t.Fatalf("headers len = %d, want 3", len(tbl.Headers))
	}
	if tbl.Headers[0] != "" || tbl.Headers[1] != "Sales" || tbl.Headers[2] != "Tax" {
		t.Errorf("headers = %#v, want [\"\" \"Sales\" \"Tax\"]", tbl.Headers)
	}

	// --- rows ---
	if len(tbl.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(tbl.Rows))
	}

	// === Row 0 (r1): label + input-with-value + formula ===
	row0 := tbl.Rows[0]
	if row0.Label != "Q1" {
		t.Errorf("row0.Label = %q, want \"Q1\"", row0.Label)
	}
	if len(row0.Cells) != 2 {
		t.Fatalf("row0 cells = %d, want 2", len(row0.Cells))
	}

	// cell at c1 (column order: c1 before c2)
	inputCell := row0.Cells[0]
	if inputCell.ID != "0_table0_cell_c1_r1" {
		t.Errorf("inputCell.ID = %q, want 0_table0_cell_c1_r1", inputCell.ID)
	}
	if inputCell.CellType != "input" {
		t.Errorf("inputCell.CellType = %q, want \"input\"", inputCell.CellType)
	}
	if inputCell.Value == nil {
		t.Fatal("inputCell.Value = nil, want \"42\"")
	}
	if *inputCell.Value != "42" {
		t.Errorf("inputCell.Value = %q, want \"42\"", *inputCell.Value)
	}
	if inputCell.Formula != nil {
		t.Errorf("inputCell.Formula should be nil, got %q", *inputCell.Formula)
	}

	// cell at c2: formula
	formulaCell := row0.Cells[1]
	if formulaCell.ID != "0_table0_cell_c2_r1" {
		t.Errorf("formulaCell.ID = %q, want 0_table0_cell_c2_r1", formulaCell.ID)
	}
	if formulaCell.CellType != "formula" {
		t.Errorf("formulaCell.CellType = %q, want \"formula\"", formulaCell.CellType)
	}
	if formulaCell.Formula == nil {
		t.Fatal("formulaCell.Formula = nil, want \"=A1+B1\"")
	}
	if *formulaCell.Formula != "=A1+B1" {
		t.Errorf("formulaCell.Formula = %q, want \"=A1+B1\"", *formulaCell.Formula)
	}
	if formulaCell.Value == nil {
		t.Fatal("formulaCell.Value = nil, want \"100\"")
	}
	if *formulaCell.Value != "100" {
		t.Errorf("formulaCell.Value = %q, want \"100\"", *formulaCell.Value)
	}

	// === Row 1 (r2): label + two empty input cells ===
	row1 := tbl.Rows[1]
	if row1.Label != "Q2" {
		t.Errorf("row1.Label = %q, want \"Q2\"", row1.Label)
	}
	if len(row1.Cells) != 2 {
		t.Fatalf("row1 cells = %d, want 2", len(row1.Cells))
	}

	for i, c := range row1.Cells {
		if c.CellType != "input" {
			t.Errorf("row1.Cells[%d].CellType = %q, want \"input\"", i, c.CellType)
		}
		if c.Value != nil {
			t.Errorf("row1.Cells[%d].Value = %q, want nil (empty cell)", i, *c.Value)
		}
	}
}
