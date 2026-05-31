// Package taxonomy derives deterministic course/module metadata from a file path.
package taxonomy

import (
	"regexp"
	"strings"

	"github.com/cajundata/starshp_manifest/types"
)

var (
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
