# Connect HTML Extractor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `manifest`, a library-first Go tool that parses saved McGraw-Hill Connect assignment HTML into a lossless, discriminated-union JSON schema (per-file envelopes + a top-level manifest index).

**Architecture:** A pure, importable core — `taxonomy` (path→course), `classify` (discriminator), `extract` (per-type extractors over `goquery`), `internal/cells` (jSheet table + dropdown parsing), `manifest` (index builder), `batch` (tree walk with per-file panic isolation + progress hook), `audit` (dev aid) — with `cmd/manifest` as one thin CLI frontend. Extraction is per-file with no shared state. JSON is the canonical source of truth.

**Tech Stack:** Go 1.26, [`goquery`](https://github.com/PuerkitoBio/goquery) (static CSS selectors over fully-rendered snapshots), standard `testing` with table-driven + golden-file tests.

**Spec:** `docs/superpowers/specs/2026-05-31-connect-html-extractor-design.md`

**Module path:** `github.com/cajundata/starshp_manifest`

---

## File Structure

| Path | Responsibility |
| --- | --- |
| `go.mod` / `go.sum` | Module definition + deps |
| `types/schema.go` | Shared contract: `Question` envelope, variant bodies, `Manifest`, warning constants |
| `internal/text/text.go` | `Normalize` — collapse/trim HTML whitespace (shared by all extractors) |
| `taxonomy/taxonomy.go` | `Derive(path)` → courseCode/courseName/module + `course-not-derivable` warning |
| `classify/classify.go` | `Classify(doc)` → `types.QuestionType` (the discriminator) |
| `internal/cells/cells.go` | Parse a `div.jSheetParent` → `types.Table`; cell classification; dropdown options |
| `extract/multiplechoice.go` | MC extractor |
| `extract/worksheet.go` | Worksheet extractor (table-source rule, tabs) |
| `extract/dlc.go` | matching / fillInTheBlank / trueFalse / multipleSelect (ported, unverified) |
| `extract/extract.go` | `ExtractHTML(io.Reader, Source)` dispatcher: parse → classify → build envelope |
| `manifest/manifest.go` | `Build(questions, root)` → `types.Manifest` |
| `batch/batch.go` | `ExtractTree(fs.FS, Options)` → `Result`; ordering, panic isolation, progress |
| `audit/audit.go` | `Scan(fs.FS)` → unknown CSS class counts (handler backlog) |
| `cmd/manifest/main.go` | Thin CLI: `extract` / `plan` / `audit` verbs |
| `testdata/` | Copied real fixtures + golden JSON |

---

## Task 1: Initialize module and skeleton

**Files:**
- Create: `go.mod`, `doc.go`

- [ ] **Step 1: Initialize the module and add goquery**

Run:
```bash
go mod init github.com/cajundata/starshp_manifest
go get github.com/PuerkitoBio/goquery@latest
```
Expected: `go.mod` created with a `require github.com/PuerkitoBio/goquery` line; `go.sum` populated.

- [ ] **Step 2: Add a package doc file so the module builds clean**

Create `doc.go`:
```go
// Package starshp_manifest is the module root for the manifest extractor,
// which converts saved McGraw-Hill Connect assignment HTML into lossless JSON.
//
// The importable core lives in subpackages: types, taxonomy, classify,
// extract, manifest, batch, and audit. cmd/manifest is the CLI frontend.
package starshp_manifest
```

- [ ] **Step 3: Verify it builds**

Run: `go build ./...`
Expected: no output, exit 0.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum doc.go
git commit -m "chore: initialize go module and goquery dependency"
```

---

## Task 2: Schema types (the shared contract)

**Files:**
- Create: `types/schema.go`
- Test: `types/schema_test.go`

- [ ] **Step 1: Write the failing test**

Create `types/schema_test.go`:
```go
package types

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestQuestionMarshalsExpectedShape(t *testing.T) {
	code := "acct4421"
	name := "Governmental & Not-for-Profit Accounting"
	mod := "mod02"
	title := "Item 1"
	q := Question{
		SchemaVersion: 1,
		Source:        Source{Path: "acct4421/mod02/x.html", CourseCode: &code, CourseName: &name, Module: &mod},
		Type:          TypeMultipleChoice,
		Title:         &title,
		Capture:       Capture{HasCapturedChoices: false, CapturedTabCount: 0},
		Warnings:      []string{},
		Tags:          []string{},
		Body:          MultipleChoiceBody{Stem: "Q?", Choices: []MCChoice{{Index: 0, Text: "A"}}, CorrectIndex: nil},
	}
	b, err := json.Marshal(q)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	for _, want := range []string{
		`"schemaVersion":1`, `"courseCode":"acct4421"`, `"type":"multipleChoice"`,
		`"tags":[]`, `"warnings":[]`, `"correctIndex":null`, `"hasCapturedChoices":false`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("marshaled JSON missing %q\n got: %s", want, got)
		}
	}
}

