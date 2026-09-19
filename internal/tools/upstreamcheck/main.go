// Command upstreamcheck reports stable upstream SDK releases newer than the
// exact versions pinned in UPSTREAM.md.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var pinPattern = regexp.MustCompile(`\[([^]]+/[^]]+)\]\(https://github\.com/[^)]+\)\s*\|\s*` + "`" + `(v[^` + "`" + `]+)` + "`")

type pin struct {
	repository string
	version    string
}

type release struct {
	TagName    string `json:"tag_name"`
	HTMLURL    string `json:"html_url"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

type update struct {
	pin
	latest release
}

func main() {
	var upstreamPath string
	var reportPath string
	var apiURL string
	flag.StringVar(&upstreamPath, "upstream", "UPSTREAM.md", "path to the upstream pin document")
	flag.StringVar(&reportPath, "report", "", "write a Markdown update report to this path")
	flag.StringVar(&apiURL, "api-url", "https://api.github.com", "GitHub API root")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := run(ctx, upstreamPath, reportPath, apiURL, http.DefaultClient, os.Getenv("GITHUB_TOKEN")); err != nil {
		fmt.Fprintln(os.Stderr, "upstreamcheck:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, upstreamPath, reportPath, apiURL string, client *http.Client, token string) error {
	data, err := os.ReadFile(upstreamPath)
	if err != nil {
		return err
	}
	pins, err := parsePins(string(data))
	if err != nil {
		return err
	}

	var updates []update
	for _, pinned := range pins {
		latest, err := latestRelease(ctx, client, apiURL, token, pinned.repository)
		if err != nil {
			return err
		}
		comparison, err := compareSemver(latest.TagName, pinned.version)
		if err != nil {
			return fmt.Errorf("compare %s releases: %w", pinned.repository, err)
		}
		if comparison > 0 {
			updates = append(updates, update{pin: pinned, latest: latest})
		}
	}

	if reportPath == "" {
		for _, available := range updates {
			fmt.Printf("%s: %s -> %s (%s)\n", available.repository, available.version, available.latest.TagName, available.latest.HTMLURL)
		}
		return nil
	}
	if len(updates) == 0 {
		return os.WriteFile(reportPath, nil, 0o644)
	}

	var report strings.Builder
	report.WriteString("A newer stable TypeSafe SDK release is available. The workflow only reports drift; it does not update pins or publish packages.\n\n")
	report.WriteString("| Repository | Pinned | Latest stable |\n|---|---:|---:|\n")
	for _, available := range updates {
		fmt.Fprintf(&report, "| `%s` | `%s` | [%s](%s) |\n", available.repository, available.version, available.latest.TagName, available.latest.HTMLURL)
	}
	report.WriteString("\nReview the pinned-to-target source, tests, and changelog according to `DESIGN.md`; update fixtures before implementation when wire behavior changes.\n")
	return os.WriteFile(reportPath, []byte(report.String()), 0o644)
}

func parsePins(document string) ([]pin, error) {
	matches := pinPattern.FindAllStringSubmatch(document, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no GitHub release pins found")
	}
	pins := make([]pin, 0, len(matches))
	for _, match := range matches {
		if _, err := parseSemver(match[2]); err != nil {
			return nil, fmt.Errorf("invalid pinned version for %s: %w", match[1], err)
		}
		pins = append(pins, pin{repository: match[1], version: match[2]})
	}
	return pins, nil
}

func latestRelease(ctx context.Context, client *http.Client, apiURL, token, repository string) (release, error) {
	url := strings.TrimRight(apiURL, "/") + "/repos/" + repository + "/releases?per_page=100"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "typesafe-sdk-go-upstreamcheck")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := client.Do(req)
	if err != nil {
		return release{}, fmt.Errorf("fetch latest release for %s: %w", repository, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
		return release{}, fmt.Errorf("fetch latest release for %s: %s: %s", repository, res.Status, strings.TrimSpace(string(body)))
	}
	var releases []release
	if err := json.NewDecoder(res.Body).Decode(&releases); err != nil {
		return release{}, fmt.Errorf("decode latest release for %s: %w", repository, err)
	}
	var latest release
	found := false
	for _, candidate := range releases {
		if candidate.Draft || candidate.Prerelease || candidate.TagName == "" || candidate.HTMLURL == "" {
			continue
		}
		if _, err := parseSemver(candidate.TagName); err != nil {
			continue
		}
		if !found {
			latest = candidate
			found = true
			continue
		}
		comparison, err := compareSemver(candidate.TagName, latest.TagName)
		if err != nil {
			return release{}, err
		}
		if comparison > 0 {
			latest = candidate
		}
	}
	if !found {
		return release{}, fmt.Errorf("no stable semantic-version release found for %s", repository)
	}
	return latest, nil
}

type semver struct {
	major int
	minor int
	patch int
}

func parseSemver(value string) (semver, error) {
	value = strings.TrimPrefix(value, "v")
	if strings.ContainsAny(value, "+-") {
		return semver{}, fmt.Errorf("%q is not a stable semantic version", value)
	}
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return semver{}, fmt.Errorf("%q does not have major.minor.patch", value)
	}
	numbers := make([]int, 3)
	for i, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return semver{}, fmt.Errorf("%q has an invalid numeric component", value)
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return semver{}, fmt.Errorf("%q has an invalid numeric component", value)
		}
		numbers[i] = n
	}
	return semver{major: numbers[0], minor: numbers[1], patch: numbers[2]}, nil
}

func compareSemver(left, right string) (int, error) {
	a, err := parseSemver(left)
	if err != nil {
		return 0, err
	}
	b, err := parseSemver(right)
	if err != nil {
		return 0, err
	}
	av := [...]int{a.major, a.minor, a.patch}
	bv := [...]int{b.major, b.minor, b.patch}
	for i := range av {
		if av[i] < bv[i] {
			return -1, nil
		}
		if av[i] > bv[i] {
			return 1, nil
		}
	}
	return 0, nil
}
