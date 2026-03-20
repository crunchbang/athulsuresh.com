package blogsync

import (
	"errors"
	"os"
	"path/filepath"
)

func Sync(root string) error {
	if _, err := os.Stat(filepath.Join(root, "articles")); errors.Is(err, os.ErrNotExist) {
		if err := ImportLegacy(root); err != nil {
			return err
		}
	}

	if _, err := os.Stat(filepath.Join(root, "content-org", "all-posts.org")); err == nil {
		if err := VerifyOrgParity(root); err != nil {
			return err
		}
	}

	return GenerateHugoContent(root)
}
