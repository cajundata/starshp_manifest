package extract

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/cajundata/starshp_manifest/types"
)

const mcHTML = `
<main>
  <h1 id="question-info-holder" class="t-hidden"><span>Item</span> 1</h1>
  <p class="question"><p>Which entry is required?</p></p>
  <div class="answers-wrap multiple-choice">
    <ul class="answers--mc">
      <li class="answer-wrap--mc"><div class="answer__span--mc"><label class="answer__label--mc"><input type="radio"><p>Option A</p></label></div></li>
      <li class="answer-wrap--mc"><div class="answer__span--mc"><label class="answer__label--mc"><input type="radio"><p>Option B</p></label></div></li>
    </ul>
  </div>
</main>`

func mustDoc(t *testing.T, html string) *goquery.Document {
	t.Helper()
	d, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestExtractMultipleChoice(t *testing.T) {
	body, title, warns := extractMultipleChoice(mustDoc(t, mcHTML))
	mc, ok := body.(types.MultipleChoiceBody)
	if !ok {
		t.Fatalf("body type = %T, want MultipleChoiceBody", body)
	}
	if mc.Stem != "Which entry is required?" {
		t.Errorf("stem = %q", mc.Stem)
	}
	if len(mc.Choices) != 2 || mc.Choices[0].Text != "Option A" || mc.Choices[1].Index != 1 {
		t.Errorf("choices = %#v", mc.Choices)
	}
	if mc.CorrectIndex != nil {
		t.Errorf("correctIndex should be nil")
	}
	if title == nil || *title != "Item 1" {
		t.Errorf("title = %v, want \"Item 1\"", title)
	}
	if len(warns) != 0 {
		t.Errorf("unexpected warnings: %v", warns)
	}
}

func TestExtractMultipleChoiceMissingTitle(t *testing.T) {
	_, title, warns := extractMultipleChoice(mustDoc(t, `<ul class="answers--mc"><li class="answer-wrap--mc"><label class="answer__label--mc"><p>A</p></label></li></ul>`))
	if title != nil {
		t.Errorf("title should be nil when absent, got %v", *title)
	}
	if len(warns) != 1 || warns[0] != types.WarnMissingTitle {
		t.Errorf("warnings = %v, want [missing-title]", warns)
	}
}

// Real Connect HTML nests block elements (<p>, <table>) inside <p class="question">.
// HTML5 forbids that, so the parser auto-closes p.question and hoists the inner
// blocks up as siblings. The stem must gather ALL of them: the prose paragraphs
// joined into Stem, and the embedded table captured as StemTable.
const mcStemTableHTML = `
<main>
  <h1 id="question-info-holder" class="t-hidden"><span>Item</span> 3</h1>
  <div class="question-wrap">
    <p class="question">
      <p>Tinsel Town had the following long-term liabilities at year-end:</p>
      <table>
        <tbody>
          <tr><th scope="row">Revenue bonds</th><td>$ 249,800</td></tr>
          <tr><th scope="row">General obligation bonds</th><td>199,800</td></tr>
        </tbody>
      </table>
      <p>What amount should be recorded as noncurrent liabilities?</p>
    </p>
  </div>
  <div class="answers-wrap multiple-choice">
    <ul class="answers--mc">
      <li class="answer-wrap--mc"><div class="answer__span--mc"><label class="answer__label--mc"><input type="radio"><p>$0</p></label></div></li>
      <li class="answer-wrap--mc"><div class="answer__span--mc"><label class="answer__label--mc"><input type="radio"><p>$199,800</p></label></div></li>
    </ul>
  </div>
</main>`

func TestExtractMultipleChoiceStemTable(t *testing.T) {
	body, _, _ := extractMultipleChoice(mustDoc(t, mcStemTableHTML))
	mc := body.(types.MultipleChoiceBody)

	wantStem := "Tinsel Town had the following long-term liabilities at year-end: What amount should be recorded as noncurrent liabilities?"
	if mc.Stem != wantStem {
		t.Errorf("stem = %q,\nwant %q", mc.Stem, wantStem)
	}

	if mc.StemTable == nil {
		t.Fatalf("stemTable should be populated when the stem contains a table")
	}
	if len(mc.StemTable.Headers) != 0 {
		t.Errorf("headers = %#v, want empty (no column-header row)", mc.StemTable.Headers)
	}
	if len(mc.StemTable.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(mc.StemTable.Rows))
	}
	if r := mc.StemTable.Rows[0]; r.Label != "Revenue bonds" || len(r.Cells) != 1 || r.Cells[0].Value != "$ 249,800" {
		t.Errorf("row[0] = %#v", r)
	}
	if r := mc.StemTable.Rows[1]; r.Label != "General obligation bonds" || len(r.Cells) != 1 || r.Cells[0].Value != "199,800" {
		t.Errorf("row[1] = %#v", r)
	}
}

