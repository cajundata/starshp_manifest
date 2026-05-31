# Design: `manifest` — Connect HTML → JSON Extractor

**Status:** Approved (brainstorming complete)
**Date:** 2026-05-31
**Supersedes:** the draft schema and CLI assumptions in `IDEA.md`
**Scope of this spec:** the first sub-project — the JSON schema and the Go extractor that produces it. The Markdown renderer and study-guide generator are separate downstream JSON consumers, each with their own later spec/plan cycle.

---

## 1. Goal

Reliably strip McGraw-Hill Connect boilerplate from saved assignment HTML and extract the
accounting problems (stems, choices, scenarios, tables, dropdown options) into structured,
lossless JSON. JSON is the canonical source of truth; all other outputs (Markdown, study
guides, in-house apps) are projections of it. The fragile HTML parsing happens exactly once,
here.

Pipeline: `HTML → (manifest extractor) → JSON → (renderers) → Markdown / study guide / starshp`.

---

## 2. Context & key facts (verified against real files)

Input is deeply-nested directories of saved Connect pages, one question per file. The files
in `.dev_references/` were captured by the `int-acct` Chrome extension *after* its
dropdown-capture fix. Verified structure:

- **Two page types appear in real data:** worksheets (`div.worksheet-wrap`, also carry
  `dropDownList` cells) and multiple-choice (`ul.answers--mc`).
- **MC files carry no `awd-probe-type-*` signal and are not `.dlc_question`** — they are plain
  `ul.answers--mc`. The DLC family (matching / fill-in-the-blank / true-false / multiple-select)
  has **zero examples** in current data.
- **Dropdown options are now captured per cell.** Each `td.dropDownList.responseCell` contains
  `div.codex-captured-choices > ul[role=listbox] > li[role=option]`; option text is in
  `a.list_content`; the **correct** option's `span.answer_holder` carries the extra class `k`.
  The list ties back to its cell via `aria-labelledby` = cell `id`. Some worksheets
  (e.g. `004.html`) have the dropdown cells but **no** captured choices.
- **Multi-tab worksheets duplicate their tables:** once in the live `div.captured-iframe-content`
  (active tab only) and once per `section.captured-tab-panel` inside `#codex-captured-tab-panels`
  (`data-tab-label`, `data-captured-tab-count`).
- **Worksheets are title-less** (no `h2.question__title`); MC titles fall back to
  `h1#question-info-holder.t-hidden` → "Item N".
- **MC correctness is not recoverable** from snapshots (no checked/marked state).

Reusable from `domclean`: its batch/runner + file-ordering scaffolding and its `audit` tool
(dump unknown CSS classes → handler backlog). Its text-only *converter* is **not** reused.

---

## 3. Locked decisions

