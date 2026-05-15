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
