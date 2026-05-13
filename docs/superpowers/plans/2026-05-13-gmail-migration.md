# Gmail Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Fastmail JMAP email client with a Gmail API client backed by a Google Workspace service account, keeping all other functionality unchanged.

**Architecture:** Extract shared types from `jmap.go` into `types.go`, then replace `jmap.go` entirely with `gmail.go` implementing the same `EmailClient` interface. `main.go` swaps env var and constructor; everything else (screenshot, tests, CLI flags) is untouched.

**Tech Stack:** `google.golang.org/api/gmail/v1`, `golang.org/x/oauth2/google`, existing `chromedp`, Go standard library.

---

### Task 1: Add Gmail API dependencies

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Add dependencies**

```bash
go get google.golang.org/api/gmail/v1
go get golang.org/x/oauth2/google
```

Expected: go.mod now lists both packages; no errors.

- [ ] **Step 2: Verify existing tests still pass**

```bash
go test ./...
```

Expected: all tests PASS (no code has changed yet).

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "feat: add gmail api and oauth2 dependencies"
```

---

### Task 2: Create `types.go` — move shared types out of `jmap.go`

**Files:**
- Create: `types.go`
- Modify: `jmap.go` (remove the type definitions that move to types.go)

The types `Email`, `Mailbox`, `EmailAddress`, `HTMLBodyPart`, and `BodyValue` are currently defined in `jmap.go`. Moving them to `types.go` makes them available to `gmail.go` without any import changes (same package).

- [ ] **Step 1: Create `types.go`**

Create `/path/to/types.go` with this exact content:

```go
package main

// Mailbox represents a folder or label
type Mailbox struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role,omitempty"`
}

// Email represents an email message
type Email struct {
	ID         string               `json:"id"`
	Subject    string               `json:"subject"`
	ReceivedAt string               `json:"receivedAt"`
	From       []EmailAddress       `json:"from"`
	HTMLBody   []HTMLBodyPart       `json:"htmlBody"`
	BodyValues map[string]BodyValue `json:"bodyValues"`
	MailboxIds map[string]bool      `json:"mailboxIds"`
}

// EmailAddress represents an email address
type EmailAddress struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

// HTMLBodyPart represents an HTML body part
type HTMLBodyPart struct {
	PartID string `json:"partId"`
	Type   string `json:"type"`
}

// BodyValue represents the body content
type BodyValue struct {
	Value  string `json:"value"`
	IsHTML bool   `json:"isEncodingProblem"`
}
```

- [ ] **Step 2: Remove the same type definitions from `jmap.go`**

Delete these lines from `jmap.go` (lines 51–84, the five type definitions: `Mailbox`, `Email`, `EmailAddress`, `HTMLBodyPart`, `BodyValue`). Leave all other code in `jmap.go` intact.

- [ ] **Step 3: Verify the build compiles**

```bash
go build ./...
```

Expected: compiles without errors.

- [ ] **Step 4: Run tests**

```bash
go test ./...
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add types.go jmap.go
git commit -m "refactor: move shared types to types.go"
```

---

### Task 3: Create `gmail.go` — implement `EmailClient` using Gmail API

**Files:**
- Create: `gmail.go`

The `GmailClient` authenticates using a service account JSON key file (path in `GOOGLE_SERVICE_ACCOUNT_KEY`) and impersonates the user in `GMAIL_USER_EMAIL`. It implements the four methods of the `EmailClient` interface.

Gmail specifics:
- Labels are found via `users.labels.list` — match by exact name.
- Messages are listed via `users.messages.list` with `labelIds`.
- Message detail is fetched via `users.messages.get` with `format=FULL` — body parts are base64url-encoded, MIME type `text/html`.
- `internalDate` from Gmail is a string of milliseconds since Unix epoch; convert to RFC3339 for `ReceivedAt`.
- Move = `users.messages.modify` adding target label ID, removing source label ID.

- [ ] **Step 1: Create `gmail.go`**

```go
package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// GmailClient handles Gmail API interactions via a service account
type GmailClient struct {
	svc  *gmail.Service
	user string
}

