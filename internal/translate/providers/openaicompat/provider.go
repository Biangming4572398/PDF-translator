package openaicompat

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"pdftranslator/internal/config"
	"pdftranslator/internal/translate"
)

const defaultTimeout = time.Duration(config.DefaultProviderTimeout) * time.Second

var ErrMissingAPIKey = errors.New("OpenRouter API key is missing")

type Provider struct {
	httpClient *http.Client
}

func New() *Provider {
	return &Provider{
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

func (p *Provider) Name() string {
	return "openai-compatible"
}

func (p *Provider) DisplayName() string {
	return "OpenRouter / OpenAI-Compatible"
}

func (p *Provider) Translate(ctx context.Context, request translate.Request) (translate.Response, error) {
	cfg := normalizeConfig(request.ProviderConfig)
	if strings.TrimSpace(cfg.APIKey) == "" {
		return translate.Response{}, ErrMissingAPIKey
	}

	endpoint, err := chatCompletionsEndpoint(cfg.BaseURL)
	if err != nil {
		return translate.Response{}, err
	}

	messages, attachment, err := buildMessages(request)
	if err != nil {
		return translate.Response{}, err
	}

	payload := chatCompletionsRequest{
		Model:       cfg.Model,
		Messages:    messages,
		Temperature: 0.1,
		Stream:      false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return translate.Response{}, err
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return translate.Response{}, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.APIKey))
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("HTTP-Referer", "https://localhost/pdftranslator")
	httpRequest.Header.Set("X-Title", config.AppName)

	client := p.httpClient
	if cfg.TimeoutSeconds > 0 {
		client = &http.Client{Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second}
	}

	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		return translate.Response{}, err
	}
	defer httpResponse.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(httpResponse.Body, 16*1024*1024))
	if err != nil {
		return translate.Response{}, err
	}

	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return translate.Response{}, newHTTPStatusError(httpResponse.StatusCode, httpResponse.Status, responseBody)
	}

	var completion chatCompletionsResponse
	if err := json.Unmarshal(responseBody, &completion); err != nil {
		return translate.Response{}, err
	}
	if len(completion.Choices) == 0 {
		return translate.Response{}, errors.New("provider response did not contain any choices")
	}

	content := strings.TrimSpace(completion.Choices[0].Message.Content)
	if content == "" {
		return translate.Response{}, errors.New("provider response message was empty")
	}

	model := completion.Model
	if strings.TrimSpace(model) == "" {
		model = cfg.Model
	}

	return translate.Response{
		Provider:       p.Name(),
		Model:          model,
		RawContent:     content,
		LatexCandidate: content,
		Metadata: map[string]string{
			"endpoint":              endpoint,
			"file_annotation_count": strconv.Itoa(len(completion.Choices[0].Message.Annotations)),
			"finish_reason":         completion.Choices[0].FinishReason,
			"pdf_attached":          strconv.FormatBool(attachment.Attached),
			"pdf_filename":          attachment.Filename,
			"pdf_raw_bytes":         strconv.Itoa(attachment.RawBytes),
			"pdf_transport":         attachment.Transport,
			"request_id":            completion.ID,
		},
	}, nil
}

type chatCompletionsRequest struct {
	Model       string        `json:"model,omitempty"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	Stream      bool          `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type contentPart struct {
	Type string       `json:"type"`
	Text string       `json:"text,omitempty"`
	File *fileContent `json:"file,omitempty"`
}

type fileContent struct {
	Filename string `json:"filename"`
	FileData string `json:"file_data"`
}

type chatCompletionsResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role        string            `json:"role"`
			Content     string            `json:"content"`
			Annotations []json.RawMessage `json:"annotations,omitempty"`
		} `json:"message"`
	} `json:"choices"`
}

type pdfAttachment struct {
	Attached  bool
	Filename  string
	RawBytes  int
	Transport string
}

type httpStatusError struct {
	statusCode int
	status     string
	body       string
}

func (e httpStatusError) Error() string {
	if e.body == "" {
		return "provider request failed: " + e.status
	}
	return fmt.Sprintf("provider request failed: %s\n%s", e.status, e.body)
}

func (e httpStatusError) Timeout() bool {
	return e.statusCode == http.StatusRequestTimeout
}

func (e httpStatusError) Temporary() bool {
	return e.statusCode == http.StatusRequestTimeout ||
		e.statusCode == http.StatusTooManyRequests ||
		e.statusCode >= 500
}

func normalizeConfig(cfg config.ProviderConfig) config.ProviderConfig {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = config.DefaultOpenRouterBaseURL
	}
	if strings.TrimSpace(cfg.Model) == "" {
		cfg.Model = config.DefaultOpenRouterModel
	}
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = int(defaultTimeout / time.Second)
	}
	return cfg
}

func buildMessages(request translate.Request) ([]chatMessage, pdfAttachment, error) {
	if len(request.Document.RawBytes) == 0 {
		return nil, pdfAttachment{}, errors.New("loaded PDF had no raw bytes to attach")
	}

	attachment := pdfAttachment{
		Attached:  true,
		Filename:  request.Document.Name,
		RawBytes:  len(request.Document.RawBytes),
		Transport: "data-url",
	}
	dataURL := "data:application/pdf;base64," + base64.StdEncoding.EncodeToString(request.Document.RawBytes)

	content := []contentPart{
		{
			Type: "text",
			Text: request.Prompt.User,
		},
		{
			Type: "file",
			File: &fileContent{
				Filename: request.Document.Name,
				FileData: dataURL,
			},
		},
	}

	return []chatMessage{
		{
			Role:    "system",
			Content: request.Prompt.System,
		},
		{
			Role:    "user",
			Content: content,
		},
	}, attachment, nil
}

func chatCompletionsEndpoint(baseURL string) (string, error) {
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
	if strings.HasSuffix(parsed.Path, "/chat/completions") {
		return parsed.String(), nil
	}
	if strings.HasSuffix(parsed.Path, "/chat") {
		parsed.Path += "/completions"
		return parsed.String(), nil
	}

	parsed.Path += "/chat/completions"
	return parsed.String(), nil
}

func newHTTPStatusError(statusCode int, status string, body []byte) httpStatusError {
	bodyText := strings.TrimSpace(string(body))
	if len(bodyText) > 4000 {
		bodyText = bodyText[:4000] + "\n..."
	}

	return httpStatusError{
		statusCode: statusCode,
		status:     status,
		body:       bodyText,
	}
}
