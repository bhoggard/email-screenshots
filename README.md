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
