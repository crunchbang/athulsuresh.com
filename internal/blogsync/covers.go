package blogsync

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type openLibrarySearchResponse struct {
	Docs []struct {
		CoverID int `json:"cover_i"`
	} `json:"docs"`
}

type googleBooksResponse struct {
	Items []struct {
		VolumeInfo struct {
			ImageLinks struct {
				Thumbnail      string `json:"thumbnail"`
				SmallThumbnail string `json:"smallThumbnail"`
			} `json:"imageLinks"`
		} `json:"volumeInfo"`
	} `json:"items"`
}

func FetchBookCovers(root string) error {
	articles, err := loadArticles(root)
	if err != nil {
		return err
	}
	bookRecords, err := loadBookRecords(root)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 20 * time.Second}
	outputDir := filepath.Join(root, "static", "book-covers")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}

	for _, article := range articles {
		if article.Meta.Kind != "review" || !strings.HasPrefix(article.Meta.Title, "Book Review:") {
			continue
		}
		targetPath := filepath.Join(outputDir, article.Meta.Slug+".jpg")
		if _, err := os.Stat(targetPath); err == nil {
			continue
		}

		coverURL, err := resolveCoverURL(client, article)
		if err != nil || coverURL == "" {
			continue
		}
		if err := downloadCover(client, coverURL, targetPath); err != nil {
			return fmt.Errorf("download cover for %s: %w", article.Meta.Slug, err)
		}
	}
	for _, book := range bookRecords {
		targetPath := filepath.Join(outputDir, book.Meta.Slug+".jpg")
		if _, err := os.Stat(targetPath); err == nil {
			continue
		}

		coverURL, err := resolveCoverURL(client, book)
		if err != nil || coverURL == "" {
			continue
		}
		if err := downloadCover(client, coverURL, targetPath); err != nil {
			return fmt.Errorf("download cover for %s: %w", book.Meta.Slug, err)
		}
	}

	return nil
}

func resolveCoverURL(client *http.Client, article Article) (string, error) {
	for _, isbn := range []string{article.Meta.GoodreadsISBN13, article.Meta.GoodreadsISBN} {
		isbn = strings.TrimSpace(isbn)
		if isbn == "" {
			continue
		}
		url := fmt.Sprintf("https://covers.openlibrary.org/b/isbn/%s-L.jpg?default=false", isbn)
		ok, err := remoteFileExists(client, url)
		if err != nil {
			return "", err
		}
		if ok {
			return url, nil
		}
	}

	query := url.Values{}
	query.Set("title", bookDisplayTitle(article.Meta.Title))
	query.Set("author", article.Meta.BookAuthor)
	query.Set("limit", "1")
	searchURL := "https://openlibrary.org/search.json?" + query.Encode()

	req, err := http.NewRequest(http.MethodGet, searchURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "athulsuresh-blog/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("search request returned %s", resp.Status)
	}

	var payload openLibrarySearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	for _, doc := range payload.Docs {
		if doc.CoverID == 0 {
			continue
		}
		url := fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-L.jpg?default=false", doc.CoverID)
		ok, err := remoteFileExists(client, url)
		if err != nil {
			return "", err
		}
		if ok {
			return url, nil
		}
	}

	googleURL, err := resolveGoogleBooksCoverURL(client, article)
	if err != nil {
		return "", err
	}
	if googleURL != "" {
		return googleURL, nil
	}

	return "", nil
}

func resolveGoogleBooksCoverURL(client *http.Client, article Article) (string, error) {
	title := bookDisplayTitle(article.Meta.Title)
	queries := []string{}
	for _, isbn := range []string{article.Meta.GoodreadsISBN13, article.Meta.GoodreadsISBN} {
		isbn = strings.TrimSpace(isbn)
		if isbn != "" {
			queries = append(queries, "isbn:"+isbn)
		}
	}
	if title != "" || article.Meta.BookAuthor != "" {
		queries = append(queries, fmt.Sprintf("intitle:%s+inauthor:%s", title, article.Meta.BookAuthor))
	}

	for _, rawQuery := range queries {
		query := url.Values{}
		query.Set("q", rawQuery)
		query.Set("maxResults", "1")
		searchURL := "https://www.googleapis.com/books/v1/volumes?" + query.Encode()

		req, err := http.NewRequest(http.MethodGet, searchURL, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", "athulsuresh-blog/1.0")

		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return "", fmt.Errorf("google books request returned %s", resp.Status)
		}

		var payload googleBooksResponse
		err = json.NewDecoder(resp.Body).Decode(&payload)
		resp.Body.Close()
		if err != nil {
			return "", err
		}

		for _, item := range payload.Items {
			for _, imageURL := range []string{
				item.VolumeInfo.ImageLinks.Thumbnail,
				item.VolumeInfo.ImageLinks.SmallThumbnail,
			} {
				imageURL = strings.TrimSpace(imageURL)
				if imageURL == "" {
					continue
				}
				imageURL = strings.Replace(imageURL, "http://", "https://", 1)
				ok, err := remoteFileExists(client, imageURL)
				if err != nil {
					return "", err
				}
				if ok {
					return imageURL, nil
				}
			}
		}
	}

	return "", nil
}

func bookDisplayTitle(title string) string {
	return strings.TrimSpace(strings.TrimPrefix(title, "Book Review: "))
}

func remoteFileExists(client *http.Client, rawURL string) (bool, error) {
	req, err := http.NewRequest(http.MethodHead, rawURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", "athulsuresh-blog/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK && strings.HasPrefix(resp.Header.Get("Content-Type"), "image/"), nil
}

func downloadCover(client *http.Client, rawURL, targetPath string) error {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "athulsuresh-blog/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %s", resp.Status)
	}
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "image/") {
		return fmt.Errorf("unexpected content type %q", resp.Header.Get("Content-Type"))
	}

	file, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}
