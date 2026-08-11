package server

import (
	"strconv"
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
		n, err := strconv.Atoi(part)
		if err != nil {
			t.Errorf("VERSION = %q; component %d (%q) is not a number", VERSION, i, part)
			continue
		}
		if n < 0 {
			t.Errorf("VERSION = %q; component %d (%d) is negative", VERSION, i, n)
		}
	}
}
