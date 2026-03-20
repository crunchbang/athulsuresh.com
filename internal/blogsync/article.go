package blogsync

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Article struct {
	Meta ArticleMeta
	Body string
}

type ArticleMeta struct {
	ID                               string
	Title                            string
	Author                           []string
	Date                             string
	Slug                             string
	Kind                             string
	BookStatus                       string
	Tags                             []string
	Draft                            bool
	Summary                          string
	OriginalSource                   string
	OriginalURL                      string
	OriginalDate                     string
	BookAuthor                       string
	GoodreadsBookID                  string
	GoodreadsRating                  string
	GoodreadsExclusiveShelf          string
	GoodreadsDateRead                string
	GoodreadsDateAdded               string
	GoodreadsISBN                    string
	GoodreadsISBN13                  string
	GoodreadsPublisher               string
	GoodreadsBinding                 string
	GoodreadsPages                   string
	GoodreadsPublicationYear         string
	GoodreadsOriginalPublicationYear string
}

var (
	errNoFrontMatter  = errors.New("missing front matter")
	numericPrefixExpr = regexp.MustCompile(`^\d+-`)
	mdLinkExpr        = regexp.MustCompile(`\[(.*?)\]\((.*?)\)`)
	orgLinkExpr       = regexp.MustCompile(`\[\[(.*?)\]\[(.*?)\]\]`)
	orgHeadingExpr    = regexp.MustCompile(`(?m)^\*+\s+`)
	punctExpr         = regexp.MustCompile(`[^a-z0-9]+`)
)

func ImportLegacy(root string) error {
	articlesDir := filepath.Join(root, "articles")
	if err := os.MkdirAll(articlesDir, 0o755); err != nil {
		return err
	}

	legacyFiles := []string{}
	for _, dir := range []string{
		filepath.Join(root, "content", "posts"),
		filepath.Join(root, "content", "books"),
	} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
				continue
			}
			legacyFiles = append(legacyFiles, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Strings(legacyFiles)

	for _, legacyFile := range legacyFiles {
		article, err := readLegacyArticle(legacyFile)
		if err != nil {
			return fmt.Errorf("read legacy article %s: %w", legacyFile, err)
		}
		targetDir := filepath.Join(articlesDir, article.Meta.ID)
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return err
		}
		targetFile := filepath.Join(targetDir, "index.md")
		if err := os.WriteFile(targetFile, []byte(renderSourceArticle(article)), 0o644); err != nil {
			return err
		}
	}

	return nil
}

func GenerateHugoContent(root string) error {
	articles, err := loadArticles(root)
	if err != nil {
		return err
	}
	if err := validateArticles(articles); err != nil {
		return err
	}
	bookRecords, err := loadBookRecords(root)
	if err != nil {
		return err
	}
	if err := validateBookRecords(bookRecords); err != nil {
		return err
	}

	outputDir := filepath.Join(root, "content", "posts")
	if err := os.RemoveAll(outputDir); err != nil {
		return err
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}

	for _, article := range articles {
		target := filepath.Join(outputDir, article.Meta.ID+".md")
		if err := os.WriteFile(target, []byte(renderHugoArticle(article)), 0o644); err != nil {
			return err
		}
	}

	booksDir := filepath.Join(root, "content", "books")
	if err := os.RemoveAll(booksDir); err != nil {
		return err
	}
	if err := writeRatedBooksData(root, bookRecords); err != nil {
		return err
	}

	return nil
}

func loadArticles(root string) ([]Article, error) {
	entries, err := os.ReadDir(filepath.Join(root, "articles"))
	if err != nil {
		return nil, err
	}

	var articles []Article
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		indexPath := filepath.Join(root, "articles", entry.Name(), "index.md")
		article, err := parseArticleFile(indexPath)
		if err != nil {
			return nil, err
		}
		articles = append(articles, article)
	}
	sort.Slice(articles, func(i, j int) bool {
		if articles[i].Meta.Date == articles[j].Meta.Date {
			return articles[i].Meta.Slug < articles[j].Meta.Slug
		}
		return articles[i].Meta.Date < articles[j].Meta.Date
	})
	return articles, nil
}