1. **Approach A — typed extractors behind a discriminator.** A classifier inspects each file and
   dispatches to a per-type extractor producing a typed struct that marshals to its branch of the
   discriminated union. (Rejected: a data-driven selector map — the worksheet's nested,
   attribute-bearing structure doesn't reduce to flat selectors.)
2. **Full type coverage (per `IDEA.md` Decision 5).** Port the DLC family from the extension's
   selectors even though no real examples exist. These handlers are explicitly **unverified
   against real data** and tag their output with a `dlc-handler-unverified` warning.
3. **Output: both per-file JSON and a top-level manifest index.** Per-file JSON mirrors the input
   tree (canonical source of truth, easy to diff and re-run); the manifest lists every question
   with type/title/source-path for renderers. (The manifest is the "manifest" in `starshp_manifest`.)
4. **Library-first; the CLI is one thin frontend.** The core is pure, importable, and knows nothing
   about stdout, flags, or `os.Exit`. A future Wails GUI and `starshp` are *other* frontends over
   the same core. v1 ships as the CLI.
5. **Standalone Go module.** `manifest` is its own repo/module with its own version tags; `starshp`
   consumes it via `go get`. It stays equally usable as CLI, GUI, or embedded library.

---

## 4. JSON schema

Every input file produces one **Question envelope** wrapping a type-specific `body`
(the discriminated union). `schemaVersion` starts at `1`.

### Envelope

```jsonc
{
  "schemaVersion": 1,
  "source": { "path": "acct4421/mod02/mod02_001.html", "module": "mod02" },
  "type": "multipleChoice" | "worksheet" | "matching" | "fillInTheBlank"
        | "trueFalse" | "multipleSelect" | "unknown",
  "title": "Item 1",            // best-available; see per-type fallbacks
  "capture": {                  // what the snapshot actually contained — first-class trust signal
    "hasCapturedChoices": true, // codex-captured-choices present (dropdown options recovered)
    "capturedTabCount": 2       // 0 = single panel / no tabs
  },
  "warnings": [],               // non-fatal: missing title, empty dropdowns, unknown classes, …
  "body": { /* one variant below */ }
}
```

### `multipleChoice`

```jsonc
{
  "stem": "Governments and not-for-profit organizations have…",  // p.question (inner <p> unwrapped)
  "choices": [ { "index": 0, "text": "Option A" } ],             // li.answer-wrap--mc > p
  "correctIndex": null   // NOT recoverable from snapshots — always null (documented ceiling)
}
// title fallback: h1#question-info-holder.t-hidden → "Item N"
```

### `worksheet`

```jsonc
{
  "scenario": "Carmel County has prepared the following schedule…",   // div.worksheet__main > p
  "required": [ "Does the disclosure comply…", "Does the county use…" ], // <ol> after <h3>Required
  "tabs": [                              // one entry per Required A/B/C; single-tab => one entry, label null
    {
      "label": "Required A",             // section.captured-tab-panel[data-tab-label]
      "tables": [                        // one per div.jSheetParent within this tab
        {
          "headers": ["", "Amount"],     // td.colHeader.td-readOnly text
          "rows": [
            {
              "label": "Program revenue",        // td.rowHeader.td-readOnly / aria-label
              "cells": [
                {
                  "id": "0_table0_cell_c1_r0",
                  "cellType": "dropdown",        // input | dropdown | readonly | formula
                  "ariaLabel": "Does the county use the modified approach…",
                  "formula": null,               // formula= attr when present
                  "value": "Program revenue",    // captured selected/typed value, else null
                  "options": [                   // from codex-captured-choices; [] if not captured
                    { "index": 1, "text": "Program revenue", "correct": false },
                    { "index": 2, "text": "Service revenue",  "correct": true } // answer_holder "k"
                  ]
                }
              ]
            }
          ]
        }
      ]
    }
  ]
}
// title: optional, usually absent in worksheets (h2.question__title when present)
```

### DLC family (ported, unverified — each carries `"warnings": ["dlc-handler-unverified"]`)

```jsonc
"matching":       { "terms": [ ... ], "choices": [ ... ] }          // .match-row / .choices-container
"fillInTheBlank": { "promptWithBlanks": "… [BLANK 1] … [BLANK 2]" }  // .fitb-input → [BLANK n]
"trueFalse":      { "stem": "...", "choices": ["True","False"], "correctIndex": null }
"multipleSelect": { "stem": "...", "choices": [ { "index": 0, "text": "..." } ], "correctIndices": null }
```

### `unknown` (safety net)

```jsonc
{ "rawClasses": [ "..." ], "note": "no discriminator matched" }
```

### Manifest (top-level index)

```jsonc
{
  "schemaVersion": 1,
  "generatedFrom": "<input root>",
  "count": 79,
  "questions": [
    { "path": "acct4421/mod02/mod02_001.html", "module": "mod02",
      "type": "worksheet", "title": null, "hasCapturedChoices": true, "warnings": 0 }
  ]
}
```

**Schema rationale.** The `capture` block makes "was this snapshot complete?" a queryable fact.
`warnings` keeps partial data flowing instead of failing a whole batch. `cellType` is normalized
up front so renderers never re-sniff DOM classes. `unknown` guarantees no file is silently dropped.

---

## 5. Extractor architecture (Approach A)

### Module layout

```
starshp_manifest/                 module: github.com/weldo/starshp_manifest
  types/         // envelope + variant structs (the schema) — the shared contract
  classify/      // the discriminator → question type
  extract/       // ExtractHTML(io.Reader, Source) (types.Question, Diagnostics) — pure, one doc
    worksheet.go
    multiplechoice.go
    dlc.go         // matching / fillInTheBlank / trueFalse / multipleSelect (ported, unverified)
  internal/cells // private shared jSheet cell + captured-choices parsing
  batch/         // ExtractTree(fs.FS, Options) (Result) — walks a tree, no printing
  manifest/      // builds the index from []Question
  audit/         // ported domclean dev tool: dump unknown CSS classes
  cmd/manifest/  // THIN CLI adapter: flags → batch.ExtractTree → write files → print summary
  testdata/      // representative .html + golden .json
```

Parsing uses `goquery` (static CSS selectors over fully-rendered snapshots; no live DOM needed).
Only genuinely-private helpers live under `internal/`; everything a GUI or `starshp` needs to
import is exported.

### Core API shape (library-first)

- `extract.ExtractHTML(r io.Reader, src types.Source) (types.Question, types.Diagnostics)` — one
  document, pure, returns data (never prints, never exits).
- `batch.ExtractTree(fsys fs.FS, opts batch.Options) (types.Result, error)` where
  `Result = { Questions []Question, Manifest, Diagnostics }`. `Options` includes an optional
  `Progress func(done, total int)` hook (CLI ignores it; a GUI wires it to a progress bar) and
  takes an `fs.FS` so tests pass an in-memory FS and the CLI passes `os.DirFS`.

### Data flow (per file — no shared state, trivially parallelizable later)

```
discover → read → goquery.Parse → classify → dispatch to extractor
  → build envelope (source/module from path, capture block, warnings)
  → marshal → write mirrored per-file .json
  → emit manifest entry
[after all files] → write manifest.json + print summary (counts, by-type, warnings, unknowns)
```

### Classifier (real-data order; DLC branch present but currently dormant)

1. `div.worksheet-wrap` → **worksheet**
2. `.dlc_question` → read `[class*="awd-probe-type-"]`, regex `awd-probe-type-([a-z_]+)` →
   **matching / fillInTheBlank / trueFalse / multipleSelect**
3. `ul.answers--mc` → **multipleChoice**
4. else → **unknown** (with `rawClasses`)

### Worksheet extractor specifics (the hard part)

- Content root `div.worksheet-wrap`; scenario from `div.worksheet__main > p`; required items from
  the `<ol>` after `<h3>Required`.
- **Table-source rule:** if `#codex-captured-tab-panels` is present, iterate
  `section.captured-tab-panel` (one `tab` per panel, label from `data-tab-label`), reading tables
  from each panel's `div.jSheetParent`. Otherwise, one tab (label `null`), tables from the single
  `div.captured-iframe-content`. This avoids double-counting the active tab.
- **Cells** (`internal/cells`): classify each `td` → `colHeader`/`rowHeader` (readonly),
  `dropdown` (`dropdowntype=dropDown`), `formula` (has `formula=`), else `input`. Pull `id`,
  `aria-label`, `formula`, captured `value`.
- **Dropdown options:** within the cell,
  `div.codex-captured-choices li[role=option] a.list_content` → `{ index, text }`; mark
  `correct: true` when the sibling `span.answer_holder` has class `k`. Missing container →
  `options: []` + warning `dropdown-options-not-captured`.

### Multiple-choice extractor

- Stem from `p.question` (unwrap the nested inner `<p>`); choices from `li.answer-wrap--mc > p`
  (in document order, 0-indexed); title from `h1#question-info-holder.t-hidden` → "Item N".
- `correctIndex` always `null` (ceiling).

### DLC extractors (ported, unverified)

- Built from the extension's documented selectors (`.match-row .match-prompt-label .content`,
  `.choices-container .choice-item-wrapper .choice-item .content`, `.fitb-input`, `.prompt`,
  `.choiceText`). Output shape only; every record carries `dlc-handler-unverified`. Not trusted
  until a real example appears.

