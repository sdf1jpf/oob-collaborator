package hostedfile

import "testing"

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		in    string
		want  string
		valid bool
	}{
		{"evil.dtd", "/evil.dtd", true},
		{"/evil.dtd", "/evil.dtd", true},
		{"nested/evil.dtd", "/nested/evil.dtd", true},
		{"/nested/evil.dtd", "/nested/evil.dtd", true},
		{"", "", false},
		{"/", "", false},
		{"../evil.dtd", "", false},
		{"/foo/../bar", "", false},
		{"/bad path.dtd", "", false},
		{"/evil\x00.dtd", "", false},
	}

	for _, tc := range tests {
		got, err := NormalizePath(tc.in)
		if tc.valid {
			if err != nil {
				t.Errorf("NormalizePath(%q): unexpected error: %v", tc.in, err)
				continue
			}
			if got != tc.want {
				t.Errorf("NormalizePath(%q) = %q, want %q", tc.in, got, tc.want)
			}
		} else if err == nil {
			t.Errorf("NormalizePath(%q) = %q, want error", tc.in, got)
		}
	}
}

func TestContentTypeFromPath(t *testing.T) {
	if got := ContentTypeFromPath("/evil.dtd"); got != "application/xml-dtd" {
		t.Fatalf("dtd: got %q", got)
	}
	if got := ContentTypeFromPath("/x.xml"); got != "application/xml" {
		t.Fatalf("xml: got %q", got)
	}
}