func parseArticleFile(path string) (Article, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Article{}, err
	}
	return parseFrontMatterMarkdown(string(data))
}

func parseFrontMatterMarkdown(input string) (Article, error) {
	if !strings.HasPrefix(input, "+++\n") {
		return Article{}, errNoFrontMatter
	}
	parts := strings.SplitN(input, "\n+++\n", 2)
	if len(parts) != 2 {
		return Article{}, errNoFrontMatter
	}

	meta, err := parseSimpleTOML(strings.TrimPrefix(parts[0], "+++\n"))
	if err != nil {
		return Article{}, err
	}
	body := strings.TrimLeft(parts[1], "\n")
	return Article{Meta: meta, Body: strings.TrimRight(body, "\n") + "\n"}, nil
}

func parseSimpleTOML(frontMatter string) (ArticleMeta, error) {
	var meta ArticleMeta
	lines := strings.Split(frontMatter, "\n")
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return ArticleMeta{}, fmt.Errorf("invalid front matter line %q", rawLine)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "id":
			meta.ID = trimQuotes(value)
		case "title":
			meta.Title = trimQuotes(value)
		case "author":
			meta.Author = parseArray(value)
		case "date":
			meta.Date = trimQuotes(value)
		case "slug":
			meta.Slug = trimQuotes(value)
		case "kind", "article_kind":
			meta.Kind = trimQuotes(value)
		case "book_status":
			meta.BookStatus = trimQuotes(value)
		case "tags":
			meta.Tags = parseArray(value)
		case "draft":
			meta.Draft = value == "true"
		case "summary":
			meta.Summary = trimQuotes(value)
		case "original_source":
			meta.OriginalSource = trimQuotes(value)
		case "original_url":
			meta.OriginalURL = trimQuotes(value)
		case "original_date":
			meta.OriginalDate = trimQuotes(value)
		case "book_author":
			meta.BookAuthor = trimQuotes(value)
		case "goodreads_book_id":
			meta.GoodreadsBookID = trimQuotes(value)
		case "goodreads_rating":
			meta.GoodreadsRating = trimQuotes(value)
		case "goodreads_exclusive_shelf":
			meta.GoodreadsExclusiveShelf = trimQuotes(value)
		case "goodreads_date_read":
			meta.GoodreadsDateRead = trimQuotes(value)
		case "goodreads_date_added":
			meta.GoodreadsDateAdded = trimQuotes(value)
		case "goodreads_isbn":
			meta.GoodreadsISBN = trimQuotes(value)
		case "goodreads_isbn13":
			meta.GoodreadsISBN13 = trimQuotes(value)
		case "goodreads_publisher":
			meta.GoodreadsPublisher = trimQuotes(value)
		case "goodreads_binding":
			meta.GoodreadsBinding = trimQuotes(value)
		case "goodreads_pages":
			meta.GoodreadsPages = trimQuotes(value)
		case "goodreads_publication_year":
			meta.GoodreadsPublicationYear = trimQuotes(value)
		case "goodreads_original_publication_year":
			meta.GoodreadsOriginalPublicationYear = trimQuotes(value)
		}
	}
	if meta.Author == nil {
		meta.Author = []string{}
	}
	if meta.Tags == nil {
		meta.Tags = []string{}
	}
	return meta, nil
}

func renderSourceArticle(article Article) string {
	var buf bytes.Buffer
	buf.WriteString("+++\n")
	writeArticleMeta(&buf, article.Meta)
	buf.WriteString("+++\n\n")
	buf.WriteString(strings.TrimRight(article.Body, "\n"))
	buf.WriteString("\n")
	return buf.String()
}

