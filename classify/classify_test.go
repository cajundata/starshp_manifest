package classify

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/cajundata/starshp_manifest/types"
)

func doc(t *testing.T, html string) *goquery.Document {
	t.Helper()
	d, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		html string
		want types.QuestionType
	}{
		{"worksheet", `<div class="worksheet-wrap"><div class="worksheet__main"></div></div>`, types.TypeWorksheet},
		{"mc", `<div class="answers-wrap"><ul class="answers--mc"><li class="answer-wrap--mc"></li></ul></div>`, types.TypeMultipleChoice},
		{"matching", `<div class="dlc_question awd-probe-type-matching"></div>`, types.TypeMatching},
		{"fitb", `<div class="dlc_question awd-probe-type-fill_in_the_blank"></div>`, types.TypeFillInTheBlank},
		{"tf", `<div class="dlc_question awd-probe-type-true_false"></div>`, types.TypeTrueFalse},
		{"ms", `<div class="dlc_question awd-probe-type-multiple_select"></div>`, types.TypeMultipleSelect},
		{"unknown", `<div class="something-else"></div>`, types.TypeUnknown},
		{"worksheet-wins-over-mc", `<div class="worksheet-wrap"></div><ul class="answers--mc"></ul>`, types.TypeWorksheet},
	}
	for _, c := range cases {
		if got := Classify(doc(t, c.html)); got != c.want {
			t.Errorf("%s: Classify = %q, want %q", c.name, got, c.want)
		}
	}
}
