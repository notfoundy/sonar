// Package sonar is a minimal HTTP client for the SonarQube web API.
package sonar

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to a SonarQube instance over HTTP.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// New returns a client for the given base URL.
func New(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 15 * time.Second},
	}
}

// Status returns the system status (UP, STARTING, DB_MIGRATION_*, DOWN) or an error if
// the server is unreachable.
func (c *Client) Status() (string, error) {
	resp, err := c.HTTP.Get(c.BaseURL + "/api/system/status")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	return body.Status, nil
}

// CheckAdmin reports whether admin/<pass> are valid credentials (used to make the
// bootstrap idempotent without storing anything locally).
func (c *Client) CheckAdmin(pass string) bool {
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+"/api/users/current", nil)
	if err != nil {
		return false
	}
	req.SetBasicAuth("admin", pass)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// ChangeAdminPassword swaps the admin password (authenticated with the previous one).
func (c *Client) ChangeAdminPassword(previous, next string) error {
	_, err := c.postForm("admin", previous, "/api/users/change_password", url.Values{
		"login":            {"admin"},
		"previousPassword": {previous},
		"password":         {next},
	})
	return err
}

// GenerateToken creates a global analysis token and returns its secret value.
func (c *Client) GenerateToken(adminPass, name string) (string, error) {
	data, err := c.postForm("admin", adminPass, "/api/user_tokens/generate", url.Values{
		"name": {name},
		"type": {"GLOBAL_ANALYSIS_TOKEN"},
	})
	if err != nil {
		return "", err
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return "", err
	}
	if body.Token == "" {
		return "", fmt.Errorf("empty token in response: %s", string(data))
	}
	return body.Token, nil
}

// RevokeToken deletes a previously generated token by name.
func (c *Client) RevokeToken(adminPass, name string) error {
	_, err := c.postForm("admin", adminPass, "/api/user_tokens/revoke", url.Values{
		"name": {name},
	})
	return err
}

func (c *Client) postForm(user, pass, endpoint string, form url.Values) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(user, pass)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s -> HTTP %d: %s", endpoint, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}
