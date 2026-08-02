---
name: jdeen-cli
description: Operate the JDeen JSON:API with the `jdeen` command-line client. Use when an agent needs to authenticate with JDeen, configure API profiles, read or manage posts, categories, uploads, comments, and users, traverse documented relationships, paginate collections, or obtain machine-readable JSON from JDeen environments.
---

# JDeen CLI

Use the installed `jdeen` binary instead of constructing HTTP requests directly.

## Work safely

- Run `jdeen --help` and `jdeen <resource> --help` when command details are unclear.
- Confirm the active target with `jdeen profiles list` before accessing sensitive or production data.
- Never print, persist, or request access or refresh tokens. Use `jdeen auth login`; credentials are stored in the operating-system credential service.
- Treat `--insecure-skip-verify` as local-development-only. Prefer a configured `--ca-cert`.
- Start with human-readable output. Add `--json` when structured output is needed.
- Get confirmation before creates, updates, deletes, session revocation, logout, profile changes, or authentication changes unless the user explicitly requested the operation.
- Treat deletes as permanent. Do not add `--yes` until the exact resource and identifier have been verified.

## Configure and authenticate

```sh
jdeen profiles list
jdeen profiles set dev --api-url https://api.jdeen.test:4021/v1 --ca-cert /path/to/jdeen.test.pem
jdeen profiles use dev
jdeen auth login --email admin@example.com --device-name "Agent CLI"
jdeen auth status
```

Production is the default profile and uses `https://api.jdeen.com/v1`. Override one invocation with `--profile <name>` or `--api-url <url>` when appropriate.

Use `jdeen auth sessions list` to inspect sessions and `jdeen auth sessions revoke <session-id>` only after verifying the session. Use `jdeen auth logout` to revoke the current server session and clear local credentials. Reserve `--local-only` for an unreachable server.

## Read content

Resources support `list`, `show <id>`, and `related <id> <relationship>` operations:

```sh
jdeen posts list --filter state=published --include author,categories --sort=-created_at
jdeen posts show <post-id> --include comments.user
jdeen posts related <post-id> categories
jdeen categories list --filter slug=engineering
jdeen uploads list --filter checksum_sha256=<sha256>
jdeen comments list --filter post=<post-id>
jdeen users list --sort name --json
```

Available resources are `posts`, `categories`, `uploads`, `comments`, and `users`. Allowed relationships differ by resource; consult command help before using `related` or `--include`.

Collections accept repeatable JSON:API `--filter key=value`, `--include`, `--fields type=field1,field2`, `--sort`, and cursor pagination flags. Follow the exact opaque URL returned in `links.next` or `links.prev` with:

```sh
jdeen posts list --page-url '<links.next URL>'
```

With `--json`, expect the full JSON:API document, including `data`, `included`, `links`, and `meta`; do not assume a bare array or object.

## Manage content

Inspect the target before updating or deleting it. Use resource-specific help to confirm flags.

```sh
jdeen posts create --title "Title" --content-file ./post.md --author <user-id> --category <category-id> --state draft --format markdown
jdeen posts update <post-id> --state published
jdeen categories create --name Engineering --description "Engineering notes"
jdeen uploads create --file ./banner.png --description "Banner" --alt-text "Accessible description"
jdeen comments create --post <post-id> --user <user-id> --body "A thoughtful response."
```

Use `--content-file -` or `--body-file -` to read from stdin. Clear nullable values and relationships only with explicit flags such as `--clear-summary`, `--clear-categories`, `--clear-banner-upload`, `--clear-description`, or `--clear-alt-text`.

Delete with `jdeen <resource> delete <id> --yes` only after the user has authorized deletion and the identifier has been checked. When creating a set of related temporary resources, clean them up in dependency order: comments, posts, uploads, then categories.
