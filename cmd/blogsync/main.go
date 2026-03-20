package main

import (
	"fmt"
	"os"

	"github.com/crunchbang/athulsuresh.com/internal/blogsync"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "blogsync: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	command := "sync"
	if len(args) > 0 {
		command = args[0]
	}

	switch command {
	case "sync":
		return blogsync.Sync(".")
	case "fetch-book-covers":
		return blogsync.FetchBookCovers(".")
	case "import-goodreads":
		return blogsync.ImportGoodreads(".", "")
	case "import-legacy":
		return blogsync.ImportLegacy(".")
	case "generate":
		return blogsync.GenerateHugoContent(".")
	case "verify-org":
		return blogsync.VerifyOrgParity(".")
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}
