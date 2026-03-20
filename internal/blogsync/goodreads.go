package blogsync

import (
	"encoding/csv"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type GoodreadsRow struct {
	BookID                  string
	Title                   string
	Author                  string
	ISBN                    string
	ISBN13                  string
	MyRating                string
	Publisher               string
	Binding                 string
	NumberOfPages           string
	YearPublished           string
	OriginalPublicationYear string
	DateRead                string
	DateAdded               string
	ExclusiveShelf          string
	MyReview                string
}

var (
	brTagExpr            = regexp.MustCompile(`(?i)<br\s*/?>`)
	htmlTagExpr          = regexp.MustCompile(`(?s)<[^>]+>`)
	parentheticalExpr    = regexp.MustCompile(`\s*\([^)]*\)`)
	multiNewlineExpr     = regexp.MustCompile(`\n{3,}`)
	spaceBeforePunctExpr = regexp.MustCompile(`\s+([,.;:!?])`)
)

func ImportGoodreads(root, csvPath string) error {
	if csvPath == "" {
		csvPath = filepath.Join(root, "goodreads_export.csv")
	}

	rows, err := readGoodreadsRows(csvPath)
	if err != nil {
		return err
	}

	articlesDir := filepath.Join(root, "articles")
	if err := os.MkdirAll(articlesDir, 0o755); err != nil {
		return err
	}
	booksDir := filepath.Join(root, "books")
	if err := os.MkdirAll(booksDir, 0o755); err != nil {
		return err
	}

	existingArticles := map[string]bool{}
	existingEntries, err := os.ReadDir(articlesDir)
	if err != nil {
		return err
	}
	for _, entry := range existingEntries {
		if entry.IsDir() {
			existingArticles[entry.Name()] = true
		}
	}
	existingBooks := map[string]bool{}
	bookEntries, err := os.ReadDir(booksDir)
	if err != nil {
		return err
	}
	for _, entry := range bookEntries {
		if entry.IsDir() {
			existingBooks[entry.Name()] = true
		}
	}

	importArticles := make([]Article, 0, len(rows))
	importBooks := make([]Article, 0, len(rows))
	reservedSlugs := map[string]string{}
	for _, row := range rows {
		if strings.TrimSpace(row.MyReview) != "" {
			article, err := goodreadsRowToArticle(row, reservedSlugs)
			if err != nil {
				return fmt.Errorf("convert Goodreads review %q: %w", row.Title, err)
			}
			if existingArticles[article.Meta.ID] {
				continue
			}
			importArticles = append(importArticles, article)
			continue
		}

		book, err := goodreadsRowToBook(row, reservedSlugs)
		if err != nil {
			return fmt.Errorf("convert Goodreads rating %q: %w", row.Title, err)
		}
		if existingBooks[book.Meta.ID] {
			continue
		}
		importBooks = append(importBooks, book)
	}

	for _, article := range importArticles {
		targetDir := filepath.Join(articlesDir, article.Meta.ID)
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return err
		}
		targetFile := filepath.Join(targetDir, "index.md")
		if err := os.WriteFile(targetFile, []byte(renderSourceArticle(article)), 0o644); err != nil {
			return err
		}
	}
	for _, book := range importBooks {
		targetDir := filepath.Join(booksDir, book.Meta.ID)
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return err
		}
		targetFile := filepath.Join(targetDir, "index.md")
		if err := os.WriteFile(targetFile, []byte(renderSourceBook(book)), 0o644); err != nil {
			return err
		}
	}

	return nil
}