// NewGmailClient creates a GmailClient using a service account key file.
// keyPath is the path to the service account JSON key file.
// userEmail is the Google Workspace user to impersonate.
func NewGmailClient(keyPath, userEmail string) (*GmailClient, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read service account key: %w", err)
	}

	config, err := google.JWTConfigFromJSON(keyData, gmail.GmailModifyScope)
	if err != nil {
		return nil, fmt.Errorf("failed to parse service account key: %w", err)
	}
	config.Subject = userEmail

	ctx := context.Background()
	svc, err := gmail.NewService(ctx, option.WithTokenSource(config.TokenSource(ctx)))
	if err != nil {
		return nil, fmt.Errorf("failed to create gmail service: %w", err)
	}

	return &GmailClient{svc: svc, user: userEmail}, nil
}

// FindMailboxByName finds a Gmail label by exact name.
func (c *GmailClient) FindMailboxByName(name string) (*Mailbox, error) {
	resp, err := c.svc.Users.Labels.List(c.user).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list labels: %w", err)
	}
	for _, label := range resp.Labels {
		if label.Name == name {
			return &Mailbox{ID: label.Id, Name: label.Name}, nil
		}
	}
	return nil, fmt.Errorf("label '%s' not found", name)
}

// GetEmailsInMailbox returns message IDs in a label, up to limit (0 = all).
func (c *GmailClient) GetEmailsInMailbox(labelID string, limit int) ([]string, error) {
	call := c.svc.Users.Messages.List(c.user).LabelIds(labelID)
	if limit > 0 {
		call = call.MaxResults(int64(limit))
	}
	resp, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list messages: %w", err)
	}
	ids := make([]string, 0, len(resp.Messages))
	for _, msg := range resp.Messages {
		ids = append(ids, msg.Id)
	}
	return ids, nil
}

// GetEmails fetches full email details for the given IDs.
func (c *GmailClient) GetEmails(emailIDs []string) ([]Email, error) {
	emails := make([]Email, 0, len(emailIDs))
	for _, id := range emailIDs {
		msg, err := c.svc.Users.Messages.Get(c.user, id).Format("full").Do()
		if err != nil {
			return nil, fmt.Errorf("failed to get message %s: %w", id, err)
		}
		email, err := convertMessage(msg)
		if err != nil {
			return nil, fmt.Errorf("failed to convert message %s: %w", id, err)
		}
		emails = append(emails, email)
	}
	return emails, nil
}

// MoveEmail removes sourceLabelID and adds targetLabelID on the message.
func (c *GmailClient) MoveEmail(emailID, sourceLabelID, targetLabelID string) error {
	req := &gmail.ModifyMessageRequest{
		AddLabelIds:    []string{targetLabelID},
		RemoveLabelIds: []string{sourceLabelID},
	}
	_, err := c.svc.Users.Messages.Modify(c.user, emailID, req).Do()
	if err != nil {
		return fmt.Errorf("failed to move message: %w", err)
	}
	return nil
}

// convertMessage converts a Gmail API message to our Email struct.
func convertMessage(msg *gmail.Message) (Email, error) {
	email := Email{
		ID:         msg.Id,
		MailboxIds: make(map[string]bool),
	}

	// internalDate is milliseconds since epoch
	ms := msg.InternalDate
	t := time.Unix(ms/1000, (ms%1000)*int64(time.Millisecond)).UTC()
	email.ReceivedAt = t.Format(time.RFC3339)

	// Label IDs → MailboxIds map
	for _, labelID := range msg.LabelIds {
		email.MailboxIds[labelID] = true
	}

	if msg.Payload == nil {
		return email, nil
	}

	// Extract subject from headers
	for _, header := range msg.Payload.Headers {
		if header.Name == "Subject" {
			email.Subject = header.Value
			break
		}
	}

	// Extract HTML body
	htmlContent := extractGmailHTML(msg.Payload)
	if htmlContent != "" {
		partID := "0"
		email.HTMLBody = []HTMLBodyPart{{PartID: partID, Type: "text/html"}}
		email.BodyValues = map[string]BodyValue{
			partID: {Value: htmlContent, IsHTML: false},
		}
	}

	return email, nil
}

