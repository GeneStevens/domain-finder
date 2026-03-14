package zonefile

import "testing"

func TestNormalizeDomain(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{input: "EXAMPLE.NET.", want: "example.net"},
		{input: " mixed.example.NET ", want: "mixed.example.net"},
		{input: ".", want: ""},
		{input: "", want: ""},
	}

	for _, tc := range cases {
		if got := NormalizeDomain(tc.input); got != tc.want {
			t.Errorf("NormalizeDomain(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestParseLine(t *testing.T) {
	cases := []struct {
		name   string
		line   string
		want   string
		wantOK bool
	}{
		{
			name:   "standard record",
			line:   "example.net. 300 IN NS ns1.example.net.",
			want:   "example.net",
			wantOK: true,
		},
		{
			name:   "mixed case",
			line:   "UPPERCASE.NET.\t300\tIN\tNS\tns1.example.net.",
			want:   "uppercase.net",
			wantOK: true,
		},
		{
			name:   "blank line",
			line:   "   ",
			want:   "",
			wantOK: false,
		},
		{
			name:   "comment line",
			line:   "; zone comment",
			want:   "",
			wantOK: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record, ok := ParseLine(tc.line)
			if ok != tc.wantOK {
				t.Fatalf("ParseLine(%q) ok = %v, want %v", tc.line, ok, tc.wantOK)
			}
			if record.Domain != tc.want {
				t.Fatalf("ParseLine(%q) domain = %q, want %q", tc.line, record.Domain, tc.want)
			}
		})
	}
}