func readGoodreadsRows(path string) ([]GoodreadsRow, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}

	header := records[0]
	indexes := map[string]int{}
	for i, name := range header {
		indexes[name] = i
	}

	rows := []GoodreadsRow{}
	for _, record := range records[1:] {
		row := GoodreadsRow{
			BookID:                  csvField(record, indexes, "Book Id"),
			Title:                   csvField(record, indexes, "Title"),
			Author:                  csvField(record, indexes, "Author"),
			ISBN:                    normalizeISBN(csvField(record, indexes, "ISBN")),
			ISBN13:                  normalizeISBN(csvField(record, indexes, "ISBN13")),
			MyRating:                csvField(record, indexes, "My Rating"),
			Publisher:               csvField(record, indexes, "Publisher"),
			Binding:                 csvField(record, indexes, "Binding"),
			NumberOfPages:           csvField(record, indexes, "Number of Pages"),
			YearPublished:           csvField(record, indexes, "Year Published"),
			OriginalPublicationYear: csvField(record, indexes, "Original Publication Year"),
			DateRead:                csvField(record, indexes, "Date Read"),
			DateAdded:               csvField(record, indexes, "Date Added"),
			ExclusiveShelf:          csvField(record, indexes, "Exclusive Shelf"),
			MyReview:                csvField(record, indexes, "My Review"),
		}
		hasReview := strings.TrimSpace(row.MyReview) != ""
		hasRating := strings.TrimSpace(row.MyRating) != "" && strings.TrimSpace(row.MyRating) != "0"
		if !hasReview && !hasRating {
			continue
		}
		rows = append(rows, row)
	}

	return rows, nil
}

func goodreadsRowToArticle(row GoodreadsRow, reservedSlugs map[string]string) (Article, error) {
	date := normalizeDate(row.DateRead)
	if date == "" {
		date = normalizeDate(row.DateAdded)
	}
	if date == "" {
		return Article{}, fmt.Errorf("missing date for review %q", row.Title)
	}

	baseSlug := "book-review-" + slugifyBookTitle(row.Title)
	if baseSlug == "book-review" {
		return Article{}, fmt.Errorf("unable to derive slug for review %q", row.Title)
	}
	slug := baseSlug
	if previousBookID, ok := reservedSlugs[slug]; ok && previousBookID != row.BookID {
		slug = fmt.Sprintf("%s-%s", baseSlug, slugifyBookTitle(row.BookID))
	}
	reservedSlugs[slug] = row.BookID

	title := "Book Review: " + strings.TrimSpace(html.UnescapeString(row.Title))
	body := convertGoodreadsReview(row.MyReview)

	return Article{
		Meta: ArticleMeta{
			ID:                               slug,
			Title:                            title,
			Author:                           []string{"Athul Suresh"},
			Date:                             date,
			Slug:                             slug,
			Kind:                             "review",
			Draft:                            false,
			OriginalSource:                   "Goodreads",
			BookAuthor:                       strings.TrimSpace(html.UnescapeString(row.Author)),
			GoodreadsBookID:                  strings.TrimSpace(row.BookID),
			GoodreadsRating:                  strings.TrimSpace(row.MyRating),
			GoodreadsExclusiveShelf:          strings.TrimSpace(row.ExclusiveShelf),
			GoodreadsDateRead:                normalizeDate(row.DateRead),
			GoodreadsDateAdded:               normalizeDate(row.DateAdded),
			GoodreadsISBN:                    strings.TrimSpace(row.ISBN),
			GoodreadsISBN13:                  strings.TrimSpace(row.ISBN13),
			GoodreadsPublisher:               strings.TrimSpace(html.UnescapeString(row.Publisher)),
			GoodreadsBinding:                 strings.TrimSpace(html.UnescapeString(row.Binding)),
			GoodreadsPages:                   strings.TrimSpace(row.NumberOfPages),
			GoodreadsPublicationYear:         strings.TrimSpace(row.YearPublished),
			GoodreadsOriginalPublicationYear: strings.TrimSpace(row.OriginalPublicationYear),
		},
		Body: body,
	}, nil
}

