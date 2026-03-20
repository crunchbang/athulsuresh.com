SHELL := /bin/zsh

GO ?= go
HUGO ?= hugo
PYTHON ?= python3
GOCACHE ?= /tmp/blog-gocache
BLOGSYNC := env GOCACHE=$(GOCACHE) $(GO) run ./cmd/blogsync
TODAY := $(shell date +%F)

.PHONY: help test sync build serve publish import-goodreads import-legacy verify-org new-post new-review

help:
	@printf "Common workflows:\n"
	@printf "  make new-post TITLE='Title'\n"
	@printf "  make new-review TITLE='Book Title'\n"
	@printf "  make import-goodreads\n"
	@printf "  make sync\n"
	@printf "  make build\n"
	@printf "  make serve\n"
	@printf "  make publish\n"
	@printf "  make test\n"

test:
	@env GOCACHE=$(GOCACHE) $(GO) test ./...

sync:
	@$(BLOGSYNC) sync

build: sync
	@$(HUGO) --cleanDestinationDir

serve: sync
	@$(HUGO) server -D --disableFastRender

publish: test build

import-goodreads:
	@$(BLOGSYNC) import-goodreads

import-legacy:
	@$(BLOGSYNC) import-legacy

verify-org:
	@$(BLOGSYNC) verify-org

new-post:
	@$(MAKE) _new-article ARTICLE_KIND=essay TITLE="$(TITLE)"

new-review:
	@if [[ -z "$(TITLE)" ]]; then \
		echo "TITLE is required. Example: make new-review TITLE='The Left Hand of Darkness'"; \
		exit 1; \
	fi
	@slug="book-review-$$($(PYTHON) -c 'import re, sys; title=sys.argv[1].strip().lower(); title=re.sub(r"[^a-z0-9]+","-", title); title=re.sub(r"-+","-", title).strip("-"); print(title)' "$(TITLE)")" ; \
	if [[ "$$slug" == "book-review-" ]]; then \
		echo "Could not derive slug from TITLE"; \
		exit 1; \
	fi; \
	$(MAKE) _new-article ARTICLE_KIND=review TITLE="Book Review: $(TITLE)" SLUG="$$slug"

.PHONY: _new-article
_new-article:
	@if [[ -z "$(TITLE)" ]]; then \
		echo "TITLE is required. Example: make new-post TITLE='My New Post'"; \
		exit 1; \
	fi
	@slug="$${SLUG:-$$($(PYTHON) -c 'import re, sys; title=sys.argv[1].strip().lower(); title=re.sub(r"[^a-z0-9]+","-", title); title=re.sub(r"-+","-", title).strip("-"); print(title)' "$(TITLE)")}" ; \
	if [[ -z "$$slug" ]]; then \
		echo "Could not derive slug from TITLE"; \
		exit 1; \
	fi; \
	dir="articles/$$slug"; \
	file="$$dir/index.md"; \
	if [[ -e "$$dir" ]]; then \
		echo "$$dir already exists"; \
		exit 1; \
	fi; \
	mkdir -p "$$dir"; \
	printf '+++\n' > "$$file"; \
	printf 'id = "%s"\n' "$$slug" >> "$$file"; \
	printf 'title = "%s"\n' "$(TITLE)" >> "$$file"; \
	printf 'author = ["Athul Suresh"]\n' >> "$$file"; \
	printf 'date = "%s"\n' "$(TODAY)" >> "$$file"; \
	printf 'slug = "%s"\n' "$$slug" >> "$$file"; \
	printf 'article_kind = "%s"\n' "$(ARTICLE_KIND)" >> "$$file"; \
	printf 'draft = true\n' >> "$$file"; \
	printf '+++\n\n' >> "$$file"; \
	printf '%s\n' 'Start writing here.' >> "$$file"; \
	echo "Created $$file"; \
	echo "Next steps: edit $$file && make build"
