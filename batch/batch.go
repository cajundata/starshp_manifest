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
