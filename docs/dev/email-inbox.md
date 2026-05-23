# Filesystem Email Inbox

The server can write outgoing email to disk instead of shipping it
through Brevo. This is a **local dev / staging affordance** for
end-to-end test harnesses and demos. It is **not** allowed in
production — the config loader hard-rejects it.

Default behavior is unchanged: with no env override, every mode uses
SMTP.

## Enabling

Set these two keys in `.env.local` (or the process env) before
starting the server:

```bash
LIVEABOARD_EMAIL_TRANSPORT=filesystem
LIVEABOARD_EMAIL_FILESYSTEM_DIR=/tmp/inbox   # default; override if you want
```

Then `make dev` (or run `bin/liveaboard --mode dev`) as normal. The
server logs:

```
WARN email transport: filesystem (no SMTP delivery) inbox_dir=/tmp/inbox
INFO dev inbox viewer mounted path=/dev/inbox inbox_dir=/tmp/inbox
```

SMTP credentials (`LIVEABOARD_SMTP_USERNAME`, etc.) are not required
in filesystem mode. The `From:` header defaults to
`Liveaboard <noreply@filesystem.local>` if `LIVEABOARD_SMTP_FROM` is
unset.

## On-disk layout

Each rendered message produces two files under the recipient's
directory:

```
/tmp/inbox/
  e2e+signup@example.invalid/
    20260520T142231.123456Z-7c2a1d.eml
    20260520T142231.123456Z-7c2a1d.json
    latest.eml      (regular file, rewritten atomically per send)
    latest.json     (regular file, rewritten atomically per send)
```

- **Recipient directory** is the lowercased address with `@ . + _ -`
  preserved; any other byte becomes `_`. Path-traversal characters
  cannot survive this transform.
- **`.eml`** is the exact MIME payload SMTP would have transmitted
  (multipart/alternative, quoted-printable). Open it in any MUA, or
  `cat` it for a terminal-friendly view.
- **`.json`** is the machine-readable contract for E2E automation:

  ```json
  {
    "id": "7c2a1db1-...",
    "from": "Liveaboard <noreply@x.test>",
    "to": "e2e+signup@example.invalid",
    "subject": "Confirm your email",
    "sent_at": "2026-05-20T14:22:31.123456Z",
    "eml_path": "/tmp/inbox/e2e+signup@example.invalid/20260520T142231.123456Z-7c2a1d.eml",
    "links": [
      "http://localhost:5173/verify-email?token=..."
    ]
  }
  ```

  `links` is produced by the same `email.ExtractLinks` helper that
  powers `MockSender.LinkFor` in unit tests, so unit tests and E2E
  tests recover identical URLs from the same rendered message.
- **`latest.*`** are regular files (not symlinks) atomically updated
  on every send. Smoke scripts can rely on
  `<dir>/<recipient>/latest.json` always reflecting the newest
  message.

## Inspecting the inbox

### CLI

```bash
make inbox                       # list recipients + latest subject
LIVEABOARD_EMAIL_FILESYSTEM_DIR=/tmp/inbox make inbox

scripts/inbox.sh owner@x.test    # pretty-print latest.json (uses jq if available)

make inbox-clear                 # rm -rf the inbox; FORCE=1 skips the prompt
```

### Dev HTTP viewer

In **dev mode with filesystem transport**, the server mounts three
read-only GET endpoints:

| URL | Returns |
|---|---|
| `/dev/inbox` | HTML index of recipients, newest first. |
| `/dev/inbox/{recipient}` | HTML list of messages for one recipient. |
| `/dev/inbox/{recipient}/{file}` | Raw `.eml` (text/plain) or `.json`. |

The viewer is conditionally mounted at router construction time; it
is **never** mounted in test mode, production mode, or with SMTP
transport. Path parameters are validated against the writer's slug
rules, and the message handler refuses paths that resolve outside
`LIVEABOARD_EMAIL_FILESYSTEM_DIR`.

There is no authentication on the viewer because it only exists in
dev mode on the developer's own machine.

## Programmatic access from tests

End-to-end tests should use the helpers in `internal/email`:

```go
link, err := email.InboxLinkFor(inboxDir, "owner@x.test", "verify-email")
meta, err := email.ReadLatestInboxMessage(inboxDir, "owner@x.test")
```

`internal/httpapi/email_filesystem_e2e_test.go` is the canonical
example: it boots the HTTP server with a `FilesystemSender` into
`t.TempDir()`, posts a signup, recovers the verification URL from the
on-disk inbox, follows it, and asserts the user is now verified.

Unit tests should continue to use `email.MockSender`; the filesystem
sender does I/O and is reserved for end-to-end-style flows.

## Security caveats

- The `.eml` and `.json` artifacts both contain token-bearing URLs.
  Anyone with shell access to the host can complete any in-flight
  verification, invitation, or password reset.
- Don't point `LIVEABOARD_EMAIL_FILESYSTEM_DIR` at a directory served
  by any HTTP handler other than the dev-mode `/dev/inbox` viewer.
  The default `/tmp/inbox` is outside the static SPA roots and the
  guest documents directory.
- Production mode rejects `LIVEABOARD_EMAIL_TRANSPORT=filesystem` at
  config-load time. There is no opt-in flag.