func renderSourceBook(book Article) string {
	var buf bytes.Buffer
	buf.WriteString("+++\n")
	writeBookMeta(&buf, book.Meta)
	buf.WriteString("+++\n")
	if strings.TrimSpace(book.Body) != "" {
		buf.WriteString("\n")
		buf.WriteString(strings.TrimRight(book.Body, "\n"))
		buf.WriteString("\n")
	}
	return buf.String()
}

func renderHugoArticle(article Article) string {
	var buf bytes.Buffer
	buf.WriteString("+++\n")
	buf.WriteString("# This file is generated by `go run ./cmd/blogsync sync`.\n")
	writeHugoMeta(&buf, article.Meta)
	buf.WriteString("+++\n\n")
	buf.WriteString(strings.TrimRight(article.Body, "\n"))
	buf.WriteString("\n")
	return buf.String()
}

func readLegacyArticle(path string) (Article, error) {
	article, err := parseArticleFile(path)
	if err != nil {
		return Article{}, err
	}

	filename := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	slug := numericPrefixExpr.ReplaceAllString(filename, "")
	kind := "essay"
	if strings.Contains(path, string(filepath.Separator)+"books"+string(filepath.Separator)) {
		kind = "review"
	}

	article.Meta.ID = slug
	article.Meta.Slug = slug
	article.Meta.Kind = kind
	if len(article.Meta.Author) == 0 {
		article.Meta.Author = []string{"Athul Suresh"}
	}
	return article, nil
}

func validateArticles(articles []Article) error {
	ids := map[string]string{}
	slugs := map[string]string{}
	for _, article := range articles {
		if article.Meta.ID == "" || article.Meta.Title == "" || article.Meta.Date == "" || article.Meta.Slug == "" {
			return fmt.Errorf("article %q is missing required metadata", article.Meta.Title)
		}
		if article.Meta.Kind == "" {
			return fmt.Errorf("article %q is missing kind", article.Meta.ID)
		}
		if article.Meta.Kind != "essay" && article.Meta.Kind != "review" {
			return fmt.Errorf("article %q has unsupported article_kind %q", article.Meta.ID, article.Meta.Kind)
		}
		if previous, ok := ids[article.Meta.ID]; ok {
			return fmt.Errorf("duplicate article id %q in %s and %s", article.Meta.ID, previous, article.Meta.Title)
		}
		if previous, ok := slugs[article.Meta.Slug]; ok {
			return fmt.Errorf("duplicate article slug %q in %s and %s", article.Meta.Slug, previous, article.Meta.Title)
		}
		ids[article.Meta.ID] = article.Meta.Title
		slugs[article.Meta.Slug] = article.Meta.Title
	}
	return nil
}

func normalizeMarkdown(input string) string {
	text := orgLinkExpr.ReplaceAllString(input, "$2")
	text = mdLinkExpr.ReplaceAllString(text, "$1")
	text = orgHeadingExpr.ReplaceAllString(text, "")
	replacer := strings.NewReplacer(
		"`", " ",
		"*", " ",
		"_", " ",
		"~", " ",
		"#", " ",
	)
	text = replacer.Replace(text)
	text = html.UnescapeString(text)
	text = strings.ToLower(text)
	text = punctExpr.ReplaceAllString(text, " ")
	return strings.Join(strings.Fields(text), " ")
}

func trimQuotes(value string) string {
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func parseArray(value string) []string {
	value = strings.TrimSpace(value)
	if value == "[]" {
		return []string{}
	}
	value = strings.TrimPrefix(value, "[")
	value = strings.TrimSuffix(value, "]")
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		items = append(items, trimQuotes(strings.TrimSpace(part)))
	}
	return items
}

func writeStringField(buf *bytes.Buffer, key, value string) {
	fmt.Fprintf(buf, "%s = %q\n", key, value)
}

