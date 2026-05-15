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
