package typesafe_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestPublicAPIScopeCompilation(t *testing.T) {
	positive := exec.Command("go", "test", "./testdata/compile/positive")
	if output, err := positive.CombinedOutput(); err != nil {
		t.Fatalf("positive API sample failed: %v\n%s", err, output)
	}
	negative := []struct{ path, fragment string }{
		{"./testdata/compile/client_rejects_request", "does not implement config.ClientOption"},
		{"./testdata/compile/request_rejects_client", "does not implement config.RequestOption"},
		{"./testdata/compile/score_requires_explicit_entry", "cannot use"},
		{"./testdata/compile/removed_choice_option", "undefined: typesafe.Option"},
	}
	for _, tc := range negative {
		cmd := exec.Command("go", "test", tc.path)
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Errorf("%s unexpectedly compiled", tc.path)
			continue
		}
		if !strings.Contains(string(output), tc.fragment) {
			t.Errorf("%s failed without %q:\n%s", tc.path, tc.fragment, output)
		}
	}
}
