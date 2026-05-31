package extract

import (
	"testing"

	"github.com/cajundata/starshp_manifest/types"
)

const wsCapturedPanels = `
<div class="worksheet-wrap">
  <div class="captured-iframe-content">
    <div class="worksheet__main">
      <p>Carmel County scenario text.</p>
      <h3>Required</h3>
      <ol><li>First requirement</li><li>Second requirement</li></ol>
    </div>
  </div>
  <div id="codex-captured-tab-panels" data-captured-tab-count="2">
    <section class="captured-tab-panel" data-tab-label="Required A">
      <div class="jSheetParent"><table class="jSheet"><tbody>
        <tr><td class="rowHeader td-readOnly" id="0_table0_cell_c0_r0">Row label A</td>
            <td class="dropDownList responseCell" id="0_table0_cell_c1_r0" dropdowntype="dropDown">
              <div class="codex-captured-choices"><ul><li role="option"><span class="answer_holder"></span><a class="list_content">Yes</a></li></ul></div>
            </td></tr>
      </tbody></table></div>
    </section>
    <section class="captured-tab-panel" data-tab-label="Required B">
      <div class="jSheetParent"><table class="jSheet"><tbody>
        <tr><td class="rowHeader td-readOnly" id="0_table0_cell_c0_r0">Row label B</td>
            <td class="responseCell" id="0_table0_cell_c1_r0"></td></tr>
      </tbody></table></div>
    </section>
  </div>
</div>`

const wsNoPanels = `
<div class="worksheet-wrap">
  <div class="captured-iframe-content">
    <div class="worksheet__main">
      <p>Single tab scenario.</p>
      <h3>Required</h3>
      <ol><li>Only requirement</li></ol>
    </div>
    <div class="jSheetParent"><table class="jSheet"><tbody>
      <tr><td class="rowHeader td-readOnly" id="0_table0_cell_c0_r0">Account</td>
          <td class="dropDownList responseCell" id="0_table0_cell_c1_r0" dropdowntype="dropDown"></td></tr>
    </tbody></table></div>
  </div>
</div>`

func TestExtractWorksheetCapturedPanels(t *testing.T) {
	body, title, warns := extractWorksheet(mustDoc(t, wsCapturedPanels))
	ws := body.(types.WorksheetBody)

	if ws.Scenario != "Carmel County scenario text." {
		t.Errorf("scenario = %q", ws.Scenario)
	}
	if len(ws.Required) != 2 || ws.Required[1] != "Second requirement" {
		t.Errorf("required = %#v", ws.Required)
	}
	if len(ws.Tabs) != 2 {
		t.Fatalf("tabs = %d, want 2 (one per captured panel)", len(ws.Tabs))
	}
	if ws.Tabs[0].Label == nil || *ws.Tabs[0].Label != "Required A" {
		t.Errorf("tab0 label = %v", ws.Tabs[0].Label)
	}
	if len(ws.Tabs[0].Tables) != 1 || len(ws.Tabs[0].Tables[0].Rows) != 1 {
		t.Fatalf("tab0 tables/rows wrong: %#v", ws.Tabs[0].Tables)
	}
	if ws.Tabs[0].Tables[0].Rows[0].Cells[0].Options[0].Text != "Yes" {
		t.Errorf("dropdown option not parsed in panel")
	}
	if title != nil {
		t.Errorf("worksheet title should be nil here, got %v", *title)
	}
	// second panel's dropdown cell has no captured choices -> warning
	hasWarn := false
	for _, w := range warns {
		if w == types.WarnDropdownNotCaptured {
			hasWarn = true
		}
	}
	if !hasWarn {
		t.Errorf("expected %q warning for empty dropdown, got %v", types.WarnDropdownNotCaptured, warns)
	}
}

func TestExtractWorksheetNoPanels(t *testing.T) {
	body, _, _ := extractWorksheet(mustDoc(t, wsNoPanels))
	ws := body.(types.WorksheetBody)
	if len(ws.Tabs) != 1 {
		t.Fatalf("tabs = %d, want 1", len(ws.Tabs))
	}
	if ws.Tabs[0].Label != nil {
		t.Errorf("single-tab label should be nil, got %v", *ws.Tabs[0].Label)
	}
	if len(ws.Tabs[0].Tables) != 1 {
		t.Errorf("tables = %d, want 1", len(ws.Tabs[0].Tables))
	}
}
