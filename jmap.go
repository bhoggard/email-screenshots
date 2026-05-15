package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	jmapServerURL = "https://api.fastmail.com/jmap/session"
)

// JMAPClient handles JMAP API interactions
type JMAPClient struct {
	apiKey     string
	accountID  string
	apiURL     string
	httpClient *http.Client
}

// SessionResponse represents the JMAP session response
type SessionResponse struct {
	Accounts        map[string]Account `json:"accounts"`
	PrimaryAccounts map[string]string  `json:"primaryAccounts"`
	ApiURL          string             `json:"apiUrl"`
}

// Account represents a JMAP account
type Account struct {
	Name string `json:"name"`
}

// MailboxQueryResponse represents the response to a Mailbox/query
type MailboxQueryResponse struct {
	MethodResponses [][]interface{} `json:"methodResponses"`
}

// EmailQueryResponse represents the response to an Email/query
type EmailQueryResponse struct {
	MethodResponses [][]interface{} `json:"methodResponses"`
}

// EmailGetResponse represents the response to an Email/get
type EmailGetResponse struct {
	MethodResponses [][]interface{} `json:"methodResponses"`
}

// NewJMAPClient creates a new JMAP client
func NewJMAPClient(apiKey string) (*JMAPClient, error) {
	client := &JMAPClient{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}

	if err := client.authenticate(); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	return client, nil
}

// authenticate establishes a session with the JMAP server
func (c *JMAPClient) authenticate() error {
	req, err := http.NewRequest("GET", jmapServerURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to JMAP server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("authentication failed with status %d: %s", resp.StatusCode, string(body))
	}

	var session SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return fmt.Errorf("failed to decode session response: %w", err)
	}

	// Get the primary account ID
	accountID, ok := session.PrimaryAccounts["urn:ietf:params:jmap:mail"]
	if !ok {
		return fmt.Errorf("no primary mail account found")
	}

	c.accountID = accountID
	c.apiURL = session.ApiURL

	return nil
}

