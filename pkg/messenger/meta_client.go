package messenger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type MetaClient struct {
	accessToken   string
	phoneNumberID string
	http          *http.Client
}

func NewMetaClient(accessToken, phoneNumberID string) *MetaClient {
	return &MetaClient{
		accessToken:   accessToken,
		phoneNumberID: phoneNumberID,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *MetaClient) SendTextMessage(ctx context.Context, to string, body string) error {
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/messages", c.phoneNumberID)

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                to,
		"type":              "text",
		"text": map[string]string{
			"body": body,
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshalling payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request to Meta API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("meta API returned error status: %s", resp.Status)
	}

	return nil
}
