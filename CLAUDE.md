# Email Screenshot Generator - CLAUDE.md

## Project Overview

A command-line Go application that reads emails from a specified Gmail label, generates screenshots of each email's content, and automatically moves processed emails to an archive label.

## Core Functionality

### Primary Goals

1. **Email Reading**: Connect to Gmail API using a Google Workspace service account
1. **Email Processing**: Retrieve emails from a configured source label
1. **Screenshot Generation**: Convert each email's HTML content to a PNG screenshot
1. **Email Management**: Move processed emails to a designated archive label
1. **Progress Tracking**: Display progress and logging for all operations

### Key Features

- Gmail API access via Google Workspace service account with domain-wide delegation
- Batch processing of multiple emails with pagination
- Configurable source and destination labels
- Screenshot generation with customizable dimensions
- Error handling for failed emails
- Detailed logging of all operations

## Technical Architecture

### Dependencies

- `google.golang.org/api/gmail/v1` - Gmail API client
- `golang.org/x/oauth2/google` - Google OAuth2 / service account authentication
- `github.com/chromedp/chromedp` - Headless Chrome for screenshot generation
- Standard library packages: `os`, `fmt`, `flag`, `log`, `context`, `encoding/base64`, `time`

### Key Components

**Gmail Client** (`gmail.go`)

- Authenticates using a service account JSON key file with domain-wide delegation
- Impersonates a Google Workspace user to access their Gmail
- Uses `gmail.GmailModifyScope` for read + label modification
- Implements the `EmailClient` interface

**Email Processor** (`main.go`)

- Creates screenshot directory if it does not already exist
- Retrieves email list from source label named `_aar`
- Processes each email: fetch details, generate screenshot, move to archive
- Handles pagination for large email sets via Gmail API `Pages()` method

**Screenshot Generator** (`screenshot.go`)

- Uses headless Chrome (Chromedp) to render HTML content
- Converts email HTML to PNG screenshots
- Saves screenshots with timestamp and email ID as filename (format: `yyyy-mm-dd-hh-mm-ss-<emailID>.png`)
- Converts UTC timestamps to New York timezone (America/New_York) for filenames

**Shared Types** (`types.go`)

- `Email`, `Mailbox`, `EmailAddress`, `HTMLBodyPart`, `BodyValue`

**Interfaces** (`interfaces.go`)

- `EmailClient` — defines the interface for email operations (find labels, list/get/move emails)
- `ScreenshotService` — defines the interface for screenshot generation

## Configuration

### Environment Variables

```
GOOGLE_SERVICE_ACCOUNT_KEY=/path/to/service-account.json
GMAIL_USER_EMAIL=user@yourdomain.com
```

### Command-Line Flags

```
-limit int         Maximum emails to process (default: 0 = all)
-dry-run bool      Preview operations without making changes
```

## Usage

### Basic Setup

1. Set up a Google Cloud service account with Gmail API enabled and domain-wide delegation
1. Download the service account JSON key file
1. Set environment variables: `GOOGLE_SERVICE_ACCOUNT_KEY` and `GMAIL_USER_EMAIL`
1. Create `_aar` and `_aar_processed` labels in the target Gmail account
1. Build the application: `go build -o aar`
1. Run: `./aar`

### Dry-Run Mode

Test your configuration without modifying emails:

```bash
./aar -dry-run
```

## Data Flow

1. **Authentication** → Load service account key, impersonate `GMAIL_USER_EMAIL`
1. **Label Discovery** → Find `_aar` and `_aar_processed` label IDs via `labels.list`
1. **Email Retrieval** → List messages with `_aar` label (paginated)
1. **Processing Loop**:
   - Fetch full message via `messages.get` with `format=FULL`
   - Extract subject from headers, HTML body from MIME parts (base64url-decoded)
   - Convert `internalDate` (ms epoch) to RFC3339, then to New York timezone
   - Generate screenshot via Chromedp
   - Save screenshot to output directory with timestamp-emailID filename
   - Move email: remove `_aar` label, add `_aar_processed` label
   - Log completion status
1. **Completion Report** → Display summary of processed emails

## Error Handling

- Auth failure: Exit with clear error message
- Label not found: Exit with clear error message
- Screenshot generation failures: Log error and continue with next email
- Per-email failures: Log and continue to next email

## Expected Output

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

## Development Notes

### Code Formatting

**IMPORTANT: When working on this project, ALWAYS run `go fmt ./...` after making any changes to Go files.**

- Run `go fmt ./...` before running tests
- Run `go fmt ./...` before committing code
- All Go code must be properly formatted according to Go standards
- Use `go test -v` to verify tests pass after formatting

### Testing Considerations

- Mock EmailClient interface for unit testing (see `main_test.go`)
- Use test fixtures with sample HTML emails
- Validate screenshot file creation and naming
- Verify label move operations

### Security Best Practices

- Never log credentials or service account key contents
- Use environment variables for sensitive data
- Store service account JSON key files securely (not in version control)
