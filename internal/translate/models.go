package translate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"pdftranslator/internal/config"
)

var ErrModelDiscoveryUnavailable = errors.New("model discovery is not available for this provider")

type ModelDescriptor struct {
	ID          string
	Name        string
	Description string
}

type ModelDiscoveryClient struct {
	httpClient *http.Client
}

func NewModelDiscoveryClient() *ModelDiscoveryClient {
	return &ModelDiscoveryClient{
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *ModelDiscoveryClient) Fetch(ctx context.Context, cfg config.ProviderConfig) ([]ModelDescriptor, error) {
	endpoint, err := modelsEndpoint(cfg.BaseURL)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if strings.TrimSpace(cfg.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.APIKey))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("model discovery failed: %s", resp.Status)
	}

	var payload struct {
		Data []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	models := make([]ModelDescriptor, 0, len(payload.Data))
	for _, item := range payload.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		models = append(models, ModelDescriptor{
			ID:          id,
			Name:        strings.TrimSpace(item.Name),
			Description: strings.TrimSpace(item.Description),
		})
	}

	sortModels(models, cfg.BaseURL)
	return models, nil
}

func modelsEndpoint(baseURL string) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = config.DefaultOpenRouterBaseURL
	}
	if !strings.Contains(baseURL, "://") {
		baseURL = "https://" + baseURL
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("base URL is missing a host: %q", baseURL)
	}

	if strings.Contains(strings.ToLower(parsed.Host), "openrouter.ai") {
		parsed.Scheme = "https"
		parsed.Host = "openrouter.ai"
		if !strings.HasPrefix(parsed.Path, "/api/v1") {
			parsed.Path = "/api/v1"
		}
	}

	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if strings.HasSuffix(parsed.Path, "/models") {
		return parsed.String(), nil
	}

	parsed.Path += "/models"
	return parsed.String(), nil
}

func sortModels(models []ModelDescriptor, baseURL string) {
	openRouter := strings.Contains(strings.ToLower(baseURL), "openrouter.ai")
	sort.SliceStable(models, func(i, j int) bool {
		leftDeepSeek := strings.HasPrefix(strings.ToLower(models[i].ID), "deepseek/")
		rightDeepSeek := strings.HasPrefix(strings.ToLower(models[j].ID), "deepseek/")
		if openRouter && leftDeepSeek != rightDeepSeek {
			return leftDeepSeek
		}
		return strings.ToLower(models[i].ID) < strings.ToLower(models[j].ID)
	})
}
