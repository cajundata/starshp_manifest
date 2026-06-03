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
	StemTable    *StemTable `json:"stemTable,omitempty"`
	Choices      []MCChoice `json:"choices"`
	CorrectIndex *int       `json:"correctIndex"`
}

// StemTable is a faithful capture of a plain HTML <table> embedded in a question
// stem (e.g. a list of balances). Unlike worksheet Table cells, these are static
// display values, so each cell carries only its text.
type StemTable struct {
	Headers []string       `json:"headers"`
	Rows    []StemTableRow `json:"rows"`
}

type StemTableRow struct {
	Label string          `json:"label"`
	Cells []StemTableCell `json:"cells"`
}

type StemTableCell struct {
	Value string `json:"value"`
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
