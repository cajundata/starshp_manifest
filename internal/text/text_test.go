package text

import "testing"

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"  hello   world  ": "hello world",
		"line1\n\n  line2":  "line1 line2",
		"\t spaced \t":      "spaced",
		"Item  1":       "Item 1", // non-breaking space collapses too
		"":                  "",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}