func writeArrayField(buf *bytes.Buffer, key string, values []string) {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, fmt.Sprintf("%q", value))
	}
	fmt.Fprintf(buf, "%s = [%s]\n", key, strings.Join(quoted, ", "))
}

func writeBoolField(buf *bytes.Buffer, key string, value bool) {
	fmt.Fprintf(buf, "%s = %t\n", key, value)
}

func writeArticleMeta(buf *bytes.Buffer, meta ArticleMeta) {
	writeStringField(buf, "id", meta.ID)
	writeStringField(buf, "title", meta.Title)
	if len(meta.Author) > 0 {
		writeArrayField(buf, "author", meta.Author)
	}
	writeStringField(buf, "date", meta.Date)
	writeStringField(buf, "slug", meta.Slug)
	writeStringField(buf, "article_kind", meta.Kind)
	if len(meta.Tags) > 0 {
		writeArrayField(buf, "tags", meta.Tags)
	}
	writeBoolField(buf, "draft", meta.Draft)
	writeOptionalStringField(buf, "summary", meta.Summary)
	writeOptionalStringField(buf, "original_source", meta.OriginalSource)
	writeOptionalStringField(buf, "original_url", meta.OriginalURL)
	writeOptionalStringField(buf, "original_date", meta.OriginalDate)
	writeOptionalStringField(buf, "book_author", meta.BookAuthor)
	writeOptionalStringField(buf, "goodreads_book_id", meta.GoodreadsBookID)
	writeOptionalStringField(buf, "goodreads_rating", meta.GoodreadsRating)
	writeOptionalStringField(buf, "goodreads_exclusive_shelf", meta.GoodreadsExclusiveShelf)
	writeOptionalStringField(buf, "goodreads_date_read", meta.GoodreadsDateRead)
	writeOptionalStringField(buf, "goodreads_date_added", meta.GoodreadsDateAdded)
	writeOptionalStringField(buf, "goodreads_isbn", meta.GoodreadsISBN)
	writeOptionalStringField(buf, "goodreads_isbn13", meta.GoodreadsISBN13)
	writeOptionalStringField(buf, "goodreads_publisher", meta.GoodreadsPublisher)
	writeOptionalStringField(buf, "goodreads_binding", meta.GoodreadsBinding)
	writeOptionalStringField(buf, "goodreads_pages", meta.GoodreadsPages)
	writeOptionalStringField(buf, "goodreads_publication_year", meta.GoodreadsPublicationYear)
	writeOptionalStringField(buf, "goodreads_original_publication_year", meta.GoodreadsOriginalPublicationYear)
}

func writeBookMeta(buf *bytes.Buffer, meta ArticleMeta) {
	writeStringField(buf, "id", meta.ID)
	writeStringField(buf, "title", meta.Title)
	writeStringField(buf, "date", meta.Date)
	writeStringField(buf, "slug", meta.Slug)
	writeStringField(buf, "book_status", meta.BookStatus)
	writeOptionalStringField(buf, "original_source", meta.OriginalSource)
	writeOptionalStringField(buf, "book_author", meta.BookAuthor)
	writeOptionalStringField(buf, "goodreads_book_id", meta.GoodreadsBookID)
	writeOptionalStringField(buf, "goodreads_rating", meta.GoodreadsRating)
	writeOptionalStringField(buf, "goodreads_exclusive_shelf", meta.GoodreadsExclusiveShelf)
	writeOptionalStringField(buf, "goodreads_date_read", meta.GoodreadsDateRead)
	writeOptionalStringField(buf, "goodreads_date_added", meta.GoodreadsDateAdded)
	writeOptionalStringField(buf, "goodreads_isbn", meta.GoodreadsISBN)
	writeOptionalStringField(buf, "goodreads_isbn13", meta.GoodreadsISBN13)
	writeOptionalStringField(buf, "goodreads_publisher", meta.GoodreadsPublisher)
	writeOptionalStringField(buf, "goodreads_binding", meta.GoodreadsBinding)
	writeOptionalStringField(buf, "goodreads_pages", meta.GoodreadsPages)
	writeOptionalStringField(buf, "goodreads_publication_year", meta.GoodreadsPublicationYear)
	writeOptionalStringField(buf, "goodreads_original_publication_year", meta.GoodreadsOriginalPublicationYear)
}

