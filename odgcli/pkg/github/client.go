package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type Client struct {
	httpClient    *http.Client
	apiUrl        string
	token         string
	username      string
	usernameCache map[string]string
}

func NewClient(apiUrl, token string) *Client {
	return &Client{
		apiUrl:        apiUrl,
		token:         token,
		usernameCache: make(map[string]string),
		httpClient:    &http.Client{},
	}
}

func (c *Client) makeAuthenticatedRequest(ctx context.Context, method, path string) (*http.Response, error) {
	reqURL, err := url.JoinPath(c.apiUrl, path)
	if err != nil {
		return nil, fmt.Errorf("failed to construct URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return resp, nil
}

func (c *Client) LoggedInUsername(ctx context.Context) (string, error) {
	if c.username != "" {
		return c.username, nil
	}
	resp, err := c.makeAuthenticatedRequest(ctx, http.MethodGet, "user")
	if err != nil {
		return "", err
	}

	defer resp.Body.Close() //nolint:errcheck

	var result struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Login, nil
}

// ResolveUsername resolves a GitHub username to the user's real name using the GitHub API
func (c *Client) ResolveUsername(ctx context.Context, username string) (string, error) {
	// Check cache first
	if realName, found := c.usernameCache[username]; found {
		return realName, nil
	}
	resp, err := c.makeAuthenticatedRequest(ctx, http.MethodGet, "users/"+username)
	if err != nil {
		return "", fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	var result struct {
		Name  string `json:"name"`
		Login string `json:"login"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Return the real name if available, otherwise return the username
	if result.Name != "" {
		c.usernameCache[username] = result.Name
		return result.Name, nil
	}
	return result.Login, nil
}

func (c *Client) GetURL() string {
	return c.apiUrl
}

func (c *Client) GetToken() string {
	return c.token
}
