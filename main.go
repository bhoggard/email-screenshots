package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
)

const (
	sourceFolder     = "_aar"
	archiveFolder    = "_aar_processed"
	screenshotDir    = "./screenshots"
	screenshotWidth  = 1280
	screenshotHeight = 800
)

var (
	limit  = flag.Int("limit", 0, "Maximum emails to process (default: 0 = all)")
	dryRun = flag.Bool("dry-run", false, "Preview operations without making changes")
)

// ProcessResult contains the results of processing emails
type ProcessResult struct {
	TotalCount     int
	ProcessedCount int
	FailedCount    int
}

func main() {
	flag.Parse()

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

	// Create screenshot generator
	generator, err := NewScreenshotGenerator(screenshotDir, screenshotWidth, screenshotHeight)
	if err != nil {
		log.Fatalf("Failed to create screenshot generator: %v", err)
	}

	// Process emails
	result, err := processEmails(client, generator, *limit, *dryRun, os.Stdout)
	if err != nil {
		log.Fatalf("Failed to process emails: %v", err)
	}

	// Print summary
	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Total emails: %d\n", result.TotalCount)
	fmt.Printf("Successfully processed: %d\n", result.ProcessedCount)
	fmt.Printf("Failed: %d\n", result.FailedCount)
}

// processEmails processes emails from source to archive folder
func processEmails(client EmailClient, generator ScreenshotService, limit int, dryRun bool, output io.Writer) (*ProcessResult, error) {
	// Find source mailbox
	sourceMailbox, err := client.FindMailboxByName(sourceFolder)
	if err != nil {
		return nil, fmt.Errorf("failed to find source folder '%s': %w", sourceFolder, err)
	}

	// Find archive mailbox
	archiveMailbox, err := client.FindMailboxByName(archiveFolder)
	if err != nil {
		return nil, fmt.Errorf("failed to find archive folder '%s': %w", archiveFolder, err)
	}

	// Get emails from source folder
	emailIDs, err := client.GetEmailsInMailbox(sourceMailbox.ID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve emails: %w", err)
	}

	emailCount := len(emailIDs)
	if emailCount == 0 {
		fmt.Fprintf(output, "No emails found in folder '%s'\n", sourceFolder)
		return &ProcessResult{TotalCount: 0, ProcessedCount: 0, FailedCount: 0}, nil
	}

	fmt.Fprintf(output, "Found %d email(s) in folder '%s'\n", emailCount, sourceFolder)

	if dryRun {
		fmt.Fprintln(output, "\nDRY RUN MODE - No changes will be made")
		fmt.Fprintf(output, "Would process %d emails:\n", emailCount)
		for i, id := range emailIDs {
			fmt.Fprintf(output, "  %d. Email ID: %s\n", i+1, id)
		}
		return &ProcessResult{TotalCount: emailCount, ProcessedCount: 0, FailedCount: 0}, nil
	}

	// Process emails
	var processedCount, failedCount int
	for i, emailID := range emailIDs {
		fmt.Fprintf(output, "\nProcessing email %d/%d (ID: %s)...\n", i+1, emailCount, emailID)

		// Get email details
		emails, err := client.GetEmails([]string{emailID})
		if err != nil {
			fmt.Fprintf(output, "  ✗ Failed to fetch email: %v\n", err)
			failedCount++
			continue
		}

		if len(emails) == 0 {
			fmt.Fprintln(output, "  ✗ Email not found")
			failedCount++
			continue
		}

		email := emails[0]
		fmt.Fprintf(output, "  Subject: %s\n", email.Subject)

		// Extract HTML content
		htmlContent := extractHTMLContent(email)
		if htmlContent == "" {
			fmt.Fprintln(output, "  ✗ No HTML content found")
			failedCount++
			continue
		}

		// Generate screenshot
		screenshotPath, err := generator.GenerateScreenshot(email.ReceivedAt, emailID, htmlContent)
		if err != nil {
			fmt.Fprintf(output, "  ✗ Failed to generate screenshot: %v\n", err)
			failedCount++
			continue
		}
		fmt.Fprintf(output, "  ✓ Screenshot generated: %s\n", screenshotPath)

		// Move email to archive folder
		if err := client.MoveEmail(emailID, sourceMailbox.ID, archiveMailbox.ID); err != nil {
			fmt.Fprintf(output, "  ✗ Failed to move email to archive: %v\n", err)
			failedCount++
			continue
		}
		fmt.Fprintln(output, "  ✓ Moved to archive folder")

		processedCount++
	}

	return &ProcessResult{
		TotalCount:     emailCount,
		ProcessedCount: processedCount,
		FailedCount:    failedCount,
	}, nil
}

// extractHTMLContent extracts HTML content from an email
func extractHTMLContent(email Email) string {
	if len(email.HTMLBody) == 0 {
		return ""
	}

	// Get the first HTML body part
	partID := email.HTMLBody[0].PartID

	// Get the body value
	if bodyValue, ok := email.BodyValues[partID]; ok {
		return bodyValue.Value
	}

	return ""
}