func goodreadsRowToBook(row GoodreadsRow, reservedSlugs map[string]string) (Article, error) {
	date := normalizeDate(row.DateRead)
	if date == "" {
		date = normalizeDate(row.DateAdded)
	}
	if date == "" {
		return Article{}, fmt.Errorf("missing date for book %q", row.Title)
	}

	baseSlug := "book-" + slugifyBookTitle(row.Title)
	if baseSlug == "book" {
		return Article{}, fmt.Errorf("unable to derive slug for book %q", row.Title)
	}
	slug := baseSlug
	if previousBookID, ok := reservedSlugs[slug]; ok && previousBookID != row.BookID {
		slug = fmt.Sprintf("%s-%s", baseSlug, slugifyBookTitle(row.BookID))
	}
	reservedSlugs[slug] = row.BookID

	return Article{
		Meta: ArticleMeta{
			ID:                               slug,
			Title:                            strings.TrimSpace(html.UnescapeString(row.Title)),
			Date:                             date,
			Slug:                             slug,
			BookStatus:                       "rated",
			OriginalSource:                   "Goodreads",
			BookAuthor:                       strings.TrimSpace(html.UnescapeString(row.Author)),
			GoodreadsBookID:                  strings.TrimSpace(row.BookID),
			GoodreadsRating:                  strings.TrimSpace(row.MyRating),
			GoodreadsExclusiveShelf:          strings.TrimSpace(row.ExclusiveShelf),
			GoodreadsDateRead:                normalizeDate(row.DateRead),
			GoodreadsDateAdded:               normalizeDate(row.DateAdded),
			GoodreadsISBN:                    strings.TrimSpace(row.ISBN),
			GoodreadsISBN13:                  strings.TrimSpace(row.ISBN13),
			GoodreadsPublisher:               strings.TrimSpace(html.UnescapeString(row.Publisher)),
			GoodreadsBinding:                 strings.TrimSpace(html.UnescapeString(row.Binding)),
			GoodreadsPages:                   strings.TrimSpace(row.NumberOfPages),
			GoodreadsPublicationYear:         strings.TrimSpace(row.YearPublished),
			GoodreadsOriginalPublicationYear: strings.TrimSpace(row.OriginalPublicationYear),
		},
	}, nil
}

func csvField(record []string, indexes map[string]int, name string) string {
	index, ok := indexes[name]
	if !ok || index >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[index])
}

func normalizeISBN(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "=")
	value = trimQuotes(value)
	return value
}

func normalizeDate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "/", "-")
	return value
}

func slugifyBookTitle(title string) string {
	title = strings.TrimSpace(html.UnescapeString(title))
	title = parentheticalExpr.ReplaceAllString(title, "")
	title = transliterateToASCII(title)
	title = strings.ToLower(title)
	title = punctExpr.ReplaceAllString(title, "-")
	title = strings.Trim(title, "-")
	title = strings.ReplaceAll(title, "--", "-")
	for strings.Contains(title, "--") {
		title = strings.ReplaceAll(title, "--", "-")
	}
	return title
}

func transliterateToASCII(value string) string {
	replacer := strings.NewReplacer(
		"à", "a", "á", "a", "â", "a", "ã", "a", "ä", "a", "å", "a",
		"æ", "ae", "ç", "c", "è", "e", "é", "e", "ê", "e", "ë", "e",
		"ì", "i", "í", "i", "î", "i", "ï", "i", "ñ", "n", "ò", "o",
		"ó", "o", "ô", "o", "õ", "o", "ö", "o", "ø", "o", "œ", "oe",
		"ù", "u", "ú", "u", "û", "u", "ü", "u", "ý", "y", "ÿ", "y",
		"ā", "a", "ē", "e", "ī", "i", "ō", "o", "ū", "u", "ł", "l",
		"À", "A", "Á", "A", "Â", "A", "Ã", "A", "Ä", "A", "Å", "A",
		"Æ", "AE", "Ç", "C", "È", "E", "É", "E", "Ê", "E", "Ë", "E",
		"Ì", "I", "Í", "I", "Î", "I", "Ï", "I", "Ñ", "N", "Ò", "O",
		"Ó", "O", "Ô", "O", "Õ", "O", "Ö", "O", "Ø", "O", "Œ", "OE",
		"Ù", "U", "Ú", "U", "Û", "U", "Ü", "U", "Ý", "Y", "Ā", "A",
		"Ē", "E", "Ī", "I", "Ō", "O", "Ū", "U", "Ł", "L",
	)
	return replacer.Replace(value)
}

func convertGoodreadsReview(review string) string {
	text := strings.ReplaceAll(review, "\r\n", "\n")
	text = brTagExpr.ReplaceAllString(text, "\n\n")
	text = htmlTagExpr.ReplaceAllString(text, "")
	text = html.UnescapeString(text)
	text = strings.ReplaceAll(text, "\u00a0", " ")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		line = spaceBeforePunctExpr.ReplaceAllString(line, "$1")
		lines[i] = line
	}
	text = strings.Join(lines, "\n")
	text = multiNewlineExpr.ReplaceAllString(text, "\n\n")
	text = strings.TrimSpace(text)
	return text + "\n"
}
