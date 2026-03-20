package blogsync

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type OrgEntry struct {
	Title      string
	Tags       []string
	Date       string
	ExportName string
	Section    string
	Body       string
}

var (
	orgTitleTagsExpr = regexp.MustCompile(`^(:[[:alnum:]_-]+)+:$`)
	orgHeadingLine   = regexp.MustCompile(`^(\*+)\s+(.*)$`)
)

func VerifyOrgParity(root string) error {
	orgEntries, err := parseOrgEntries(filepath.Join(root, "content-org", "all-posts.org"))
	if err != nil {
		return err
	}
	articles, err := loadArticles(root)
	if err != nil {
		return err
	}
	if err := validateArticles(articles); err != nil {
		return err
	}

	articleBySlug := map[string]Article{}
	for _, article := range articles {
		articleBySlug[article.Meta.Slug] = article
	}

	if len(orgEntries) != len(articleBySlug) {
		return fmt.Errorf("org/article count mismatch: %d org entries, %d articles", len(orgEntries), len(articleBySlug))
	}

	var mismatches []string
	for _, entry := range orgEntries {
		slug := numericPrefixExpr.ReplaceAllString(entry.ExportName, "")
		article, ok := articleBySlug[slug]
		if !ok {
			mismatches = append(mismatches, fmt.Sprintf("missing article for org entry %q (%s)", entry.Title, slug))
			continue
		}
		if normalizeMarkdown(article.Meta.Title) != normalizeMarkdown(entry.Title) {
			mismatches = append(mismatches, fmt.Sprintf("title mismatch for %s: org=%q article=%q", slug, entry.Title, article.Meta.Title))
		}
		if article.Meta.Date != entry.Date {
			mismatches = append(mismatches, fmt.Sprintf("date mismatch for %s: org=%q article=%q", slug, entry.Date, article.Meta.Date))
		}
		expectedKind := "essay"
		if entry.Section == "books" {
			expectedKind = "review"
		}
		if article.Meta.Kind != expectedKind {
			mismatches = append(mismatches, fmt.Sprintf("kind mismatch for %s: org=%q article=%q", slug, expectedKind, article.Meta.Kind))
		}
		if !similarBody(entry.Body, article.Body) {
			mismatches = append(mismatches, fmt.Sprintf("body mismatch for %s", slug))
		}
	}

	if len(mismatches) > 0 {
		sort.Strings(mismatches)
		return fmt.Errorf("org parity failed:\n%s", strings.Join(mismatches, "\n"))
	}
	return nil
}

func similarBody(orgBody, articleBody string) bool {
	orgTokens := tokenSet(normalizeMarkdown(orgBody))
	articleTokens := tokenSet(normalizeMarkdown(articleBody))
	if len(orgTokens) == 0 || len(articleTokens) == 0 {
		return len(orgTokens) == len(articleTokens)
	}

	intersection := 0
	union := map[string]struct{}{}
	for token := range orgTokens {
		union[token] = struct{}{}
		if _, ok := articleTokens[token]; ok {
			intersection++
		}
	}
	for token := range articleTokens {
		union[token] = struct{}{}
	}

	similarity := float64(intersection) / float64(len(union))
	return similarity >= 0.97
}

func tokenSet(input string) map[string]struct{} {
	tokens := map[string]struct{}{}
	for _, token := range strings.Fields(input) {
		tokens[token] = struct{}{}
	}
	return tokens
}

func parseOrgEntries(path string) ([]OrgEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	var entries []OrgEntry
	var current *OrgEntry
	var topLevel string
	inProperties := false

	finalize := func() {
		if current == nil {
			return
		}
		current.Body = strings.TrimSpace(current.Body)
		entries = append(entries, *current)
		current = nil
	}

	for _, line := range lines {
		if match := orgHeadingLine.FindStringSubmatch(line); match != nil {
			level := len(match[1])
			title, tags := parseOrgHeadingTitle(match[2])
			switch {
			case level == 1 && title == "Books":
				topLevel = "Books"
				finalize()
				continue
			case level == 1:
				topLevel = title
				finalize()
				current = &OrgEntry{Title: title, Tags: tags, Section: "posts"}
				inProperties = false
				continue
			case level == 2 && topLevel == "Books":
				finalize()
				current = &OrgEntry{Title: title, Tags: tags, Section: "books"}
				inProperties = false
				continue
			}
		}

		if current == nil {
			continue
		}
		switch strings.TrimSpace(line) {
		case ":PROPERTIES:":
			inProperties = true
			continue
		case ":END:":
			inProperties = false
			continue
		}
		if inProperties {
			parseOrgProperty(line, current)
			continue
		}
		current.Body += line + "\n"
	}
	finalize()
	return entries, nil
}

func parseOrgHeadingTitle(raw string) (string, []string) {
	raw = strings.TrimSpace(raw)
	lastSpace := strings.LastIndex(raw, " ")
	if lastSpace == -1 {
		return raw, nil
	}
	tagBlock := strings.TrimSpace(raw[lastSpace+1:])
	if !orgTitleTagsExpr.MatchString(tagBlock) {
		return raw, nil
	}
	title := strings.TrimSpace(raw[:lastSpace])
	tags := strings.Split(strings.Trim(tagBlock, ":"), ":")
	return title, tags
}

func parseOrgProperty(line string, entry *OrgEntry) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, ":") {
		return
	}
	parts := strings.SplitN(strings.TrimPrefix(line, ":"), ":", 2)
	if len(parts) != 2 {
		return
	}
	key := parts[0]
	value := strings.TrimSpace(parts[1])
	switch key {
	case "EXPORT_DATE":
		entry.Date = value
	case "EXPORT_FILE_NAME":
		entry.ExportName = value
	case "EXPORT_HUGO_SECTION":
		entry.Section = value
	}
}
