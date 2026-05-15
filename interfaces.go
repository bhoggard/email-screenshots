package main

// EmailClient defines the interface for email operations
type EmailClient interface {
	FindMailboxByName(name string) (*Mailbox, error)
	GetEmailsInMailbox(mailboxID string, limit int) ([]string, error)
	GetEmails(emailIDs []string) ([]Email, error)
	MoveEmail(emailID, sourceMailboxID, targetMailboxID string) error
}

// ScreenshotService defines the interface for screenshot generation
type ScreenshotService interface {
	GenerateScreenshot(timestamp, emailID, htmlContent string) (string, error)
}
