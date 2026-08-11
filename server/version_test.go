package server

import (
	"strings"
	"testing"
)

// VERSION is reported to every MCP client in the initialize handshake (see
// serverInfo in mcp.go) and printed by --version, so a malformed value is
// visible to users. The release job additionally checks it against the pushed
// git tag; this test covers the shape, which is cheap to get wrong when
// `make set-version` is run by hand.
func TestVersionIsWellFormed(t *testing.T) {
	if VERSION == "" {
		t.Fatal("VERSION is empty")
	}

	if strings.HasPrefix(VERSION, "v") {
		t.Errorf("VERSION = %q; the constant holds the bare version, the 'v' prefix belongs to the git tag only", VERSION)
	}

	parts := strings.Split(VERSION, ".")
	if len(parts) != 3 {
		t.Fatalf("VERSION = %q; want three dot-separated components (major.minor.patch)", VERSION)
	}

	for i, part := range parts {
		if !isNumericIdentifier(part) {
			t.Errorf("VERSION = %q; component %d (%q) is not a SemVer numeric identifier", VERSION, i, part)
		}
	}
}

// TestIsNumericIdentifier pins down the cases strconv.Atoi would wave through:
// a sign or a leading zero makes a component invalid SemVer even though it
// converts to an integer cleanly.
func TestIsNumericIdentifier(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"0", true},
		{"1", true},
		{"10", true},
		{"100", true},
		{"01", false},  // leading zero
		{"007", false}, // leading zeros
		{"00", false},  // leading zero, not the bare "0"
		{"+2", false},  // sign; strconv.Atoi accepts this
		{"-3", false},  // sign; strconv.Atoi accepts this
		{"", false},    // empty component, e.g. "1..3"
		{" 1", false},  // whitespace
		{"1a", false},  // trailing junk
		{"1_0", false}, // underscore
		{"१", false},   // non-ASCII digit
		{"1.2", false}, // not a single component
	}

	for _, tt := range tests {
		if got := isNumericIdentifier(tt.in); got != tt.want {
			t.Errorf("isNumericIdentifier(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// isNumericIdentifier reports whether s is a SemVer numeric identifier: one or
// more ASCII digits, with no sign and no leading zero unless s is exactly "0".
// strconv.Atoi is not enough here, since it accepts "+2", "-3" and "01".
func isNumericIdentifier(s string) bool {
	if s == "" {
		return false
	}

	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	// Leading zeros are only allowed when the identifier is exactly "0"
	return s == "0" || s[0] != '0'
}