// extractGmailHTML walks the message payload to find the text/html part.
func extractGmailHTML(part *gmail.MessagePart) string {
	if part == nil {
		return ""
	}
	if part.MimeType == "text/html" && part.Body != nil && part.Body.Data != "" {
		decoded, err := base64.URLEncoding.DecodeString(part.Body.Data)
		if err != nil {
			// Try standard encoding as fallback
			decoded, err = base64.StdEncoding.DecodeString(part.Body.Data)
			if err != nil {
				return ""
			}
		}
		return string(decoded)
	}
	for _, subPart := range part.Parts {
		if html := extractGmailHTML(subPart); html != "" {
			return html
		}
	}
	return ""
}

```

- [ ] **Step 2: Verify the build compiles**

```bash
go build ./...
```

Expected: compiles without errors.

- [ ] **Step 3: Run existing tests**

```bash
go test ./...
```

Expected: all existing tests PASS (they use the mock client, not GmailClient).

- [ ] **Step 4: Commit**

```bash
git add gmail.go
git commit -m "feat: add GmailClient implementing EmailClient"
```

---

### Task 4: Update `main.go` — switch from JMAP to Gmail

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Update `main.go`**

Replace this block in `main()`:

```go
	// Get API key from environment
	apiKey := os.Getenv("FASTMAIL_AAR_KEY")
	if apiKey == "" {
		log.Fatal("FASTMAIL_AAR_KEY environment variable is required")
	}

	fmt.Println("Starting email screenshot generator...")

	// Create JMAP client
	client, err := NewJMAPClient(apiKey)
	if err != nil {
		log.Fatalf("Failed to create JMAP client: %v", err)
	}
	fmt.Println("✓ Connected to JMAP server")
```

With:

```go
	// Get Gmail configuration from environment
	serviceAccountKey := os.Getenv("GOOGLE_SERVICE_ACCOUNT_KEY")
	if serviceAccountKey == "" {
		log.Fatal("GOOGLE_SERVICE_ACCOUNT_KEY environment variable is required")
	}
	gmailUser := os.Getenv("GMAIL_USER_EMAIL")
	if gmailUser == "" {
		log.Fatal("GMAIL_USER_EMAIL environment variable is required")
	}

	fmt.Println("Starting email screenshot generator...")

	// Create Gmail client
	client, err := NewGmailClient(serviceAccountKey, gmailUser)
	if err != nil {
		log.Fatalf("Failed to create Gmail client: %v", err)
	}
	fmt.Println("✓ Connected to Gmail API")
```

- [ ] **Step 2: Run `go fmt`**

```bash
go fmt ./...
```

- [ ] **Step 3: Verify the build compiles**

```bash
go build ./...
```

Expected: compiles without errors.

- [ ] **Step 4: Run tests**

```bash
go test ./...
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add main.go
git commit -m "feat: switch main.go from JMAP/Fastmail to Gmail"
```

---

### Task 5: Delete `jmap.go`

**Files:**
- Delete: `jmap.go`

- [ ] **Step 1: Delete the file**

```bash
rm jmap.go
```

- [ ] **Step 2: Verify the build still compiles**

```bash
go build ./...
```

Expected: compiles without errors. All JMAP types have moved to `types.go`; all JMAP logic is replaced by `gmail.go`.

- [ ] **Step 3: Run tests**

```bash
go test ./...
```

Expected: all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add -u jmap.go
git commit -m "chore: remove jmap.go (replaced by gmail.go)"
```

---

### Task 6: Update `README.md`

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Replace README.md content**

Replace the full contents of `README.md` with:

```markdown
# Email Screenshot Generator

A command-line Go application that reads emails from a specified Gmail label, generates screenshots of each email's content, and automatically moves processed emails to an archive label.

## Features

- Gmail API access via Google Workspace service account
- Batch processing of multiple emails
- Screenshot generation with customizable dimensions
- Automatic email archiving after processing
- Dry-run mode for testing
- Detailed progress logging

## Prerequisites

- Go 1.19 or later
- Chrome/Chromium browser (for headless screenshot generation)
- Google Workspace account
- A service account with domain-wide delegation enabled

## Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd aar
```

2. Install dependencies:
```bash
go mod download
```

