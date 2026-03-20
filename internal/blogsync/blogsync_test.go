package blogsync

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeMarkdown(t *testing.T) {
	orgBody := `*** Background
Check it [[https://example.com][out]] if you haven't already. ~code~ and *bold* and &amp; entities.`
	mdBody := `## Background
Check it [out](https://example.com) if you haven't already. ` + "`code`" + ` and **bold** and & entities.`

	if got, want := normalizeMarkdown(orgBody), normalizeMarkdown(mdBody); got != want {
		t.Fatalf("normalized text mismatch\norg: %s\nmd:  %s", got, want)
	}
}

func TestParseOrgHeadingTitle(t *testing.T) {
	title, tags := parseOrgHeadingTitle("Hello World :life:notes:")
	if title != "Hello World" {
		t.Fatalf("unexpected title %q", title)
	}
	if len(tags) != 2 || tags[0] != "life" || tags[1] != "notes" {
		t.Fatalf("unexpected tags %#v", tags)
	}
}

func TestParseSimpleTOML(t *testing.T) {
	meta, err := parseSimpleTOML(`title = "Hello"
author = ["Athul Suresh"]
date = 2020-05-03
slug = "hello-world"
kind = "essay"
tags = ["life", "go"]
draft = false`)
	if err != nil {
		t.Fatalf("parseSimpleTOML error: %v", err)
	}
	if meta.Title != "Hello" || meta.Date != "2020-05-03" || meta.Slug != "hello-world" || meta.Kind != "essay" {
		t.Fatalf("unexpected meta %#v", meta)
	}
	if len(meta.Tags) != 2 || meta.Tags[1] != "go" {
		t.Fatalf("unexpected tags %#v", meta.Tags)
	}
}

func TestSlugifyBookTitle(t *testing.T) {
	got := slugifyBookTitle("Shōgun (Asian Saga, #1)")
	if got != "shogun" {
		t.Fatalf("unexpected slug %q", got)
	}
}

func TestConvertGoodreadsReview(t *testing.T) {
	got := convertGoodreadsReview("Hello<br/><br/>world &amp; friends")
	want := "Hello\n\nworld & friends\n"
	if got != want {
		t.Fatalf("unexpected review conversion:\n%s", got)
	}
}

func TestImportGoodreads(t *testing.T) {
	root := t.TempDir()
	csvPath := filepath.Join(root, "goodreads_export.csv")
	file, err := os.Create(csvPath)
	if err != nil {
		t.Fatalf("create csv: %v", err)
	}
	writer := csv.NewWriter(file)
	records := [][]string{
		{"Book Id", "Title", "Author", "Author l-f", "Additional Authors", "ISBN", "ISBN13", "My Rating", "Average Rating", "Publisher", "Binding", "Number of Pages", "Year Published", "Original Publication Year", "Date Read", "Date Added", "Bookshelves", "Bookshelves with positions", "Exclusive Shelf", "My Review", "Spoiler", "Private Notes", "Read Count", "Owned Copies"},
		{"1", "Shōgun (Asian Saga, #1)", "James Clavell", "", "", "", "", "5", "", "Hodder", "Paperback", "1000", "1975", "1975", "2024/03/01", "2024/03/02", "", "", "read", "Loved it.<br/><br/>Very immersive.", "", "", "1", "0"},
		{"2", "No Review Book", "Someone", "", "", "", "", "0", "", "Pub", "Paperback", "200", "2024", "2024", "", "2024/03/03", "", "", "read", "", "", "", "1", "0"},
		{"3", "Pachinko", "Min Jin Lee", "", "", "", "", "4", "", "Grand Central Publishing", "Paperback", "496", "2017", "2017", "", "2024/04/03", "", "", "read", "Excellent book.", "", "", "1", "0"},
	}
	for _, record := range records {
		if err := writer.Write(record); err != nil {
			t.Fatalf("write csv record: %v", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatalf("flush csv: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close csv: %v", err)
	}

	if err := ImportGoodreads(root, csvPath); err != nil {
		t.Fatalf("ImportGoodreads error: %v", err)
	}

	shogunPath := filepath.Join(root, "articles", "book-review-shogun", "index.md")
	article, err := parseArticleFile(shogunPath)
	if err != nil {
		t.Fatalf("parse imported article: %v", err)
	}
	if article.Meta.Title != "Book Review: Shōgun (Asian Saga, #1)" {
		t.Fatalf("unexpected title %q", article.Meta.Title)
	}
	if article.Meta.Date != "2024-03-01" {
		t.Fatalf("unexpected date %q", article.Meta.Date)
	}
	if article.Meta.Kind != "review" || article.Meta.OriginalSource != "Goodreads" {
		t.Fatalf("unexpected meta %#v", article.Meta)
	}
	if !strings.Contains(article.Body, "Loved it.") || !strings.Contains(article.Body, "Very immersive.") {
		t.Fatalf("unexpected body %q", article.Body)
	}

	pachinkoPath := filepath.Join(root, "articles", "book-review-pachinko", "index.md")
	pachinko, err := parseArticleFile(pachinkoPath)
	if err != nil {
		t.Fatalf("parse pachinko article: %v", err)
	}
	if pachinko.Meta.Date != "2024-04-03" {
		t.Fatalf("expected Date Added fallback, got %q", pachinko.Meta.Date)
	}

	if err := ImportGoodreads(root, csvPath); err == nil {
		t.Fatalf("expected re-import conflict error")
	}
}
