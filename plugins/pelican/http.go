package pelican

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

type HttpClient struct {
	Token string
	Url   string
}

func NewHttpClient(token, url string) *HttpClient {
	return &HttpClient{
		Token: token,
		Url:   url,
	}
}

// buildUrl combines the base URL with the provided endpoint path.
func (c *HttpClient) buildUrl(endpoint string) (string, string, error) {
	base, err := url.Parse(c.Url)
	if err != nil {
		return "", "", err
	}
	base.Path = path.Join("api", "client", endpoint)
	host := base.Host
	// Remove port if present for Host header, if you want only the domain
	if colon := strings.Index(host, ":"); colon != -1 {
		host = host[:colon]
	}
	return base.String(), host, nil
}

// Get sends a GET request to the specified endpoint and returns the response body.
func (c *HttpClient) Get(endpoint string) ([]byte, error) {
	fullUrl, host, err := c.buildUrl(endpoint)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", fullUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))
	req.Host = host

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Printf("Error closing response body: %v\n", err)
		}
	}(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s returned status %d", fullUrl, resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// Post sends a POST request with a JSON body to the specified endpoint and returns the response body.
func (c *HttpClient) Post(endpoint string, body interface{}) ([]byte, error) {
	fullUrl, host, err := c.buildUrl(endpoint)
	if err != nil {
		return nil, err
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fullUrl, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))
	req.Header.Set("Content-Type", "application/json")
	req.Host = host

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Printf("Error closing response body: %v\n", err)
		}
	}(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("POST %s returned status %d", fullUrl, resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func (c *HttpClient) StartServer(server string) error {
	endpoint := fmt.Sprintf("servers/%s/power", server)
	body := map[string]interface{}{
		"signal": "start",
	}
	_, err := c.Post(endpoint, body)
	if err != nil {
		return fmt.Errorf("error starting server %s: %w", server, err)
	}
	return nil
}

func (c *HttpClient) StopServer(server string) error {
	endpoint := fmt.Sprintf("servers/%s/power", server)
	body := map[string]interface{}{
		"signal": "stop",
	}
	_, err := c.Post(endpoint, body)
	if err != nil {
		return fmt.Errorf("error stopping server %s: %w", server, err)
	}
	return nil
}