3. Set the required environment variables (see Configuration below).

## Google Workspace Setup

### 1. Create a Service Account

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create or select a project
3. Enable the Gmail API
4. Go to **IAM & Admin > Service Accounts** and create a service account
5. Create and download a JSON key for the service account

### 2. Enable Domain-Wide Delegation

1. In Google Cloud Console, open the service account and enable **Domain-wide delegation**
2. In [Google Workspace Admin Console](https://admin.google.com/), go to **Security > API Controls > Domain-wide Delegation**
3. Add the service account's client ID with the scope: `https://www.googleapis.com/auth/gmail.modify`

### 3. Create Gmail Labels

Create two labels in the target Gmail account:

- **`_aar`** — Source label containing emails to process
- **`_aar_processed`** — Archive label for processed emails

## Configuration

| Setting | Description |
|---------|-------------|
| `GOOGLE_SERVICE_ACCOUNT_KEY` | Path to service account JSON key file (required) |
| `GMAIL_USER_EMAIL` | Gmail address of the user to impersonate (required) |
| Screenshot directory | `./screenshots` (constant in code) |
| Screenshot width | `1280` px (constant in code) |
| Screenshot height | `800` px (constant in code) |
| Source label | `_aar` (constant in code) |
| Archive label | `_aar_processed` (constant in code) |

Example `.env` or shell exports:

```bash
export GOOGLE_SERVICE_ACCOUNT_KEY=/path/to/service-account.json
export GMAIL_USER_EMAIL=user@yourdomain.com
```

## Usage

### Basic Usage

Process all emails in the source label:
```bash
go run .
```

Or build and run:
```bash
go build -o aar
./aar
```

### Command-Line Flags

**Limit the number of emails to process:**
```bash
./aar -limit 10
```

**Dry-run mode (preview without making changes):**
```bash
./aar -dry-run
```

**Combine flags:**
```bash
./aar -limit 5 -dry-run
```

## Output

Screenshots are saved to `./screenshots` with the naming format:
```
yyyy-mm-dd-hh-mm-ss-<emailID>.png
```
Timestamps are converted from UTC to New York time (America/New_York).

Example output:
```
Starting email screenshot generator...
✓ Connected to Gmail API
Found 5 email(s) in folder '_aar'

Processing email 1/5 (ID: 18c1a2b3d4e5f6a7)...
  Subject: Welcome to our service
  ✓ Screenshot generated: screenshots/2025-10-24-10-30-45-18c1a2b3d4e5f6a7.png
  ✓ Moved to archive folder

=== Summary ===
Total emails: 5
Successfully processed: 5
Failed: 0
```

## Project Structure

```
.
├── main.go           # Main application and orchestration
├── gmail.go          # Gmail API client implementation
├── types.go          # Shared data types
├── interfaces.go     # EmailClient and ScreenshotService interfaces
├── screenshot.go     # Screenshot generation via headless Chrome
├── go.mod            # Go module dependencies
└── README.md         # This file
```

## Running Tests

```bash
go test ./...
```

## Troubleshooting

**"GOOGLE_SERVICE_ACCOUNT_KEY environment variable is required"**
- Set `GOOGLE_SERVICE_ACCOUNT_KEY` to the path of your service account JSON key file.

**"GMAIL_USER_EMAIL environment variable is required"**
- Set `GMAIL_USER_EMAIL` to the Gmail address of the user to impersonate.

**"Failed to create Gmail client"**
- Verify the service account JSON key file exists and is readable.
- Verify the service account has domain-wide delegation configured.
- Verify the Gmail API is enabled in your Google Cloud project.

**"label '_aar' not found"**
- Create the `_aar` label in the target Gmail account.

**"label '_aar_processed' not found"**
- Create the `_aar_processed` label in the target Gmail account.

**"Failed to generate screenshot"**
- Ensure Chrome/Chromium is installed on your system.

## License

MIT
```

- [ ] **Step 2: Run `go fmt`** (no-op for markdown, but verify go code is clean)

```bash
go fmt ./...
```

- [ ] **Step 3: Run tests**

```bash
go test ./...
```

Expected: all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: update README for Gmail/service account setup"
```
