package blogsync

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func loadBookRecords(root string) ([]Article, error) {
	booksDir := filepath.Join(root, "books")
	entries, err := os.ReadDir(booksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Article{}, nil
		}
		return nil, err
	}

	books := make([]Article, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		indexPath := filepath.Join(booksDir, entry.Name(), "index.md")
		book, err := parseArticleFile(indexPath)
		if err != nil {
			return nil, err
		}
		books = append(books, book)
	}

	sort.Slice(books, func(i, j int) bool {
		if books[i].Meta.Date == books[j].Meta.Date {
			return books[i].Meta.Slug < books[j].Meta.Slug
		}
		return books[i].Meta.Date < books[j].Meta.Date
	})

	return books, nil
}

func validateBookRecords(books []Article) error {
	ids := map[string]string{}
	slugs := map[string]string{}
	for _, book := range books {
		if book.Meta.ID == "" || book.Meta.Title == "" || book.Meta.Date == "" || book.Meta.Slug == "" {
			return fmt.Errorf("book %q is missing required metadata", book.Meta.Title)
		}
		if book.Meta.BookStatus == "" {
			return fmt.Errorf("book %q is missing book_status", book.Meta.ID)
		}
		if book.Meta.BookStatus != "rated" {
			return fmt.Errorf("book %q has unsupported book_status %q", book.Meta.ID, book.Meta.BookStatus)
		}
		if previous, ok := ids[book.Meta.ID]; ok {
			return fmt.Errorf("duplicate book id %q in %s and %s", book.Meta.ID, previous, book.Meta.Title)
		}
		if previous, ok := slugs[book.Meta.Slug]; ok {
			return fmt.Errorf("duplicate book slug %q in %s and %s", book.Meta.Slug, previous, book.Meta.Title)
		}
		ids[book.Meta.ID] = book.Meta.Title
		slugs[book.Meta.Slug] = book.Meta.Title
	}
	return nil
}
