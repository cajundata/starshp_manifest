// Package audit reports CSS classes that no extractor handler recognizes,
// as a backlog of potential new handlers. Dev aid, not part of extraction.
package audit

import (
	"io/fs"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// known is the set of CSS classes the extractor already understands.
var known = map[string]struct{}{
	"worksheet-wrap": {}, "worksheet__main": {}, "answers--mc": {}, "answer-wrap--mc": {},
	"answer__label--mc": {}, "question": {}, "dlc_question": {}, "captured-iframe-content": {},
	"captured-tab-panel": {}, "codex-captured-choices": {}, "jSheetParent": {}, "jSheet": {},
	"dropDownList": {}, "responseCell": {}, "colHeader": {}, "rowHeader": {}, "td-readOnly": {},
	"answer_holder": {}, "list_content": {}, "match-row": {}, "match-prompt-label": {},
	"choices-container": {}, "choice-item-wrapper": {}, "choice-item": {}, "content": {},
	"fitb-input": {}, "prompt": {}, "choiceText": {},
}

// Scan walks all *.html files in fsys and returns counts of unknown CSS classes.
func Scan(fsys fs.FS) (map[string]int, error) {
	counts := map[string]int{}
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".html") {
			return nil
		}
		f, err := fsys.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		doc, err := goquery.NewDocumentFromReader(f)
		if err != nil {
			return nil // skip unparseable file, don't abort the scan
		}
		doc.Find("[class]").Each(func(_ int, s *goquery.Selection) {
			for _, c := range strings.Fields(s.AttrOr("class", "")) {
				if _, ok := known[c]; ok {
					continue
				}
				counts[c]++
			}
		})
		return nil
	})
	return counts, err
}
