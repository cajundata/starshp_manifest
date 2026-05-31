# IDEA: Boilerplate-Free Extraction of Connect Accounting Problems → JSON

**Status:** Design in progress (brainstorming paused mid-session)
**Date:** 2026-05-29
**Goal:** Reliably strip McGraw Hill Connect boilerplate from saved assignment HTML and extract the accounting problems (questions, answer choices, scenario data, tables) into structured JSON.

---

## Problem Statement

The repo contains many deeply-nested sub-directories of saved Connect assignment pages
(`mod<module_number>_<question_number>.html`), one question per file. Each file is a ~100–170 KB Ember.js
app shell where the real content is a tiny fraction of the DOM. We want an efficient,
reliable method to remove the boilerplate while preserving the problem formatting
(questions, answer choices, data, tables) for downstream programmatic use.

**Affected courses/dirs surveyed:**

- `acct4421_gov-nonprof-acct\{mod01, mod02, mod03}`
- `acct3221_tax-acct-01\{mod01, mod02, mod03}`
- `z_completed\acct_3020_int_accounting_1\homework`
- `C:\Users\weldo\OneDrive\School\LSUA_PBC_Accounting\acct4421_gov-nonprof-acct\z_midterm`

---

## Decisions Locked In

1. **JSON is the canonical source of truth.** It is lossless structured data; Markdown and
   the eventual study guide are _projections_ of the JSON.
   Pipeline: `HTML → (extractor) → JSON → (renderers) → Markdown / study guide / in-house apps`.
   The hard, fragile HTML parsing happens exactly once.

2. **Build the extractor in Go**, using `goquery` (jQuery-style CSS selectors over static HTML).
   The saved files are fully-rendered static snapshots, so no browser/Node DOM is needed —
   a static parser sees all content. The existing Chrome extension's JS serves as the
   _specification_ (which selectors are correct), not as reusable runtime code.

3. **Markdown output is a separate, thin renderer** that reads the JSON (for Obsidian, etc.).
   Keep Obsidian/formatting concerns out of the extraction logic.

4. **Option B — fix capture up front.** Recovering dropdown options and answer values is
   important, so we will enhance the Chrome extension to snapshot that data _before_ saving
   (rather than only parsing whatever is already in existing snapshots). The same JSON schema
   covers both; the capture fix fills fields that pure parsing of current files leaves empty. **COMPLETED**

5. **Full problem-type coverage.** The Go tool should handle every type the extension supports
   (multiple choice, worksheet + sub-variants, matching, fill-in-the-blank, true/false),
   not just the MC + Worksheet types that happen to appear in the current files. Cheap to port,
   future-proofs other courses.

---

## Key Findings From Investigation

### Chrome extension — `C:\Users\weldo\Projects\int-acct`

- Vanilla JS, Manifest V3, popup-driven. Two relevant features: **Save Page HTML** and
  **Extract Question → JSON** (stored in `chrome.storage.local`, exportable to `questions.json`).
- **Extraction logic has NO live-DOM-only dependencies** (no `getComputedStyle`, visibility,
  `:checked`, etc.) — selectors port cleanly to Go/goquery.
- **BUT its worksheet extractor is shallow:** it captures only `title`, `prompt` (scenario),
  and `requirements`. **It discards the entire answer table.** So the current JSON store is a
  partial for worksheets.
- It detects two page formats: `.dlc_question` (DLC: multiple_choice, multiple_select,
  true_false, matching, fill_in_the_blank) and `.worksheet-wrap` (exercise/worksheet).
- The **HTML save** feature does one genuinely browser-only thing: it live-clicks each
  "Required A/B/C" tab and waits for Ember to re-render, snapshotting each panel into
  `<section class="captured-tab-panel">`, and inlines same-origin iframes as
  `<div class="captured-iframe-content">`. This tab-capture trick is the model for the
  Option B dropdown-capture fix.
- Current JSON shape: `{ type, title, prompt, choices[], requirements[], terms? }`.

### domclean — `C:\Users\weldo\Projects\domclean`