### Error handling

Parsing is **per-file isolated**: a panic or parse failure on one file degrades to an `unknown`
envelope plus a warning, and the batch continues. The final summary reports total / by-type /
files-with-warnings / unknowns so nothing fails silently.

### CLI (thin adapter; mirrors domclean's verb split)

- `manifest extract <inputDir> [-o outDir]` — main path: per-file JSON + manifest.
- `manifest plan <inputDir>` — dry run: classify every file, print the type breakdown, write nothing.
- `manifest audit <inputDir>` — dump unknown/unhandled CSS classes (handler backlog).

---

## 6. Testing

**Golden-file tests are the backbone.** `testdata/` holds curated real `.html` inputs paired with
expected `.json`. Tests diff actual output against the golden; a `-update` flag regenerates goldens
on intentional schema changes. This pins exact output for real Connect markup against drift.

**Representative fixtures** (copied from `.dev_references`, covering the structural variety found):

- **MC:** `001.html` (plain stem) + a `mod01_*` (nested `<p>` in `p.question` — exercises
  stem-unwrapping).
- **Worksheet, choices captured + multi-tab:** `005.html` (`codex-captured-choices` present, a
  `correct` option via `answer_holder k`, multiple `captured-tab-panel`s).
- **Worksheet, no captured choices:** `004.html` — asserts `options: []` **and** the
  `dropdown-options-not-captured` warning (the partial-data path).
