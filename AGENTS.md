# Repository Guidelines

Use `mise` for project tools and run project commands through `mise exec --` so
the Go version in `mise.toml` is used consistently.

Before finishing changes, run:

```sh
mise exec -- gofmt -w .
mise run check
mise run test
mise run test-race
mise run build
```