// makeRequest makes a JMAP API request
func (c *JMAPClient) makeRequest(methodCalls []interface{}) ([]byte, error) {
	requestBody := map[string]interface{}{
		"using": []string{
			"urn:ietf:params:jmap:core",
			"urn:ietf:params:jmap:mail",
		},
		"methodCalls": methodCalls,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// FindMailboxByName finds a mailbox by name
func (c *JMAPClient) FindMailboxByName(name string) (*Mailbox, error) {
	methodCalls := []interface{}{
		[]interface{}{
			"Mailbox/query",
			map[string]interface{}{
				"accountId": c.accountID,
				"filter": map[string]interface{}{
					"name": name,
				},
			},
			"0",
		},
		[]interface{}{
			"Mailbox/get",
			map[string]interface{}{
				"accountId": c.accountID,
				"#ids": map[string]interface{}{
					"resultOf": "0",
					"name":     "Mailbox/query",
					"path":     "/ids",
				},
			},
			"1",
		},
	}

	responseData, err := c.makeRequest(methodCalls)
	if err != nil {
		return nil, err
	}

	var response struct {
		MethodResponses [][]interface{} `json:"methodResponses"`
	}

	if err := json.Unmarshal(responseData, &response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.MethodResponses) < 2 {
		return nil, fmt.Errorf("unexpected response format")
	}

	// Parse the Mailbox/get response
	getResponseData, err := json.Marshal(response.MethodResponses[1][1])
	if err != nil {
		return nil, err
	}

	var getResponse struct {
		List []Mailbox `json:"list"`
	}

	if err := json.Unmarshal(getResponseData, &getResponse); err != nil {
		return nil, fmt.Errorf("failed to decode mailbox response: %w", err)
	}

	if len(getResponse.List) == 0 {
		return nil, fmt.Errorf("mailbox '%s' not found", name)
	}

	return &getResponse.List[0], nil
}

// GetEmailsInMailbox retrieves emails from a specific mailbox
func (c *JMAPClient) GetEmailsInMailbox(mailboxID string, limit int) ([]string, error) {
	queryArgs := map[string]interface{}{
		"accountId": c.accountID,
		"filter": map[string]interface{}{
			"inMailbox": mailboxID,
		},
	}

	if limit > 0 {
		queryArgs["limit"] = limit
	}

	methodCalls := []interface{}{
		[]interface{}{
			"Email/query",
			queryArgs,
			"0",
		},
	}

	responseData, err := c.makeRequest(methodCalls)
	if err != nil {
		return nil, err
	}

	var response struct {
		MethodResponses [][]interface{} `json:"methodResponses"`
	}

	if err := json.Unmarshal(responseData, &response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.MethodResponses) == 0 {
		return nil, fmt.Errorf("unexpected response format")
	}

	// Parse the Email/query response
	queryResponseData, err := json.Marshal(response.MethodResponses[0][1])
	if err != nil {
		return nil, err
	}

	var queryResponse struct {
		IDs []string `json:"ids"`
	}

	if err := json.Unmarshal(queryResponseData, &queryResponse); err != nil {
		return nil, fmt.Errorf("failed to decode query response: %w", err)
	}

	return queryResponse.IDs, nil
}

// GetEmails retrieves email details
func (c *JMAPClient) GetEmails(emailIDs []string) ([]Email, error) {
	methodCalls := []interface{}{
		[]interface{}{
			"Email/get",
			map[string]interface{}{
				"accountId": c.accountID,
				"ids":       emailIDs,
				"properties": []string{
					"id",
					"subject",
					"receivedAt",
					"from",
					"htmlBody",
					"bodyValues",
					"mailboxIds",
				},
				"fetchHTMLBodyValues": true,
			},
			"0",
		},
	}

	responseData, err := c.makeRequest(methodCalls)
	if err != nil {
		return nil, err
	}

	var response struct {
		MethodResponses [][]interface{} `json:"methodResponses"`
	}

	if err := json.Unmarshal(responseData, &response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.MethodResponses) == 0 {
		return nil, fmt.Errorf("unexpected response format")
	}

	// Parse the Email/get response
	getResponseData, err := json.Marshal(response.MethodResponses[0][1])
	if err != nil {
		return nil, err
	}

	var getResponse struct {
		List []Email `json:"list"`
	}

	if err := json.Unmarshal(getResponseData, &getResponse); err != nil {
		return nil, fmt.Errorf("failed to decode email response: %w", err)
	}

	return getResponse.List, nil
}

// MoveEmail moves an email to a different mailbox
func (c *JMAPClient) MoveEmail(emailID, sourceMailboxID, targetMailboxID string) error {
	methodCalls := []interface{}{
		[]interface{}{
			"Email/set",
			map[string]interface{}{
				"accountId": c.accountID,
				"update": map[string]interface{}{
					emailID: map[string]interface{}{
						"mailboxIds/" + sourceMailboxID: nil,
						"mailboxIds/" + targetMailboxID: true,
					},
				},
			},
			"0",
		},
	}

	responseData, err := c.makeRequest(methodCalls)
	if err != nil {
		return err
	}

	var response struct {
		MethodResponses [][]interface{} `json:"methodResponses"`
	}

	if err := json.Unmarshal(responseData, &response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.MethodResponses) == 0 {
		return fmt.Errorf("unexpected response format")
	}

	// Check if the response is an error
	if len(response.MethodResponses[0]) > 0 {
		if methodName, ok := response.MethodResponses[0][0].(string); ok && methodName == "error" {
			errorData, _ := json.Marshal(response.MethodResponses[0][1])
			var errorResp struct {
				Type        string `json:"type"`
				Description string `json:"description"`
			}
			if err := json.Unmarshal(errorData, &errorResp); err == nil {
				if errorResp.Type == "accountReadOnly" {
					return fmt.Errorf("API key has read-only permissions. Please create a new Fastmail API token with read-write permissions for Mail")
				}
				return fmt.Errorf("JMAP error (%s): %s", errorResp.Type, errorResp.Description)
			}
			return fmt.Errorf("JMAP error: %s", string(errorData))
		}
	}

	// Parse successful response
	setResponseData, err := json.Marshal(response.MethodResponses[0][1])
	if err != nil {
		return err
	}

	var setResponse struct {
		Updated    map[string]interface{} `json:"updated"`
		NotUpdated map[string]interface{} `json:"notUpdated"`
	}

	if err := json.Unmarshal(setResponseData, &setResponse); err != nil {
		return fmt.Errorf("failed to decode set response: %w", err)
	}

	if notUpdated, ok := setResponse.NotUpdated[emailID]; ok {
		errData, _ := json.Marshal(notUpdated)
		return fmt.Errorf("failed to move email: %s", string(errData))
	}

	return nil
}
