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