- Go tool (uses `golang.org/x/net/html`), Wails GUI + CLI (`convert`/`plan`/`audit`).
- **Built for McGraw-Hill _textbook_ reader snapshots, NOT Connect _assignment_ worksheets.**
  The "notoriously misses fields" complaint is structural, not edge-case bugs.
- **Root cause of misses:** it reads only **text nodes, never attributes.** The account/line
  labels live in `aria-label`; computed logic in `formula=`. domclean never looks at either.
- No handlers for jSheet grids, dropdowns/`<select>`, tabs, or `captured-iframe-content`.
- **Reusable from domclean:** its scaffolding (extract→clean→convert→runner split, x/net/html
  parsing, batch/file-ordering) and especially its `audit` tool, which dumps unknown CSS
  classes — point it at Connect HTML to get a ready-made handler backlog. Its _converter_ is
  not reusable for our purposes.

### HTML variety survey

- **Only TWO top-level types actually appear** across all courses: **Multiple Choice** and
  **Worksheet**. (The extension's matching/FITB/TF support has no examples in the current tree,
  but we're porting it anyway per Decision 5.)
- Type is **per-file**, not per-directory — acct4421 homework folders interleave MC and
  worksheet files. The parser must branch on each file.
- **Worksheet sub-variants** (sub-flags, not separate top-level types): multi-tab Required
  A/B/C (`ul[role=tablist] > li.tab`), dropdown cells (`td.dropDownList[dropdowntype=dropDown]`),
  journal-entry grids, and multi-table problems (>1 `div.jSheetParent`).
- **Title is not uniform:** acct3221 (tax) + acct3020 worksheets/MC use `h2.question__title`;
  **acct4421 has no such element** (its only `h2` is the junk wrapper "Captured Requirement
  Panels"). Fallback to `h1#question-info-holder` ("Item N") + scenario text.

### THE DATA-AVAILABILITY CEILING (the finding that set scope)

Saved HTML captures worksheet **structure** but not **answers**:

- **Dropdown option lists are JS-injected — `<option>` count is literally zero** in every
  saved file. The selectable choices are absent.
- **Response-cell values are empty** — typed/correct answers are not persisted in the snapshot.
- Fully recoverable from current files: scenario text, required items, MC choice text, table
  row labels (`aria-label`), formulas (`formula=`), layout.
- This ceiling is exactly why we chose **Option B** (fix capture) rather than parse-only.

---

## Discriminated-Union JSON Schema (draft)

```
QuestionType =
  | { kind: "multipleChoice",
      marker: "ul.answers--mc",
      stem,            // p.question
      choices: [{ index, text }],   // li.answer-wrap--mc > p ; NOT labeled A/B/C in DOM
      title? }
  | { kind: "worksheet",
      marker: "div.worksheet-wrap + div.captured-iframe-content",
      title?,                       // h2.question__title (optional; absent in acct4421)
      scenario,                     // div.worksheet__main > p
      required: [ ol > li ],
      tabs?: [ "Required A", "Required B", ... ],
      tables: [ {
        headers: [],
        cells: [ {
          ariaLabel,                // best source of row/line label
          id,                       // e.g. 0_table0_cell_c2_r1
          cellType: "input" | "dropdown" | "readonly" | "formula",
          formula?,                 // on computed cells
          value?,                   // often null from current snapshots (Option B fills this)
          options?                  // dropdown choices (Option B fills this; empty today)
        } ]
      } ],
      subtype?: "calculation" | "journalEntry" | "dropdownMatching" }
  | { kind: "matching",  terms[], choices[] }        // ported from extension; no examples yet
  | { kind: "fillInTheBlank", promptWithBlanks }     // [BLANK n] markers
  | { kind: "trueFalse", stem, choices }
```

**Parser discriminator order:** (1) `div.worksheet-wrap` → worksheet; (2) else `.dlc_question`
→ branch on `awd-probe-type-*` class / legend text / input types; (3) else `ul.answers--mc`
→ multipleChoice; (4) else unknown.

---

## Reusable Selector Map (from the extension, verified static-safe)

| Field                           | Selector                                                                                                                            |
| ------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| Format: DLC                     | `.dlc_question`                                                                                                                     |
| Format: worksheet               | `.worksheet-wrap`                                                                                                                   |
| DLC type signal (authoritative) | `[class*="awd-probe-type-"]`, regex `awd-probe-type-([a-z_]+)`                                                                      |
| DLC prompt                      | `.prompt`                                                                                                                           |
| DLC choices (MC/MS/TF)          | `.choiceText`                                                                                                                       |
| Matching terms                  | `.match-row .match-prompt-label .content`                                                                                           |
| Matching definitions            | `.choices-container .choice-item-wrapper .choice-item .content`                                                                     |
| FITB blanks                     | `.fitb-input` (replace with `[BLANK n]`)                                                                                            |
| Worksheet title                 | `.worksheet-wrap .question__title`                                                                                                  |
| Worksheet body                  | `.worksheet__main` (iterate direct children; stop prompt at `<h3>`)                                                                 |
| Worksheet required items        | first `<ol>` after the `<h3>`, its `<li>`s                                                                                          |
| MC stem                         | `p.question`                                                                                                                        |
| MC choices                      | `li.answer-wrap--mc > p` (+ `input.answer--mc[type=radio]`)                                                                         |
| Worksheet table cells           | scope within `div.captured-iframe-content`: `td.responseCell`, `td.colHeader.td-readOnly`, `td.dropDownList[dropdowntype=dropDown]` |
| Cell label                      | `aria-label` attribute                                                                                                              |
| Cell formula                    | `formula` attribute                                                                                                                 |
| Tabs                            | `#tabs li.tab > span[title]` (e.g. "Required A")                                                                                    |

---

## Open Question (interrupted — resolve before designing the capture fix)

**Where does dropdown option data come from?** We were about to investigate whether the
option list for `dropdownid=N` is embedded _somewhere_ in the saved page (a JS/JSON blob,
hidden element, `EZ` global keyed by dropdown id) — in which case we could parse it and may
not even need to re-save — versus only being fetched/rendered on click (in which case the
extension must force each dropdown to render, like it already does for tabs, before capture).

This single fact determines the Option B implementation. **This investigation must run first.**

Also TBD for answers/values: whether correct/student answers exist anywhere in the page or
must likewise be captured via interaction.

---

## Proposed Next Steps

- [x] **Investigate the dropdown/answer data source** (the open question above) — definitive
      yes/no on whether it's parseable from the page vs. interaction-only.
      **dropdown/answer data source has been solved by updating the Chrome Extension that now captures all dropdowns as an unordered list for each questions containing a dropdown.**
- [x] Based on (1), **design the Chrome extension capture fix** (force-render dropdowns/values
      → snapshot into saved HTML, mirroring the existing tab-capture approach).
      **Complete**
- [ ] **Finalize the JSON schema** (the draft above) once we know what data is recoverable.
- [ ] **Build the Go extractor** (`goquery`): subtree isolation → descend into
      `captured-iframe-content` → classify → per-type extract reading **attributes** (not just
      text) → emit JSON. Reuse domclean's batching/file-ordering scaffolding and `audit` tool.
- [ ] **Build the Markdown renderer** (separate, reads JSON → `.md` for Obsidian).
- [ ] Later: **study-guide generator** as another JSON consumer.

---

## Reference Paths

- Chrome extension: `C:\Users\weldo\Projects\int-acct` (extraction: `popup\popup.js:459-589`; save: `popup\popup.js:58-244`)
- domclean: `C:\Users\weldo\Projects\domclean` (`internal/extract/srcdoc.go`, `internal/convert/markdown.go:49-122`, `internal/audit/audit.go:68-95`)
- Sample worksheet: `acct3221_tax-acct-01\mod03\hw04\page-2026-05-28T19-51-24.html` ("Problem 4-29")
- Sample MC: `acct4421_gov-nonprof-acct\mod02\qz-03-04\page-2026-05-23T17-15-11.html`
