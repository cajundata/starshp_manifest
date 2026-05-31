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
