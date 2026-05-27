package views

import (
	"bytes"
	"context"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/AndriGitDev/meil/internal/domains"
	"github.com/AndriGitDev/meil/internal/scanner"
)

type renderable interface {
	Render(context.Context, io.Writer) error
}

func TestRenderedViewsKeepStaticAccessibilityContracts(t *testing.T) {
	t.Parallel()

	pages := map[string]string{
		"login":   renderView(t, Login("csrf-token")),
		"domains": renderView(t, AppDomains("user@example.com", sampleDomains(), "csrf-token", []Flash{{Kind: "success", Message: "Protected example.com."}})),
	}
	for name, html := range pages {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(html, `<html lang="en"`) {
				t.Fatalf("missing document language")
			}
			if !strings.Contains(html, `name="viewport"`) {
				t.Fatalf("missing responsive viewport")
			}
			if strings.Contains(html, "tracking-[-") {
				t.Fatalf("negative letter spacing class should not be used")
			}
			for _, stale := range []string{"Reporter later", "will be able"} {
				if strings.Contains(html, stale) {
					t.Fatalf("stale future-facing copy %q should not appear", stale)
				}
			}
			assertTextInputsHaveLabels(t, html)
			assertFormsHaveSubmitButtons(t, html)
			assertLinksHaveNames(t, html)
		})
	}
	if !strings.Contains(pages["domains"], `role="status"`) || !strings.Contains(pages["domains"], `aria-live="polite"`) {
		t.Fatalf("domains page should expose flash messages as a polite live region")
	}
}

func renderView(t *testing.T, component renderable) string {
	t.Helper()
	var buf bytes.Buffer
	if err := component.Render(context.Background(), &buf); err != nil {
		t.Fatalf("rendering component: %v", err)
	}
	return buf.String()
}

func assertTextInputsHaveLabels(t *testing.T, html string) {
	t.Helper()
	labels := map[string]bool{}
	for _, match := range labelForPattern.FindAllStringSubmatch(html, -1) {
		labels[match[1]] = true
	}
	for _, match := range inputPattern.FindAllStringSubmatch(html, -1) {
		inputType := attributeValue(match[0], "type")
		if inputType != "text" && inputType != "email" {
			continue
		}
		id := attributeValue(match[0], "id")
		if id == "" {
			t.Fatalf("text/email input missing id: %s", match[0])
		}
		if !labels[id] {
			t.Fatalf("text/email input %q missing matching label", id)
		}
	}
}

func assertFormsHaveSubmitButtons(t *testing.T, html string) {
	t.Helper()
	for _, match := range formPattern.FindAllStringSubmatch(html, -1) {
		if !strings.Contains(match[1], "<button") {
			t.Fatalf("form missing submit button: %s", match[0])
		}
	}
}

func assertLinksHaveNames(t *testing.T, html string) {
	t.Helper()
	for _, match := range anchorPattern.FindAllStringSubmatch(html, -1) {
		opening := match[1]
		text := strings.TrimSpace(stripTags(match[2]))
		if text == "" && attributeValue(opening, "aria-label") == "" {
			t.Fatalf("link missing accessible name: %s", match[0])
		}
	}
}

func attributeValue(tag string, name string) string {
	pattern := regexp.MustCompile(name + `="([^"]*)"`)
	match := pattern.FindStringSubmatch(tag)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func stripTags(input string) string {
	return strings.TrimSpace(tagPattern.ReplaceAllString(input, ""))
}

func sampleDomains() []domains.Domain {
	return []domains.Domain{
		{
			ID:        "domain-1",
			Domain:    "example.com",
			CreatedAt: time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
			LatestCheck: &domains.Check{
				CheckedAt: time.Date(2026, 5, 16, 12, 5, 0, 0, time.UTC),
				SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass, Message: "SPF passes."},
				DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.StatusPass, Message: "DKIM passes."},
				DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.StatusWarn, Message: "DMARC is monitoring."},
				BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing, Message: "BIMI is missing."},
			},
		},
	}
}

var (
	labelForPattern = regexp.MustCompile(`<label\b[^>]*\bfor="([^"]+)"[^>]*>`)
	inputPattern    = regexp.MustCompile(`<input\b[^>]*>`)
	formPattern     = regexp.MustCompile(`(?s)<form\b[^>]*>(.*?)</form>`)
	anchorPattern   = regexp.MustCompile(`(?s)<a\b([^>]*)>(.*?)</a>`)
	tagPattern      = regexp.MustCompile(`<[^>]+>`)
)