func writeHugoMeta(buf *bytes.Buffer, meta ArticleMeta) {
	hugoMeta := meta
	hugoMeta.Author = defaultAuthor(meta.Author)
	writeArticleMeta(buf, hugoMeta)
}

func writeOptionalStringField(buf *bytes.Buffer, key, value string) {
	if value == "" {
		return
	}
	writeStringField(buf, key, value)
}

func defaultAuthor(author []string) []string {
	if len(author) == 0 {
		return []string{"Athul Suresh"}
	}
	return author
}

type ratedBookData struct {
	ID                               string `json:"id"`
	Title                            string `json:"title"`
	Date                             string `json:"date"`
	Slug                             string `json:"slug"`
	BookStatus                       string `json:"book_status"`
	OriginalSource                   string `json:"original_source,omitempty"`
	BookAuthor                       string `json:"book_author,omitempty"`
	GoodreadsBookID                  string `json:"goodreads_book_id,omitempty"`
	GoodreadsRating                  string `json:"goodreads_rating,omitempty"`
	GoodreadsExclusiveShelf          string `json:"goodreads_exclusive_shelf,omitempty"`
	GoodreadsDateRead                string `json:"goodreads_date_read,omitempty"`
	GoodreadsDateAdded               string `json:"goodreads_date_added,omitempty"`
	GoodreadsISBN                    string `json:"goodreads_isbn,omitempty"`
	GoodreadsISBN13                  string `json:"goodreads_isbn13,omitempty"`
	GoodreadsPublisher               string `json:"goodreads_publisher,omitempty"`
	GoodreadsBinding                 string `json:"goodreads_binding,omitempty"`
	GoodreadsPages                   string `json:"goodreads_pages,omitempty"`
	GoodreadsPublicationYear         string `json:"goodreads_publication_year,omitempty"`
	GoodreadsOriginalPublicationYear string `json:"goodreads_original_publication_year,omitempty"`
}

func writeRatedBooksData(root string, books []Article) error {
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}

	target := filepath.Join(dataDir, "rated_books.json")
	if len(books) == 0 {
		if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}

	payload := make([]ratedBookData, 0, len(books))
	for _, book := range books {
		payload = append(payload, ratedBookData{
			ID:                               book.Meta.ID,
			Title:                            book.Meta.Title,
			Date:                             book.Meta.Date,
			Slug:                             book.Meta.Slug,
			BookStatus:                       book.Meta.BookStatus,
			OriginalSource:                   book.Meta.OriginalSource,
			BookAuthor:                       book.Meta.BookAuthor,
			GoodreadsBookID:                  book.Meta.GoodreadsBookID,
			GoodreadsRating:                  book.Meta.GoodreadsRating,
			GoodreadsExclusiveShelf:          book.Meta.GoodreadsExclusiveShelf,
			GoodreadsDateRead:                book.Meta.GoodreadsDateRead,
			GoodreadsDateAdded:               book.Meta.GoodreadsDateAdded,
			GoodreadsISBN:                    book.Meta.GoodreadsISBN,
			GoodreadsISBN13:                  book.Meta.GoodreadsISBN13,
			GoodreadsPublisher:               book.Meta.GoodreadsPublisher,
			GoodreadsBinding:                 book.Meta.GoodreadsBinding,
			GoodreadsPages:                   book.Meta.GoodreadsPages,
			GoodreadsPublicationYear:         book.Meta.GoodreadsPublicationYear,
			GoodreadsOriginalPublicationYear: book.Meta.GoodreadsOriginalPublicationYear,
		})
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(target, data, 0o644)
}
