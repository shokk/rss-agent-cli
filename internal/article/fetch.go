package article

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

var jinaEndpoint = "https://r.jina.ai/%s"
var userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.3.1 Safari/605.1.15"
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

func SetJinaEndpointForTesting(endpoint string) string {
	old := jinaEndpoint
	jinaEndpoint = endpoint
	return old
}

// SetUserAgent sets the User-Agent header used when fetching articles.
func SetUserAgent(ua string) {
	userAgent = ua
}

func FetchArticle(ctx context.Context, articleURL string, noCache bool) (string, error) {
	jinaURL := fmt.Sprintf(jinaEndpoint, url.QueryEscape(articleURL))
	if noCache {
		jinaURL += "?no-cache=true"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jinaURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch content: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("jina reader API returned %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	content := string(body)
	if content == "" {
		return "", fmt.Errorf("received empty content from jina reader")
	}

	return content, nil
}