// A header row (all <th>, no <td>) populates Headers; subsequent rows are data.
const mcStemTableHeadersHTML = `
<main>
  <h1 id="question-info-holder" class="t-hidden"><span>Item</span> 1</h1>
  <div class="question-wrap">
    <p class="question">
      <p>Which row balances?</p>
      <table>
        <thead><tr><th scope="col">Account</th><th scope="col">Debit</th><th scope="col">Credit</th></tr></thead>
        <tbody><tr><th scope="row">Cash</th><td>100</td><td>&nbsp;</td></tr></tbody>
      </table>
    </p>
  </div>
  <div class="answers-wrap multiple-choice">
    <ul class="answers--mc">
      <li class="answer-wrap--mc"><div class="answer__span--mc"><label class="answer__label--mc"><p>Yes</p></label></div></li>
    </ul>
  </div>
</main>`

func TestExtractMultipleChoiceStemTableHeaders(t *testing.T) {
	body, _, _ := extractMultipleChoice(mustDoc(t, mcStemTableHeadersHTML))
	mc := body.(types.MultipleChoiceBody)
	if mc.Stem != "Which row balances?" {
		t.Errorf("stem = %q", mc.Stem)
	}
	if mc.StemTable == nil {
		t.Fatalf("stemTable should be populated")
	}
	wantHeaders := []string{"Account", "Debit", "Credit"}
	if len(mc.StemTable.Headers) != 3 || mc.StemTable.Headers[0] != "Account" || mc.StemTable.Headers[2] != "Credit" {
		t.Errorf("headers = %#v, want %v", mc.StemTable.Headers, wantHeaders)
	}
	if len(mc.StemTable.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(mc.StemTable.Rows))
	}
	if r := mc.StemTable.Rows[0]; r.Label != "Cash" || len(r.Cells) != 2 || r.Cells[0].Value != "100" || r.Cells[1].Value != "" {
		t.Errorf("row[0] = %#v", r)
	}
}

// A stem with no table leaves StemTable nil (omitted from JSON via omitempty).
func TestExtractMultipleChoiceNoStemTable(t *testing.T) {
	body, _, _ := extractMultipleChoice(mustDoc(t, mcHTML))
	mc := body.(types.MultipleChoiceBody)
	if mc.StemTable != nil {
		t.Errorf("stemTable should be nil when no table is present, got %#v", mc.StemTable)
	}
}

// Graded MC snapshots mark the correct option's .answer__span--mc with
// "is-correct" (regardless of whether the student selected it). The second
// choice is the correct one here.
const mcGradedHTML = `
<ul class="answers--mc">
  <li class="answer-wrap--mc"><div class="answer__span--mc is-checked"><label class="answer__label--mc"><input type="radio"><p>Wrong choice</p></label></div></li>
  <li class="answer-wrap--mc"><div class="answer__span--mc is-correct"><label class="answer__label--mc"><input type="radio"><p>Right choice</p><span class="t-hidden">Correct</span><span class="answer--is-correct" data-icon="✓"></span></label></div></li>
  <li class="answer-wrap--mc"><div class="answer__span--mc"><label class="answer__label--mc"><input type="radio"><p>Another wrong</p></label></div></li>
</ul>`

func TestExtractMultipleChoiceGradedCorrectIndex(t *testing.T) {
	body, _, _ := extractMultipleChoice(mustDoc(t, mcGradedHTML))
	mc := body.(types.MultipleChoiceBody)
	if mc.CorrectIndex == nil {
		t.Fatalf("correctIndex should be set for a graded snapshot")
	}
	if *mc.CorrectIndex != 1 {
		t.Errorf("correctIndex = %d, want 1", *mc.CorrectIndex)
	}
	if len(mc.Choices) != 3 {
		t.Errorf("choices = %d, want 3", len(mc.Choices))
	}
}
