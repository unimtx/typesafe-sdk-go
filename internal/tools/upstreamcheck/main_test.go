package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePins(t *testing.T) {
	document := "| Role | Repository | Tag | Commit |\n" +
		"|---|---|---|---|\n" +
		"| Reference | [typesafe-ai/typesafe-sdk-js](https://github.com/typesafe-ai/typesafe-sdk-js) | `v0.6.0` | `abc` |\n" +
		"| Cross-check | [typesafe-ai/typesafe-sdk-python](https://github.com/typesafe-ai/typesafe-sdk-python) | `v0.7.0` | `def` |\n"
	pins, err := parsePins(document)
	if err != nil {
		t.Fatal(err)
	}
	if len(pins) != 2 || pins[0].repository != "typesafe-ai/typesafe-sdk-js" || pins[1].version != "v0.7.0" {
		t.Fatalf("pins=%+v", pins)
	}
}

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		left  string
		right string
		want  int
	}{
		{"v0.7.0", "v0.6.0", 1},
		{"v1.0.0", "v0.99.99", 1},
		{"v1.2.3", "v1.2.3", 0},
		{"v1.2.2", "v1.2.3", -1},
	}
	for _, test := range tests {
		got, err := compareSemver(test.left, test.right)
		if err != nil || got != test.want {
			t.Errorf("compareSemver(%q, %q)=%d, %v; want %d", test.left, test.right, got, err, test.want)
		}
	}
	if _, err := compareSemver("v1.0.0-rc.1", "v1.0.0"); err == nil {
		t.Fatal("accepted a prerelease")
	}
}

func TestRunWritesOnlyNewerReleases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/typesafe-ai/typesafe-sdk-js/releases":
			_, _ = w.Write([]byte(`[
				{"tag_name":"v0.7.0-rc.1","html_url":"https://example.test/js/v0.7.0-rc.1","prerelease":true},
				{"tag_name":"v0.6.1","html_url":"https://example.test/js/v0.6.1"},
				{"tag_name":"v0.7.0","html_url":"https://example.test/js/v0.7.0"}
			]`))
		case "/repos/typesafe-ai/typesafe-sdk-python/releases":
			_, _ = w.Write([]byte(`[{"tag_name":"v0.7.0","html_url":"https://example.test/python/v0.7.0"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	upstream := filepath.Join(dir, "UPSTREAM.md")
	report := filepath.Join(dir, "report.md")
	document := "| [typesafe-ai/typesafe-sdk-js](https://github.com/typesafe-ai/typesafe-sdk-js) | `v0.6.0` |\n" +
		"| [typesafe-ai/typesafe-sdk-python](https://github.com/typesafe-ai/typesafe-sdk-python) | `v0.7.0` |\n"
	if err := os.WriteFile(upstream, []byte(document), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), upstream, report, server.URL, server.Client(), ""); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "typesafe-sdk-js") || strings.Contains(text, "typesafe-sdk-python") {
		t.Fatalf("report=%s", text)
	}
}
