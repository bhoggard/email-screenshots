# Gmail Migration Design

**Date:** 2026-05-13
**Branch:** gmail-switch

## Overview

Replace the Fastmail JMAP email client with a Gmail API client using a Google Workspace service account. All other functionality (screenshot generation, CLI flags, folder-to-label mapping) remains unchanged.

## File Changes

| File | Change |
|------|--------|
| `jmap.go` | Delete |
| `types.go` | New — shared types moved from `jmap.go`: `Email`, `Mailbox`, `EmailAddress`, `HTMLBodyPart`, `BodyValue` |
| `gmail.go` | New — `GmailClient` implementing `EmailClient` |
| `interfaces.go` | No change |
| `screenshot.go` | No change |
| `main.go` | Replace `NewJMAPClient` + `FASTMAIL_AAR_KEY` with `NewGmailClient` + new env vars |
| `go.mod` | Add `google.golang.org/api` and `golang.org/x/oauth2` |
| `README.md` | Update for using gmail configuration instead of fastmail |

## Authentication

- Auth method: Google Workspace service account with domain-wide delegation
- Service account JSON key file path provided via `GOOGLE_SERVICE_ACCOUNT_KEY` env var
- Impersonated user email provided via `GMAIL_USER_EMAIL` env var
- Scopes required: `https://www.googleapis.com/auth/gmail.modify`

## GmailClient Implementation

### `FindMailboxByName(name string) (*Mailbox, error)`
- Calls `users.labels.list` for the impersonated user
- Finds the label matching `name` (exact match)
- Returns a `Mailbox` with the label's ID and name

### `GetEmailsInMailbox(labelID string, limit int) ([]string, error)`
- Calls `users.messages.list` with `labelIds=[labelID]`
- Applies `limit` if > 0
- Returns message IDs

### `GetEmails(ids []string) ([]Email, error)`
- Calls `users.messages.get` for each ID with `format=full`
- Extracts: subject (from headers), `internalDate` (converted from ms epoch to RFC3339), HTML body part, label IDs (as `mailboxIds` map)
- Returns `[]Email` matching the existing struct

### `MoveEmail(emailID, sourceLabelID, targetLabelID string) error`
- Calls `users.messages.modify` with `addLabelIds=[targetLabelID]` and `removeLabelIds=[sourceLabelID]`

## Configuration

### Removed
```
FASTMAIL_AAR_KEY
```

### Added
```
GOOGLE_SERVICE_ACCOUNT_KEY=/path/to/service-account.json
GMAIL_USER_EMAIL=user@yourdomain.com
```

### Unchanged
```
SCREENSHOT_DIR=./screenshots
SCREENSHOT_WIDTH=1280
SCREENSHOT_HEIGHT=800
```

### CLI Flags (unchanged)
```
-limit int      Maximum emails to process (default: 0 = all)
-dry-run bool   Preview operations without making changes
```

## Label Names

Source and archive label names are unchanged: `_aar` and `_aar_processed`. These must exist as Gmail labels in the target account.

## Data Flow

1. Load service account key + impersonate `GMAIL_USER_EMAIL`
2. Find `_aar` label ID via `labels.list`
3. Find `_aar_processed` label ID via `labels.list`
4. List messages with `_aar` label
5. For each message:
   - Fetch full message, extract subject + HTML body + `internalDate`
   - Convert `internalDate` (ms epoch) to RFC3339 for screenshot filename
   - Generate screenshot
   - Move message: remove `_aar`, add `_aar_processed`

## Error Handling

- Auth failure: exit with clear message
- Label not found: exit with clear message (same as current mailbox-not-found behavior)
- Per-email failures: log and continue to next email