- **Worksheet, multi-table:** one large file (e.g. `013`/`015`) with several `div.jSheetParent`
  in one tab.
- **unknown:** a tiny synthetic file matching no discriminator → asserts the `unknown` envelope +
  `rawClasses`.

**Unit tests** for the fiddly bits: cell-type classification (`internal/cells`), the
`awd-probe-type-` regex, title fallbacks, the table-source rule (captured-panels vs live iframe).

**Batch tests** use an in-memory `fstest.MapFS` (no disk) to verify tree-walk ordering, manifest
aggregation, and that a deliberately-broken file degrades to `unknown` without aborting the batch.

**DLC family:** synthetic fixtures from the extension's documented selectors, asserting shape only,
explicitly marked unverified-against-real-data so a passing test is never mistaken for "works on
real files."

---

## 7. Out of scope (separate later cycles)

- Markdown renderer (separate JSON consumer).
- Study-guide generator (separate JSON consumer).
- A Wails GUI for `manifest` and/or integration into `starshp` — the architecture leaves both open
  (library-first, standalone importable module), but no GUI code ships in v1.

---

## 8. Known ceilings & risks

- **MC correct answers are unrecoverable** from current snapshots (no marked state). `correctIndex`
  stays `null` until/unless the capture is enhanced.
- **DLC handlers are unverified** — no real examples exist; treat their output as best-effort.
- **Worksheets without captured choices** (e.g. `004.html`) yield empty dropdown option lists by
  design; the `dropdown-options-not-captured` warning makes this explicit rather than silent.

---

## Reference paths

- Sample files: `C:\Users\weldo\Projects\starshp_manifest\.dev_references\` — worksheets
  `004–016.html`; MC `001–003.html`, `mod01_*`, `mod02_*`, `mod03_*`.
- Chrome extension (selector spec, capture logic): `C:\Users\weldo\Projects\int-acct`.
- domclean (reusable scaffolding + `audit`): `C:\Users\weldo\Projects\domclean`.
- Originating idea doc: `IDEA.md`.
