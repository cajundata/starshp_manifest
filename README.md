# starshp_manifest

`manifest` — a boilerplate-free extractor that turns saved **McGraw-Hill Connect** assignment
HTML into structured, lossless **JSON**.

Each saved Connect page is a ~100 KB–2 MB Ember app shell where the real problem (stem, choices,
scenario, tables, dropdown options) is a tiny fraction of the DOM. `manifest` strips the
boilerplate and emits clean JSON that downstream tools can render to Markdown, study guides, or
feed into other apps.

```
HTML → (manifest extractor) → JSON → (renderers) → Markdown / study guide / starshp
```

JSON is the **canonical source of truth**; everything else is a projection of it. The fragile
HTML parsing happens exactly once.

## Status

Design complete; implementation not yet started. See the design spec:
[`docs/superpowers/specs/2026-05-31-connect-html-extractor-design.md`](docs/superpowers/specs/2026-05-31-connect-html-extractor-design.md).

## What it produces

One JSON **envelope** per HTML file (mirroring the input tree) plus a top-level **manifest**
index listing every question. The envelope is a discriminated union over question type:

- `multipleChoice` — stem + choices (correct answer not recoverable from snapshots)
- `worksheet` — scenario, required items, per-tab jSheet tables with cells; dropdown cells carry
  their captured options and which option is correct
- `matching` / `fillInTheBlank` / `trueFalse` / `multipleSelect` — ported for coverage, currently
  unverified against real data
- `unknown` — safety net so no file is ever silently dropped

Every envelope also carries deterministic **taxonomy** derived from the input path
(`courseCode`, `courseName`, `module`) and a reserved `tags[]` slot for a future semantic
topic-tagging pass.

## Design

`manifest` is **library-first**: a pure, importable Go core (parsing knows nothing about stdout
or flags) with the CLI as one thin frontend. This keeps the door open to a Wails GUI or direct
embedding into `starshp` later. It is a standalone Go module so `starshp` can consume it via
`go get`.

Parsing uses [`goquery`](https://github.com/PuerkitoBio/goquery) over the fully-rendered static
snapshots — no browser/Node DOM required.

### CLI (planned)

```
manifest extract <inputDir> [-o outDir]   # per-file JSON + manifest index
manifest plan    <inputDir>               # dry run: classify every file, print type breakdown
manifest audit   <inputDir>               # dump unknown/unhandled CSS classes (handler backlog)
```

## Sample data

Representative captured HTML lives in `.dev_references/` (git-ignored). Worksheets:
`004–016.html`; multiple-choice: `001–003.html`, `mod01_*`, `mod02_*`, `mod03_*`.

## Related projects

- **int-acct** — the Chrome extension that captures the Connect pages (selector spec + the
  dropdown-capture fix).
- **domclean** — Wails app for McGraw-Hill *textbook* snapshots; its batch/runner scaffolding and
  `audit` tool are reused here.
