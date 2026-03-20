# Blogging Workflow

`articles/` is the source of truth for published writing. Each article lives in its own directory:

- `articles/<id>/index.md`
- `articles/<id>/assets/...` for article-local files if needed later

Article files use TOML front matter with a small portable schema:

```toml
+++
id = "bug-ttl"
title = "Bug Story: It's not you, it's the environment"
author = ["Athul Suresh"]
date = "2020-05-03"
slug = "bug-ttl"
article_kind = "essay"
tags = ["life"]
draft = false
+++
```

Required fields:

- `id`
- `title`
- `date`
- `slug`
- `article_kind`

Notes:

- `id` is the stable internal identifier.
- `slug` controls the public URL and should not change after publishing.
- `article_kind` is `essay` or `review`.
- Technical posts and book reviews are both articles. Hugo publishes them all under `/posts/<slug>/`.

## Commands

Common workflows through `make`:

```sh
make help
make new-post TITLE="My New Post"
make new-review TITLE="Example Title"
make import-goodreads
make build
make serve
make publish
```

`make publish` runs tests and rebuilds the site.
`make new-review` prefixes the title as `Book Review: ...` automatically.

Generate Hugo content from `articles/`:

```sh
env GOCACHE=/tmp/blog-gocache go run ./cmd/blogsync sync
```

Verify that the legacy Org source matches the Markdown articles:

```sh
env GOCACHE=/tmp/blog-gocache go run ./cmd/blogsync verify-org
```

Re-import the old Hugo-authored Markdown into `articles/`:

```sh
env GOCACHE=/tmp/blog-gocache go run ./cmd/blogsync import-legacy
```

Import Goodreads reviews with non-empty `My Review` values into `articles/`:

```sh
env GOCACHE=/tmp/blog-gocache go run ./cmd/blogsync import-goodreads
```

The Goodreads importer:

- reads `goodreads_export.csv`
- creates review articles with titles prefixed as `Book Review: ...`
- uses `Date Read` as the post date, falling back to `Date Added`
- stores Goodreads metadata in front matter
- refuses to overwrite existing article directories

## Generated Content

`content/posts/` is generated. Do not edit files there manually.

The Hugo-only pages remain hand-authored:

- `content/archive.md`
- `content/search.md`
