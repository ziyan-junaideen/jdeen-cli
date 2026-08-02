# JDeen CLI

Go command-line client for authenticating with and managing the JDeen JSON:API
v1.

## Prerequisite

Install [mise](https://mise.jdx.dev/) and trust this repository's tool config,
then install the pinned Go toolchain:

```sh
mise trust
mise install
```

## Development

```sh
mise run tidy
mise run test
mise run test-race
mise run check
mise run build
./bin/jdeen --help
```

For an ad-hoc run:

```sh
mise run run -- --help
```

## Profiles

Production is the active profile by default and targets
`https://api.jdeen.com/v1`. The built-in `dev` profile targets the local HTTPS
API at `https://api.jdeen.test:4021/v1`.

```sh
jdeen profiles list
jdeen profiles use dev
jdeen profiles set local-http --api-url http://api.jdeen.test:4020
jdeen profiles set dev --ca-cert /Volumes/Dev/JDeen/ex_jdeen/priv/cert/jdeen.test.pem
```

Configuration is stored in the platform XDG config directory. Command flags
override `JDEEN_*` environment variables, which override profile settings:

- `JDEEN_PROFILE`
- `JDEEN_API_URL`
- `JDEEN_CA_CERT`
- `JDEEN_INSECURE_SKIP_VERIFY`

`--insecure-skip-verify` is limited to non-production hosts and always prints a
warning.

## Authentication

Interactive login securely prompts for the password and, when enabled, the
six-digit TOTP code:

```sh
jdeen auth login --email admin@example.com --device-name "Ziyan's MacBook CLI"
jdeen auth status
jdeen auth sessions list
jdeen auth sessions revoke <session-id>
jdeen auth logout
```

The access token, refresh token, session ID, and expiry timestamps are stored as
one record in the operating-system credential service. Tokens are refreshed
automatically and never printed. `auth logout` revokes the current remote
session before clearing it locally; use `--local-only` only when the server
cannot be reached.

For non-interactive login, use `--non-interactive` with `JDEEN_EMAIL`,
`JDEEN_PASSWORD`, and, when needed, `JDEEN_TOTP_CODE`. The keyring library's
encrypted-file fallback uses `JDEEN_KEYRING_PASSWORD`. A short-lived
`JDEEN_ACCESS_TOKEN` can directly authenticate a CI invocation; direct tokens
cannot be refreshed by the CLI.

## Reading content

All collections accept JSON:API filters, includes, sparse fieldsets, sorting,
and cursor pagination:

```sh
jdeen posts list \
  --filter state=draft \
  --include author,categories,banner_upload \
  --fields posts=title,slug,state \
  --sort=-created_at \
  --page-size 25

jdeen posts show <post-id> --include comments.user
jdeen categories list --filter slug=engineering
jdeen uploads list --filter checksum_sha256=<sha256>
jdeen comments list --filter post=<post-id>
jdeen users list --sort name
```

Human-readable collections print the exact `Next` and `Previous` URLs returned
by the API. Follow one without inspecting its opaque cursor:

```sh
jdeen posts list --page-url '<links.next URL>'
```

Use `--json` to return the full JSON:API response document.

Fetch documented relationships with `related`:

```sh
jdeen posts related <post-id> author
jdeen posts related <post-id> categories
jdeen posts related <post-id> comments --include user
jdeen comments related <comment-id> replies
jdeen users related <user-id> profile_upload
```

## Managing content

### Posts

```sh
jdeen posts create \
  --title "Building dependable software" \
  --content-file ./post.md \
  --author <user-id> \
  --category <category-id> \
  --state draft \
  --format markdown

jdeen posts update <post-id> --state published --featured=true
jdeen posts update <post-id> --clear-categories --clear-banner-upload
jdeen posts delete <post-id> --yes
```

Use `--content-file -` to read post content from stdin.

### Categories

```sh
jdeen categories create --name Engineering --description "Engineering notes"
jdeen categories update <category-id> --name "Software Engineering"
jdeen categories delete <category-id> --yes
```

### Uploads

```sh
jdeen uploads create \
  --file ./banner.png \
  --description "Architecture diagram" \
  --alt-text "Services connected through an event bus"

jdeen uploads update <upload-id> --alt-text "Updated accessible description"
jdeen uploads delete <upload-id> --yes
```

The CLI computes and displays the upload's SHA-256 checksum. It never retries
an upload after an ambiguous connection failure; use the suggested checksum
filter to determine whether the server accepted it.

### Comments

```sh
jdeen comments create \
  --post <post-id> \
  --user <user-id> \
  --body "A thoughtful response."

jdeen comments create \
  --post <post-id> \
  --user <user-id> \
  --parent-comment <comment-id> \
  --body-file ./reply.txt

jdeen comments update <comment-id> --status quarantined
jdeen comments delete <comment-id> --yes
```

Deletes are permanent. Without `--yes`, the CLI requires an interactive
confirmation.

## Local smoke test

With `ex_jdeen` running and a confirmed administrator available:

1. Select `dev` and run `jdeen auth login`.
2. Create a temporary category and upload.
3. Create a draft post using those IDs, then fetch its relationships.
4. Create and update a comment, and exercise a filtered/paginated list.
5. Delete the comment, post, upload, and category in that order.
6. Run `jdeen auth logout` and confirm the session was revoked.

## Installation with Homebrew

The included formula is prepared for the `v0.1.0` release tag:

```sh
brew tap ziyan-junaideen/tap
brew install jdeen-cli
jdeen --version
```
