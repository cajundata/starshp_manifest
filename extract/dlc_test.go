package extract

import (
	"testing"

	"github.com/cajundata/starshp_manifest/types"
)

func hasWarn(warns []string, w string) bool {
	for _, x := range warns {
		if x == w {
			return true
		}
	}
	return false
}

func TestExtractMatching(t *testing.T) {
	html := `<div class="dlc_question awd-probe-type-matching">
	  <div class="match-row"><div class="match-prompt-label"><span class="content">Term 1</span></div></div>
	  <div class="choices-container"><div class="choice-item-wrapper"><div class="choice-item"><span class="content">Definition 1</span></div></div></div>
	</div>`
	body, _, warns := extractMatching(mustDoc(t, html))
	m := body.(types.MatchingBody)
	if len(m.Terms) != 1 || m.Terms[0] != "Term 1" {
		t.Errorf("terms = %#v", m.Terms)
	}
	if len(m.Choices) != 1 || m.Choices[0] != "Definition 1" {
		t.Errorf("choices = %#v", m.Choices)
	}
	if !hasWarn(warns, types.WarnDLCUnverified) {
		t.Errorf("missing dlc-handler-unverified warning")
	}
}

func TestExtractFillInTheBlank(t *testing.T) {
	html := `<div class="dlc_question awd-probe-type-fill_in_the_blank"><div class="prompt">The capital of <span class="fitb-input"></span> is <span class="fitb-input"></span>.</div></div>`
	body, _, warns := extractFillInTheBlank(mustDoc(t, html))
	f := body.(types.FillInTheBlankBody)
	if f.PromptWithBlanks != "The capital of [BLANK 1] is [BLANK 2] ." {
		t.Errorf("prompt = %q", f.PromptWithBlanks)
	}
	if !hasWarn(warns, types.WarnDLCUnverified) {
		t.Errorf("missing dlc-handler-unverified warning")
	}
}

func TestExtractTrueFalse(t *testing.T) {
	html := `<div class="dlc_question awd-probe-type-true_false"><div class="prompt">The sky is green.</div><div class="choiceText">True</div><div class="choiceText">False</div></div>`
	body, _, warns := extractTrueFalse(mustDoc(t, html))
	tf := body.(types.TrueFalseBody)
	if tf.Stem != "The sky is green." || len(tf.Choices) != 2 {
		t.Errorf("tf = %#v", tf)
	}
	if !hasWarn(warns, types.WarnDLCUnverified) {
		t.Errorf("missing warning")
	}
}

func TestExtractMultipleSelect(t *testing.T) {
	html := `<div class="dlc_question awd-probe-type-multiple_select"><div class="prompt">Pick all.</div><div class="choiceText">X</div><div class="choiceText">Y</div></div>`
	body, _, warns := extractMultipleSelect(mustDoc(t, html))
	ms := body.(types.MultipleSelectBody)
	if ms.Stem != "Pick all." || len(ms.Choices) != 2 || ms.Choices[1].Index != 1 {
		t.Errorf("ms = %#v", ms)
	}
	if ms.CorrectIndices != nil {
		t.Errorf("correctIndices should be nil")
	}
	if !hasWarn(warns, types.WarnDLCUnverified) {
		t.Errorf("missing warning")
	}
}
