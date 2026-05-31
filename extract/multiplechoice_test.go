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