func TestEmptySlicesMarshalAsArraysNotNull(t *testing.T) {
	q := Question{SchemaVersion: 1, Warnings: []string{}, Tags: []string{}, Body: UnknownBody{RawClasses: []string{}, Note: "x"}}
	b, _ := json.Marshal(q)
	if strings.Contains(string(b), `"warnings":null`) || strings.Contains(string(b), `"tags":null`) {
		t.Errorf("nil slice leaked as null: %s", b)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./types/`
Expected: FAIL — `undefined: Question` (and related symbols).

- [ ] **Step 3: Write the schema types**

Create `types/schema.go`:
```go
// Package types defines the JSON schema contract shared by every frontend.
package types

// QuestionType is the discriminated-union tag.
type QuestionType string

const (
	TypeMultipleChoice QuestionType = "multipleChoice"
	TypeWorksheet      QuestionType = "worksheet"
	TypeMatching       QuestionType = "matching"
	TypeFillInTheBlank QuestionType = "fillInTheBlank"
	TypeTrueFalse      QuestionType = "trueFalse"
	TypeMultipleSelect QuestionType = "multipleSelect"
	TypeUnknown        QuestionType = "unknown"
)

// Warning codes (stable strings; downstream may switch on them).
const (
	WarnCourseNotDerivable  = "course-not-derivable"
	WarnDropdownNotCaptured = "dropdown-options-not-captured"
	WarnDLCUnverified       = "dlc-handler-unverified"
	WarnMissingTitle        = "missing-title"
	WarnParseFailed         = "parse-failed"
)

const SchemaVersion = 1

// Source is deterministic provenance derived from the input path.
type Source struct {
	Path       string  `json:"path"`
	CourseCode *string `json:"courseCode"`
	CourseName *string `json:"courseName"`
	Module     *string `json:"module"`
}

// Capture records what the HTML snapshot actually contained.
type Capture struct {
	HasCapturedChoices bool `json:"hasCapturedChoices"`
	CapturedTabCount   int  `json:"capturedTabCount"`
}

// Question is the per-file envelope wrapping a type-specific Body.
type Question struct {
	SchemaVersion int          `json:"schemaVersion"`
	Source        Source       `json:"source"`
	Type          QuestionType `json:"type"`
	Title         *string      `json:"title"`
	Capture       Capture      `json:"capture"`
	Warnings      []string     `json:"warnings"`
	Tags          []string     `json:"tags"`
	Body          any          `json:"body"`
}

// --- Variant bodies ---

type MCChoice struct {
	Index int    `json:"index"`
	Text  string `json:"text"`
}

type MultipleChoiceBody struct {
	Stem         string     `json:"stem"`
	Choices      []MCChoice `json:"choices"`
	CorrectIndex *int       `json:"correctIndex"`
}

type DropdownOption struct {
	Index   int    `json:"index"`
	Text    string `json:"text"`
	Correct bool   `json:"correct"`
}

type Cell struct {
	ID        string           `json:"id"`
	CellType  string           `json:"cellType"` // input | dropdown | readonly | formula
	AriaLabel string           `json:"ariaLabel"`
	Formula   *string          `json:"formula"`
	Value     *string          `json:"value"`
	Options   []DropdownOption `json:"options"`
}

type Row struct {
	Label string `json:"label"`
	Cells []Cell `json:"cells"`
}

type Table struct {
	Headers []string `json:"headers"`
	Rows    []Row    `json:"rows"`
}

type Tab struct {
	Label  *string `json:"label"`
	Tables []Table `json:"tables"`
}

type WorksheetBody struct {
	Scenario string   `json:"scenario"`
	Required []string `json:"required"`
	Tabs     []Tab    `json:"tabs"`
}

type MatchingBody struct {
	Terms   []string `json:"terms"`
	Choices []string `json:"choices"`
}

type FillInTheBlankBody struct {
	PromptWithBlanks string `json:"promptWithBlanks"`
}

type TrueFalseBody struct {
	Stem         string   `json:"stem"`
	Choices      []string `json:"choices"`
	CorrectIndex *int     `json:"correctIndex"`
}

type MultipleSelectBody struct {
	Stem           string     `json:"stem"`
	Choices        []MCChoice `json:"choices"`
	CorrectIndices []int      `json:"correctIndices"`
}

type UnknownBody struct {
	RawClasses []string `json:"rawClasses"`
	Note       string   `json:"note"`
}

// --- Manifest index ---

type ManifestEntry struct {
	Path               string       `json:"path"`
	CourseCode         *string      `json:"courseCode"`
	Module             *string      `json:"module"`
	Type               QuestionType `json:"type"`
	Title              *string      `json:"title"`
	HasCapturedChoices bool         `json:"hasCapturedChoices"`
	Warnings           int          `json:"warnings"`
}

type Manifest struct {
	SchemaVersion int             `json:"schemaVersion"`
	GeneratedFrom string          `json:"generatedFrom"`
	Count         int             `json:"count"`
	Questions     []ManifestEntry `json:"questions"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./types/`
Expected: PASS (`ok ... types`).

- [ ] **Step 5: Commit**

```bash
git add types/
git commit -m "feat(types): add JSON schema contract structs"
```

---

## Task 3: Text normalization utility

**Files:**
- Create: `internal/text/text.go`
- Test: `internal/text/text_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/text/text_test.go`:
```go
package text

import "testing"

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"  hello   world  ": "hello world",
		"line1\n\n  line2":  "line1 line2",
		"\t spaced \t":      "spaced",
		"Item 1":       "Item 1", // non-breaking space collapses too
		"":                  "",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/text/`
Expected: FAIL — `undefined: Normalize`.

- [ ] **Step 3: Implement Normalize**

Create `internal/text/text.go`:
```go
// Package text provides whitespace normalization for extracted HTML text.
package text

import "strings"

// Normalize collapses all runs of Unicode whitespace (including the
// non-breaking space U+00A0) to a single ASCII space and trims the ends.
func Normalize(s string) string {
	s = strings.ReplaceAll(s, " ", " ")
	return strings.Join(strings.Fields(s), " ")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/text/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/text/
git commit -m "feat(text): add whitespace Normalize helper"
```

---

## Task 4: Taxonomy derivation

**Files:**
- Create: `taxonomy/taxonomy.go`
- Test: `taxonomy/taxonomy_test.go`

- [ ] **Step 1: Write the failing test**

Create `taxonomy/taxonomy_test.go`:
```go
package taxonomy

import "testing"

func strOrNil(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

func TestDerive(t *testing.T) {
	cases := []struct {
		path     string
		code     string
		name     string
		module   string
		wantWarn bool
	}{
		{"acct4421_gov-nonprof-acct/mod02/mod02_001.html", "acct4421", "Governmental & Not-for-Profit Accounting", "mod02", false},
		{"acct3221_tax-acct-01/mod03/hw04/page.html", "acct3221", "Tax Accounting", "mod03", false},
		{"acct9999_unknown-course/mod01/x.html", "acct9999", "<nil>", "mod01", false}, // unmapped slug -> nil name, no warning
		{"001.html", "<nil>", "<nil>", "<nil>", true},                                  // flattened fixtures -> all nil + warning
	}
	for _, c := range cases {
		tax, warn := Derive(c.path)
		if strOrNil(tax.CourseCode) != c.code {
			t.Errorf("%s: courseCode = %s, want %s", c.path, strOrNil(tax.CourseCode), c.code)
		}
		if strOrNil(tax.CourseName) != c.name {
			t.Errorf("%s: courseName = %s, want %s", c.path, strOrNil(tax.CourseName), c.name)
		}
		if strOrNil(tax.Module) != c.module {
			t.Errorf("%s: module = %s, want %s", c.path, strOrNil(tax.Module), c.module)
		}
		if (warn != "") != c.wantWarn {
			t.Errorf("%s: warn=%q wantWarn=%v", c.path, warn, c.wantWarn)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./taxonomy/`
Expected: FAIL — `undefined: Derive`.

- [ ] **Step 3: Implement Derive**

Create `taxonomy/taxonomy.go`:
```go
// Package taxonomy derives deterministic course/module metadata from a file path.
package taxonomy

import (
	"regexp"
	"strings"

	"github.com/cajundata/starshp_manifest/types"
)

var (
	// NOTE: Go's RE2 \b is a \w/\W boundary, so `\bacct(\d{4})\b` fails when the
	// digits are immediately followed by `_` (e.g. acct4421_gov-...): both `1` and
	// `_` are word chars, so there is no boundary. Use explicit non-alpha/non-digit
	// guards instead.
	courseRe = regexp.MustCompile(`(?i)(?:^|[^a-zA-Z])acct[_-]?(\d{4})(?:[^0-9]|$)`)
	moduleRe = regexp.MustCompile(`(?i)\b(mod\d+)\b`)
)

// courseNames maps a normalized course code to a display name. Unmapped codes
// yield a nil CourseName (graceful: a new course still gets a code, just no name).
var courseNames = map[string]string{
	"acct4421": "Governmental & Not-for-Profit Accounting",
	"acct3221": "Tax Accounting",
	"acct3020": "Intermediate Accounting I",
}

// Derive returns taxonomy parsed from path. The second return value is a warning
// code ("" when none). When no acct#### segment exists, all fields are nil and
// the WarnCourseNotDerivable code is returned.
func Derive(path string) (types.Source, string) {
	src := types.Source{Path: path}
	if m := moduleRe.FindStringSubmatch(path); m != nil {
		mod := strings.ToLower(m[1])
		src.Module = &mod
	}
	cm := courseRe.FindStringSubmatch(path)
	if cm == nil {
		return src, types.WarnCourseNotDerivable
	}
	code := "acct" + cm[1]
	src.CourseCode = &code
	if name, ok := courseNames[code]; ok {
		n := name
		src.CourseName = &n
	}
	return src, ""
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./taxonomy/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add taxonomy/
git commit -m "feat(taxonomy): derive course/module from input path"
```

---

## Task 5: Classifier (the discriminator)

**Files:**
- Create: `classify/classify.go`
- Test: `classify/classify_test.go`

- [ ] **Step 1: Write the failing test**

Create `classify/classify_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./classify/`
Expected: FAIL — `undefined: Classify`.

- [ ] **Step 3: Implement Classify**

Create `classify/classify.go`:
```go
// Package classify determines a Connect page's question type (the discriminator).
package classify

import (
	"regexp"

	"github.com/PuerkitoBio/goquery"
	"github.com/cajundata/starshp_manifest/types"
)

var probeRe = regexp.MustCompile(`awd-probe-type-([a-z_]+)`)

// Classify inspects the document in real-data priority order:
//  1. .worksheet-wrap        -> worksheet
//  2. .dlc_question          -> branch on awd-probe-type-*
//  3. ul.answers--mc         -> multipleChoice
//  4. else                   -> unknown
func Classify(doc *goquery.Document) types.QuestionType {
	if doc.Find(".worksheet-wrap").Length() > 0 {
		return types.TypeWorksheet
	}
	if dlc := doc.Find(".dlc_question"); dlc.Length() > 0 {
		if t := probeType(dlc); t != types.TypeUnknown {
			return t
		}
	}
	if doc.Find("ul.answers--mc").Length() > 0 {
		return types.TypeMultipleChoice
	}
	return types.TypeUnknown
}

func probeType(dlc *goquery.Selection) types.QuestionType {
	class, _ := dlc.First().Attr("class")
	m := probeRe.FindStringSubmatch(class)
	if m == nil {
		return types.TypeUnknown
	}
	switch m[1] {
	case "matching":
		return types.TypeMatching
	case "fill_in_the_blank":
		return types.TypeFillInTheBlank
	case "true_false":
		return types.TypeTrueFalse
	case "multiple_select":
		return types.TypeMultipleSelect
	case "multiple_choice":
		return types.TypeMultipleChoice
	default:
		return types.TypeUnknown
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./classify/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add classify/
git commit -m "feat(classify): add page-type discriminator"
```

---

## Task 6: jSheet table + dropdown parsing

**Files:**
- Create: `internal/cells/cells.go`
- Test: `internal/cells/cells_test.go`

This parses one `div.jSheetParent` into a `types.Table`. Real markup facts (verified in `005.html`): the data table is `table.jSheet`; cells carry ids like `0_table0_cell_c1_r0`; row labels are `td.rowHeader.td-readOnly`; column headers are `td.colHeader.td-readOnly`; dropdown cells are `td.dropDownList[dropdowntype=dropDown]` and contain `div.codex-captured-choices > ul > li[role=option]` with text in `a.list_content` and the correct flag as class `k` on `span.answer_holder`. A duplicate live listbox exists in `div.listContainer` — we avoid it by scoping option parsing to **inside the cell**.

- [ ] **Step 1: Write the failing test**

Create `internal/cells/cells_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cells/`
Expected: FAIL — `undefined: ParseTable`.

- [ ] **Step 3: Implement the parser**

Create `internal/cells/cells.go`:
```go
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

// ParseTable converts one div.jSheetParent into a types.Table.
func ParseTable(parent *goquery.Selection) types.Table {
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
			row.Cells = append(row.Cells, buildCell(pc))
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

func buildCell(pc parsedCell) types.Cell {
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
		c.Options = parseOptions(pc.sel)
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

// parseOptions reads ONLY the cell's own div.codex-captured-choices, ignoring any
// duplicate live listbox (div.listContainer) elsewhere in the parent.
func parseOptions(cell *goquery.Selection) []types.DropdownOption {
	opts := []types.DropdownOption{}
	cell.Find(".codex-captured-choices li[role='option']").Each(func(i int, li *goquery.Selection) {
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cells/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cells/
git commit -m "feat(cells): parse jSheet tables and captured dropdown options"
```

---

## Task 7: Multiple-choice extractor

**Files:**
- Create: `extract/multiplechoice.go`
- Test: `extract/multiplechoice_test.go`

Verified markup (`001.html`): stem in `p.question`; choices in `li.answer-wrap--mc` with text at `.answer__label--mc p`; title fallback `#question-info-holder` → "Item 1". `correctIndex` is never recoverable (always nil).

- [ ] **Step 1: Write the failing test**

Create `extract/multiplechoice_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./extract/ -run MultipleChoice`
Expected: FAIL — `undefined: extractMultipleChoice`.

- [ ] **Step 3: Implement the extractor**

Create `extract/multiplechoice.go`:
```go
package extract

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/cajundata/starshp_manifest/internal/text"
	"github.com/cajundata/starshp_manifest/types"
)

// extractMultipleChoice returns (body, title, warnings).
func extractMultipleChoice(doc *goquery.Document) (any, *string, []string) {
	warns := []string{}

	// The stem is in p.question; when the source HTML wraps an inner <p> (and/or a
	// table), the HTML5 parser hoists those out as siblings, leaving p.question
	// empty. Fall back to the next sibling <p> in that case (this also keeps an
	// options-table out of the stem text).
	stemSel := doc.Find("p.question").First()
	stem := text.Normalize(stemSel.Text())
	if stem == "" {
		stem = text.Normalize(stemSel.Next().Text())
	}

	body := types.MultipleChoiceBody{
		Stem:         stem,
		Choices:      []types.MCChoice{},
		CorrectIndex: nil, // not recoverable from snapshots
	}

	doc.Find("li.answer-wrap--mc").Each(func(i int, li *goquery.Selection) {
		t := text.Normalize(li.Find(".answer__label--mc p").First().Text())
		if t == "" {
			t = text.Normalize(li.Text())
		}
		body.Choices = append(body.Choices, types.MCChoice{Index: i, Text: t})
	})

	title := titleOrWarn(doc, &warns)
	return body, title, warns
}

// titleOrWarn resolves the MC title from #question-info-holder ("Item N"),
// appending WarnMissingTitle when absent.
func titleOrWarn(doc *goquery.Document, warns *[]string) *string {
	t := text.Normalize(doc.Find("#question-info-holder").First().Text())
	if t == "" {
		*warns = append(*warns, types.WarnMissingTitle)
		return nil
	}
	return &t
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./extract/ -run MultipleChoice`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add extract/multiplechoice.go extract/multiplechoice_test.go
git commit -m "feat(extract): multiple-choice extractor"
```

---

## Task 8: Worksheet extractor

**Files:**
- Create: `extract/worksheet.go`
- Test: `extract/worksheet_test.go`

Table-source rule: if `#codex-captured-tab-panels` exists, one `Tab` per `section.captured-tab-panel` (label from `data-tab-label`), tables from each panel's `div.jSheetParent`. Otherwise one tab (label nil), tables from `div.captured-iframe-content`. Scenario: `.worksheet__main > p` (first). Required: `<li>`s of the first `<ol>` in `.worksheet__main`.

- [ ] **Step 1: Write the failing test**

Create `extract/worksheet_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./extract/ -run Worksheet`
Expected: FAIL — `undefined: extractWorksheet`.

- [ ] **Step 3: Implement the extractor**

Create `extract/worksheet.go`:
```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./extract/ -run Worksheet`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add extract/worksheet.go extract/worksheet_test.go
git commit -m "feat(extract): worksheet extractor with table-source rule"
```

---

## Task 9: DLC family extractors (ported, unverified)

**Files:**
- Create: `extract/dlc.go`
- Test: `extract/dlc_test.go`

Ported from the extension's documented selectors; **no real examples exist**, so every DLC body carries `WarnDLCUnverified`.

- [ ] **Step 1: Write the failing test**

Create `extract/dlc_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./extract/ -run "Matching|FillInTheBlank|TrueFalse|MultipleSelect"`
Expected: FAIL — `undefined: extractMatching` (etc.).

- [ ] **Step 3: Implement the DLC extractors**

Create `extract/dlc.go`:
```go
package extract

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/cajundata/starshp_manifest/internal/text"
	"github.com/cajundata/starshp_manifest/types"
)

func extractMatching(doc *goquery.Document) (any, *string, []string) {
	body := types.MatchingBody{Terms: []string{}, Choices: []string{}}
	doc.Find(".match-row .match-prompt-label .content").Each(func(_ int, s *goquery.Selection) {
		body.Terms = append(body.Terms, text.Normalize(s.Text()))
	})
	doc.Find(".choices-container .choice-item-wrapper .choice-item .content").Each(func(_ int, s *goquery.Selection) {
		body.Choices = append(body.Choices, text.Normalize(s.Text()))
	})
	return body, nil, []string{types.WarnDLCUnverified}
}

func extractFillInTheBlank(doc *goquery.Document) (any, *string, []string) {
	// Replace each .fitb-input with a [BLANK n] marker, then normalize.
	prompt := doc.Find(".prompt").First()
	n := 0
	prompt.Find(".fitb-input").Each(func(_ int, s *goquery.Selection) {
		n++
		s.ReplaceWithHtml(" [BLANK " + itoa(n) + "] ")
	})
	body := types.FillInTheBlankBody{PromptWithBlanks: text.Normalize(prompt.Text())}
	return body, nil, []string{types.WarnDLCUnverified}
}

func extractTrueFalse(doc *goquery.Document) (any, *string, []string) {
	body := types.TrueFalseBody{
		Stem:         text.Normalize(doc.Find(".prompt").First().Text()),
		Choices:      choiceTexts(doc),
		CorrectIndex: nil,
	}
	return body, nil, []string{types.WarnDLCUnverified}
}

func extractMultipleSelect(doc *goquery.Document) (any, *string, []string) {
	body := types.MultipleSelectBody{
		Stem:           text.Normalize(doc.Find(".prompt").First().Text()),
		Choices:        []types.MCChoice{},
		CorrectIndices: nil,
	}
	for i, t := range choiceTexts(doc) {
		body.Choices = append(body.Choices, types.MCChoice{Index: i, Text: t})
	}
	return body, nil, []string{types.WarnDLCUnverified}
}

func choiceTexts(doc *goquery.Document) []string {
	out := []string{}
	doc.Find(".choiceText").Each(func(_ int, s *goquery.Selection) {
		out = append(out, text.Normalize(s.Text()))
	})
	return out
}

func itoa(n int) string {
	// small local helper to avoid importing strconv in this file
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
```

> Note: `ReplaceWithHtml` is a goquery `*Selection` method; the `[BLANK n]` text is inserted before `prompt.Text()` is read, so ordering is preserved.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./extract/ -run "Matching|FillInTheBlank|TrueFalse|MultipleSelect"`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add extract/dlc.go extract/dlc_test.go
git commit -m "feat(extract): ported DLC family extractors (unverified)"
```

---

## Task 10: ExtractHTML dispatcher

**Files:**
- Create: `extract/extract.go`
- Test: `extract/extract_test.go`

Ties it together: parse → classify → derive taxonomy → dispatch → assemble `types.Question` (capture block, merged warnings, `tags: []`).

- [ ] **Step 1: Write the failing test**

Create `extract/extract_test.go`:
```go
package extract

import (
	"strings"
	"testing"

	"github.com/cajundata/starshp_manifest/types"
)

func TestExtractHTMLMultipleChoice(t *testing.T) {
	q, err := ExtractHTML(strings.NewReader(mcHTML), types.Source{Path: "acct4421_gov-nonprof-acct/mod02/mod02_001.html"})
	if err != nil {
		t.Fatal(err)
	}
	if q.Type != types.TypeMultipleChoice {
		t.Errorf("type = %q", q.Type)
	}
	if q.SchemaVersion != types.SchemaVersion {
		t.Errorf("schemaVersion = %d", q.SchemaVersion)
	}
	if q.Source.CourseCode == nil || *q.Source.CourseCode != "acct4421" {
		t.Errorf("taxonomy not derived: %#v", q.Source)
	}
	if q.Tags == nil || len(q.Tags) != 0 {
		t.Errorf("tags must be empty slice, got %#v", q.Tags)
	}
	if q.Capture.HasCapturedChoices {
		t.Errorf("MC should not report captured choices")
	}
	if _, ok := q.Body.(types.MultipleChoiceBody); !ok {
		t.Errorf("body type = %T", q.Body)
	}
}

func TestExtractHTMLWorksheetCaptureBlock(t *testing.T) {
	q, _ := ExtractHTML(strings.NewReader(wsCapturedPanels), types.Source{Path: "x/004.html"})
	if q.Type != types.TypeWorksheet {
		t.Fatalf("type = %q", q.Type)
	}
	if !q.Capture.HasCapturedChoices {
		t.Errorf("should report captured choices")
	}
	if q.Capture.CapturedTabCount != 2 {
		t.Errorf("capturedTabCount = %d, want 2", q.Capture.CapturedTabCount)
	}
	// path has no acct#### -> course-not-derivable warning present
	found := false
	for _, w := range q.Warnings {
		if w == types.WarnCourseNotDerivable {
			found = true
		}
	}
	if !found {
		t.Errorf("expected course-not-derivable warning, got %v", q.Warnings)
	}
}

func TestExtractHTMLUnknown(t *testing.T) {
	q, _ := ExtractHTML(strings.NewReader(`<div class="weird-thing other"></div>`), types.Source{Path: "001.html"})
	if q.Type != types.TypeUnknown {
		t.Fatalf("type = %q", q.Type)
	}
	ub := q.Body.(types.UnknownBody)
	if len(ub.RawClasses) == 0 {
		t.Errorf("unknown body should list rawClasses")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./extract/ -run ExtractHTML`
Expected: FAIL — `undefined: ExtractHTML`.

- [ ] **Step 3: Implement the dispatcher**

Create `extract/extract.go`:
```go
// Package extract converts a single Connect HTML document into a types.Question.
// It is pure: no printing, no filesystem, no process exit.
package extract

import (
	"io"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/cajundata/starshp_manifest/classify"
	"github.com/cajundata/starshp_manifest/internal/cells"
	"github.com/cajundata/starshp_manifest/taxonomy"
	"github.com/cajundata/starshp_manifest/types"
)

// ExtractHTML parses r and returns the populated Question envelope. The error is
// non-nil only when the HTML cannot be parsed at all.
func ExtractHTML(r io.Reader, src types.Source) (types.Question, error) {
	derived, taxWarn := taxonomy.Derive(src.Path)
	// Preserve caller-provided path; take derived course/module fields.
	derived.Path = src.Path

	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return types.Question{
			SchemaVersion: types.SchemaVersion,
			Source:        derived,
			Type:          types.TypeUnknown,
			Capture:       types.Capture{},
			Warnings:      mergeWarn(taxWarn, types.WarnParseFailed),
			Tags:          []string{},
			Body:          types.UnknownBody{RawClasses: []string{}, Note: "html parse failed"},
		}, err
	}

	qType := classify.Classify(doc)
	body, title, warns := dispatch(qType, doc)

	q := types.Question{
		SchemaVersion: types.SchemaVersion,
		Source:        derived,
		Type:          qType,
		Title:         title,
		Capture: types.Capture{
			HasCapturedChoices: cells.HasCapturedChoices(doc.Selection),
			CapturedTabCount:   capturedTabCount(doc),
		},
		Warnings: mergeWarn(taxWarn, warns...),
		Tags:     []string{},
		Body:     body,
	}
	return q, nil
}

func dispatch(t types.QuestionType, doc *goquery.Document) (any, *string, []string) {
	switch t {
	case types.TypeWorksheet:
		return extractWorksheet(doc)
	case types.TypeMultipleChoice:
		return extractMultipleChoice(doc)
	case types.TypeMatching:
		return extractMatching(doc)
	case types.TypeFillInTheBlank:
		return extractFillInTheBlank(doc)
	case types.TypeTrueFalse:
		return extractTrueFalse(doc)
	case types.TypeMultipleSelect:
		return extractMultipleSelect(doc)
	default:
		return unknownBody(doc), nil, []string{}
	}
}

func unknownBody(doc *goquery.Document) types.UnknownBody {
	set := map[string]struct{}{}
	doc.Find("[class]").Each(func(_ int, s *goquery.Selection) {
		for _, c := range strings.Fields(s.AttrOr("class", "")) {
			set[c] = struct{}{}
		}
	})
	classes := make([]string, 0, len(set))
	for c := range set {
		classes = append(classes, c)
	}
	sort.Strings(classes)
	return types.UnknownBody{RawClasses: classes, Note: "no discriminator matched"}
}

// mergeWarn combines an optional taxonomy warning ("" = none) with extractor warnings.
func mergeWarn(taxWarn string, rest ...string) []string {
	out := []string{}
	if taxWarn != "" {
		out = append(out, taxWarn)
	}
	out = append(out, rest...)
	if out == nil {
		return []string{}
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./extract/`
Expected: PASS (all extract tests).

- [ ] **Step 5: Commit**

```bash
git add extract/extract.go extract/extract_test.go
git commit -m "feat(extract): ExtractHTML dispatcher with capture block and taxonomy"
```

---

## Task 11: Manifest builder

**Files:**
- Create: `manifest/manifest.go`
- Test: `manifest/manifest_test.go`

- [ ] **Step 1: Write the failing test**

Create `manifest/manifest_test.go`:
```go
package manifest

import (
	"testing"

	"github.com/cajundata/starshp_manifest/types"
)

func TestBuild(t *testing.T) {
	code := "acct4421"
	mod := "mod02"
	title := "Item 1"
	qs := []types.Question{
		{
			Source:   types.Source{Path: "a/mod02/x.html", CourseCode: &code, Module: &mod},
			Type:     types.TypeMultipleChoice,
			Title:    &title,
			Capture:  types.Capture{HasCapturedChoices: false},
			Warnings: []string{},
		},
		{
			Source:   types.Source{Path: "a/mod02/y.html"},
			Type:     types.TypeWorksheet,
			Capture:  types.Capture{HasCapturedChoices: true},
			Warnings: []string{"dropdown-options-not-captured"},
		},
	}
	m := Build(qs, "a")
	if m.SchemaVersion != types.SchemaVersion || m.GeneratedFrom != "a" || m.Count != 2 {
		t.Fatalf("header wrong: %#v", m)
	}
	if m.Questions[0].CourseCode == nil || *m.Questions[0].CourseCode != "acct4421" {
		t.Errorf("entry0 courseCode = %v", m.Questions[0].CourseCode)
	}
	if m.Questions[1].Warnings != 1 || !m.Questions[1].HasCapturedChoices {
		t.Errorf("entry1 = %#v", m.Questions[1])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./manifest/`
Expected: FAIL — `undefined: Build`.

- [ ] **Step 3: Implement Build**

Create `manifest/manifest.go`:
```go
// Package manifest builds the top-level index from extracted questions.
package manifest

import "github.com/cajundata/starshp_manifest/types"

// Build produces the manifest index. generatedFrom is the input root descriptor.
func Build(questions []types.Question, generatedFrom string) types.Manifest {
	m := types.Manifest{
		SchemaVersion: types.SchemaVersion,
		GeneratedFrom: generatedFrom,
		Count:         len(questions),
		Questions:     make([]types.ManifestEntry, 0, len(questions)),
	}
	for _, q := range questions {
		m.Questions = append(m.Questions, types.ManifestEntry{
			Path:               q.Source.Path,
			CourseCode:         q.Source.CourseCode,
			Module:             q.Source.Module,
			Type:               q.Type,
			Title:              q.Title,
			HasCapturedChoices: q.Capture.HasCapturedChoices,
			Warnings:           len(q.Warnings),
		})
	}
	return m
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./manifest/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add manifest/
git commit -m "feat(manifest): build index from extracted questions"
```

---

## Task 12: Audit dev tool

**Files:**
- Create: `audit/audit.go`
- Test: `audit/audit_test.go`

Counts CSS classes that no handler knows about, across an `fs.FS` of `.html` files — a backlog generator.

- [ ] **Step 1: Write the failing test**

Create `audit/audit_test.go`:
```go
package audit

import (
	"testing"
	"testing/fstest"
)

func TestScanReportsUnknownClasses(t *testing.T) {
	fsys := fstest.MapFS{
		"a.html": {Data: []byte(`<div class="worksheet-wrap"></div><div class="mystery-grid widget-xyz"></div>`)},
		"b.html": {Data: []byte(`<span class="widget-xyz"></span>`)},
		"notes.txt": {Data: []byte("ignored")},
	}
	counts, err := Scan(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if counts["worksheet-wrap"] != 0 {
		t.Errorf("known class worksheet-wrap should be excluded, got %d", counts["worksheet-wrap"])
	}
	if counts["widget-xyz"] != 2 {
		t.Errorf("widget-xyz = %d, want 2", counts["widget-xyz"])
	}
	if counts["mystery-grid"] != 1 {
		t.Errorf("mystery-grid = %d, want 1", counts["mystery-grid"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./audit/`
Expected: FAIL — `undefined: Scan`.

- [ ] **Step 3: Implement Scan**

Create `audit/audit.go`:
```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./audit/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add audit/
git commit -m "feat(audit): unknown-CSS-class scanner"
```

---

## Task 13: Batch tree walker

**Files:**
- Create: `batch/batch.go`
- Test: `batch/batch_test.go`

Walks an `fs.FS` for `*.html` in deterministic order, extracts each with per-file panic isolation (a panic becomes an `unknown` question + `parse-failed` warning, batch continues), calls an optional progress hook, and builds the manifest.

- [ ] **Step 1: Write the failing test**

Create `batch/batch_test.go`:
```go
package batch

import (
	"testing"
	"testing/fstest"

	"github.com/cajundata/starshp_manifest/types"
)

func TestExtractTree(t *testing.T) {
	mc := `<ul class="answers--mc"><li class="answer-wrap--mc"><label class="answer__label--mc"><p>A</p></label></li></ul>`
	ws := `<div class="worksheet-wrap"><div class="worksheet__main"><p>scenario</p></div></div>`
	fsys := fstest.MapFS{
		"acct4421_gov-nonprof-acct/mod01/mod01_001.html": {Data: []byte(mc)},
		"acct4421_gov-nonprof-acct/mod01/mod01_002.html": {Data: []byte(ws)},
		"readme.txt": {Data: []byte("ignore me")},
	}

	var progress [][2]int
	res, err := ExtractTree(fsys, Options{
		Root:     "courseroot",
		Progress: func(done, total int) { progress = append(progress, [2]int{done, total}) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Questions) != 2 {
		t.Fatalf("questions = %d, want 2", len(res.Questions))
	}
	// deterministic order: mod01_001 before mod01_002
	if res.Questions[0].Source.Path != "acct4421_gov-nonprof-acct/mod01/mod01_001.html" {
		t.Errorf("order wrong: %s", res.Questions[0].Source.Path)
	}
	if res.Manifest.Count != 2 || res.Manifest.GeneratedFrom != "courseroot" {
		t.Errorf("manifest = %#v", res.Manifest)
	}
	if res.ByType[types.TypeMultipleChoice] != 1 || res.ByType[types.TypeWorksheet] != 1 {
		t.Errorf("byType = %#v", res.ByType)
	}
	if len(progress) != 2 || progress[1] != [2]int{2, 2} {
		t.Errorf("progress = %v, want final (2,2)", progress)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./batch/`
Expected: FAIL — `undefined: ExtractTree`.

- [ ] **Step 3: Implement ExtractTree**

Create `batch/batch.go`:
```go
// Package batch walks a filesystem of saved Connect HTML and extracts every file.
package batch

import (
	"io/fs"
	"sort"
	"strings"

	"github.com/cajundata/starshp_manifest/extract"
	"github.com/cajundata/starshp_manifest/manifest"
	"github.com/cajundata/starshp_manifest/taxonomy"
	"github.com/cajundata/starshp_manifest/types"
)

// Options configures a tree extraction.
type Options struct {
	// Root is a descriptor stored in the manifest's generatedFrom field.
	Root string
	// Progress, if set, is called after each file with (done, total).
	Progress func(done, total int)
}

// Result is the full output of a batch run.
type Result struct {
	Questions        []types.Question
	Manifest         types.Manifest
	ByType           map[types.QuestionType]int
	FilesWithWarning int
	Unknowns         int
}

// ExtractTree extracts every *.html file under fsys (deterministic lexical order).
func ExtractTree(fsys fs.FS, opts Options) (Result, error) {
	paths, err := collectHTML(fsys)
	if err != nil {
		return Result{}, err
	}

	res := Result{
		Questions: make([]types.Question, 0, len(paths)),
		ByType:    map[types.QuestionType]int{},
	}
	total := len(paths)
	for i, p := range paths {
		q := extractOne(fsys, p)
		res.Questions = append(res.Questions, q)
		res.ByType[q.Type]++
		if len(q.Warnings) > 0 {
			res.FilesWithWarning++
		}
		if q.Type == types.TypeUnknown {
			res.Unknowns++
		}
		if opts.Progress != nil {
			opts.Progress(i+1, total)
		}
	}
	res.Manifest = manifest.Build(res.Questions, opts.Root)
	return res, nil
}

// extractOne isolates a single file: any panic degrades to an unknown question.
func extractOne(fsys fs.FS, path string) (q types.Question) {
	defer func() {
		if r := recover(); r != nil {
			derived, taxWarn := deriveSafe(path)
			derived.Path = path
			warns := []string{types.WarnParseFailed}
			if taxWarn != "" {
				warns = append([]string{taxWarn}, warns...)
			}
			q = types.Question{
				SchemaVersion: types.SchemaVersion,
				Source:        derived,
				Type:          types.TypeUnknown,
				Warnings:      warns,
				Tags:          []string{},
				Body:          types.UnknownBody{RawClasses: []string{}, Note: "panic during extraction"},
			}
		}
	}()

	f, err := fsys.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	q, err = extract.ExtractHTML(f, types.Source{Path: path})
	if err != nil {
		// ExtractHTML already returned a populated unknown envelope on parse error.
		return q
	}
	return q
}

func collectHTML(fsys fs.FS) ([]string, error) {
	var paths []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(p), ".html") {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

// deriveSafe wraps taxonomy.Derive for use in the panic-recovery path.
func deriveSafe(path string) (types.Source, string) {
	return taxonomy.Derive(path)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./batch/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add batch/
git commit -m "feat(batch): tree walker with per-file isolation and progress hook"
```

---

## Task 14: CLI frontend

**Files:**
- Create: `cmd/manifest/main.go`
- Test: `cmd/manifest/main_test.go`

Thin adapter. The verb logic lives in testable `run*` functions that take an output writer and return an error — `main` only wires `os.Args`/`os.Stdout`/`os.Exit`.

- [ ] **Step 1: Write the failing test**

Create `cmd/manifest/main_test.go`:
```go
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cajundata/starshp_manifest/types"
)

func TestRunExtractWritesPerFileJSONAndManifest(t *testing.T) {
	in := t.TempDir()
	out := t.TempDir()
	mc := `<ul class="answers--mc"><li class="answer-wrap--mc"><label class="answer__label--mc"><p>A</p></label></li></ul>`
	if err := os.MkdirAll(filepath.Join(in, "mod01"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(in, "mod01", "q1.html"), []byte(mc), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := runExtract(&buf, in, out); err != nil {
		t.Fatalf("runExtract: %v", err)
	}

	// per-file JSON mirrors the input tree
	jsonPath := filepath.Join(out, "mod01", "q1.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("expected %s: %v", jsonPath, err)
	}
	var q types.Question
	if err := json.Unmarshal(data, &q); err != nil {
		t.Fatalf("per-file JSON invalid: %v", err)
	}
	if q.Type != types.TypeMultipleChoice {
		t.Errorf("type = %q", q.Type)
	}

	// manifest at output root
	if _, err := os.Stat(filepath.Join(out, "manifest.json")); err != nil {
		t.Errorf("manifest.json missing: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("multipleChoice")) {
		t.Errorf("summary should mention type counts; got: %s", buf.String())
	}
}

func TestRunPlanWritesNothing(t *testing.T) {
	in := t.TempDir()
	if err := os.WriteFile(filepath.Join(in, "q.html"), []byte(`<ul class="answers--mc"></ul>`), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := runPlan(&buf, in); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(in)
	if len(entries) != 1 {
		t.Errorf("plan must not write files; dir now has %d entries", len(entries))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/manifest/`
Expected: FAIL — `undefined: runExtract` / `runPlan`.

- [ ] **Step 3: Implement the CLI**

Create `cmd/manifest/main.go`:
```go
// Command manifest is the CLI frontend over the manifest extractor core.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cajundata/starshp_manifest/audit"
	"github.com/cajundata/starshp_manifest/batch"
)

const usage = `manifest — Connect HTML -> JSON extractor

Usage:
  manifest extract <inputDir> [-o outDir]   per-file JSON + manifest index
  manifest plan    <inputDir>               dry run: classify, print breakdown
  manifest audit   <inputDir>               dump unknown CSS classes
`

func main() {
	if err := dispatch(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func dispatch(args []string, w io.Writer) error {
	if len(args) < 2 {
		fmt.Fprint(w, usage)
		if len(args) == 0 {
			return nil
		}
		return fmt.Errorf("missing input directory")
	}
	verb, inputDir := args[0], args[1]
	switch verb {
	case "extract":
		out := defaultOut(inputDir)
		if i := indexOf(args, "-o"); i >= 0 && i+1 < len(args) {
			out = args[i+1]
		}
		return runExtract(w, inputDir, out)
	case "plan":
		return runPlan(w, inputDir)
	case "audit":
		return runAudit(w, inputDir)
	default:
		fmt.Fprint(w, usage)
		return fmt.Errorf("unknown command %q", verb)
	}
}

func defaultOut(inputDir string) string {
	return filepath.Join(inputDir, "_json")
}

func indexOf(args []string, flag string) int {
	for i, a := range args {
		if a == flag {
			return i
		}
	}
	return -1
}

func runExtract(w io.Writer, inputDir, outDir string) error {
	res, err := batch.ExtractTree(os.DirFS(inputDir), batch.Options{
		Root: inputDir,
		Progress: func(done, total int) {
			fmt.Fprintf(w, "\rextracting %d/%d", done, total)
		},
	})
	if err != nil {
		return err
	}
	fmt.Fprintln(w)

	for _, q := range res.Questions {
		rel := strings.TrimSuffix(q.Source.Path, filepath.Ext(q.Source.Path)) + ".json"
		dst := filepath.Join(outDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		data, err := json.MarshalIndent(q, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return err
		}
	}

	mdata, err := json.MarshalIndent(res.Manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "manifest.json"), mdata, 0o644); err != nil {
		return err
	}

	printSummary(w, res)
	return nil
}

func runPlan(w io.Writer, inputDir string) error {
	res, err := batch.ExtractTree(os.DirFS(inputDir), batch.Options{Root: inputDir})
	if err != nil {
		return err
	}
	printSummary(w, res)
	return nil
}

func runAudit(w io.Writer, inputDir string) error {
	counts, err := audit.Scan(os.DirFS(inputDir))
	if err != nil {
		return err
	}
	type kv struct {
		class string
		n     int
	}
	rows := make([]kv, 0, len(counts))
	for c, n := range counts {
		rows = append(rows, kv{c, n})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].n != rows[j].n {
			return rows[i].n > rows[j].n
		}
		return rows[i].class < rows[j].class
	})
	fmt.Fprintf(w, "unknown CSS classes (%d distinct):\n", len(rows))
	for _, r := range rows {
		fmt.Fprintf(w, "%6d  %s\n", r.n, r.class)
	}
	return nil
}

func printSummary(w io.Writer, res batch.Result) {
	fmt.Fprintf(w, "extracted %d question(s)\n", len(res.Questions))
	names := make([]string, 0, len(res.ByType))
	counts := map[string]int{}
	for t, n := range res.ByType {
		names = append(names, string(t))
		counts[string(t)] = n
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(w, "  %-16s %d\n", name, counts[name])
	}
	fmt.Fprintf(w, "files with warnings: %d   unknown: %d\n", res.FilesWithWarning, res.Unknowns)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/manifest/`
Expected: PASS.

- [ ] **Step 5: Verify the whole module builds and the binary runs**

Run:
```bash
go build ./...
go run ./cmd/manifest
```
Expected: build clean; `go run` with no args prints the usage block and exits 0.

- [ ] **Step 6: Commit**

```bash
git add cmd/
git commit -m "feat(cmd): manifest CLI with extract/plan/audit verbs"
```

---

## Task 15: Golden-file integration tests against real fixtures

**Files:**
- Create: `testdata/mc_001.html`, `testdata/ws_005.html` (copied real files)
- Create: `extract/golden_test.go`
- Create: `testdata/mc_001.golden.json`, `testdata/ws_005.golden.json` (generated, then verified)

- [ ] **Step 1: Copy representative real fixtures into testdata**

Run (PowerShell, from repo root):
```powershell
New-Item -ItemType Directory -Force testdata | Out-Null
Copy-Item .dev_references\001.html testdata\mc_001.html
Copy-Item .dev_references\005.html testdata\ws_005.html
```
Expected: both files exist under `testdata/` (these are committed even though `.dev_references/` is git-ignored).

- [ ] **Step 2: Write the golden test (with -update support)**

Create `extract/golden_test.go`:
```go
package extract

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/cajundata/starshp_manifest/types"
)

var update = flag.Bool("update", false, "regenerate golden files")

func TestGolden(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		golden   string
		wantType types.QuestionType
	}{
		{"mc_001", "../testdata/mc_001.html", "../testdata/mc_001.golden.json", types.TypeMultipleChoice},
		{"ws_005", "../testdata/ws_005.html", "../testdata/ws_005.golden.json", types.TypeWorksheet},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, err := os.Open(c.input)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			q, err := ExtractHTML(f, types.Source{Path: filepath.Base(c.input)})
			if err != nil {
				t.Fatal(err)
			}
			if q.Type != c.wantType {
				t.Fatalf("type = %q, want %q", q.Type, c.wantType)
			}
			got, err := json.MarshalIndent(q, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if *update {
				if err := os.WriteFile(c.golden, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(c.golden)
			if err != nil {
				t.Fatalf("read golden (run `go test ./extract -run TestGolden -update` first): %v", err)
			}
			if string(got) != string(want) {
				t.Errorf("golden mismatch for %s. Run with -update if the change is intended.\n--- got ---\n%s", c.name, got)
			}
		})
	}
}
```

- [ ] **Step 3: Generate the golden files, then inspect them**

Run:
```bash
go test ./extract/ -run TestGolden -update
```
Expected: creates `testdata/mc_001.golden.json` and `testdata/ws_005.golden.json`.

Then **manually inspect** both JSON files and confirm against the real markup:
- `mc_001.golden.json`: `type` is `multipleChoice`, `title` is `"Item 1"`, `stem` contains the printer-purchase question text, `choices` has 4 entries ("Option A".."Option D"), `correctIndex` is null.
- `ws_005.golden.json`: `type` is `worksheet`, `capture.hasCapturedChoices` is true, at least one dropdown cell has `options` including "Program revenue"/"Service revenue", and a leading blank option carries `correct: true` (the known correctness-semantics ambiguity — losslessly captured, to be revisited).

If anything is clearly wrong (not just the documented ambiguity), fix the relevant extractor and re-run `-update`.

- [ ] **Step 4: Run the golden test without -update to confirm it pins**

Run: `go test ./extract/ -run TestGolden`
Expected: PASS (output matches committed goldens).

- [ ] **Step 5: Run the full suite**

Run: `go test ./...`
Expected: all packages PASS.

- [ ] **Step 6: Commit**

```bash
git add testdata/ extract/golden_test.go
git commit -m "test: golden-file integration tests against real fixtures"
```

---

## Task 16: README usage update + final verification

**Files:**
- Modify: `README.md` (Status + a real usage example)

- [ ] **Step 1: Update the Status line and add a usage example**

In `README.md`, replace the `## Status` paragraph with:
```markdown
## Status

v1 extractor implemented (MC + worksheet verified against real fixtures; DLC
family ported but unverified). See the design spec and plan under
`docs/superpowers/`.

## Usage

```bash
go run ./cmd/manifest extract path/to/saved/html -o path/to/output
go run ./cmd/manifest plan   path/to/saved/html
go run ./cmd/manifest audit  path/to/saved/html
```
```

- [ ] **Step 2: Run the full test suite and vet**

Run:
```bash
go vet ./...
go test ./...
```
Expected: vet clean; all tests PASS.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: update README with v1 status and usage"
```

---

## Self-Review Notes (for the implementer)

- **Spec coverage:** every schema field (envelope, all six body variants + unknown, manifest, taxonomy, capture, tags) maps to a task (2, 4, 7–11). CLI verbs `extract`/`plan`/`audit` → Task 14. Golden tests against real `001.html`/`005.html` → Task 15.
- **Known v1 limitations (documented, intentionally deferred):** MC `correctIndex` always null (ceiling); DLC handlers unverified (`dlc-handler-unverified`); MC stems with embedded option tables are flattened to text; dropdown correctness semantics of the leading blank option are captured losslessly but not interpreted. These are "iterate later" holes, not plan gaps.
- **Type consistency:** extractor functions uniformly return `(any, *string, []string)`; `types.Question.Body` is `any`; warning codes are the `types.Warn*` constants throughout. The dispatcher (`dispatch`) and the per-type extractors share that signature so the switch is uniform.
- **No placeholders:** every code block is complete and compiles as written — no TODOs, stubbed bodies, or non-Go filler lines.
