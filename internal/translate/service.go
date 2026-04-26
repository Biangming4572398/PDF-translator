package translate

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"pdftranslator/internal/config"
	"pdftranslator/internal/pdf"
)

var ErrProviderNotFound = errors.New("translation provider not found")

type Registry struct {
	providers map[string]Provider
}

func NewRegistry(providers ...Provider) *Registry {
	items := make(map[string]Provider, len(providers))
	for _, provider := range providers {
		items[provider.Name()] = provider
	}

	return &Registry{providers: items}
}

func (r *Registry) Get(name string) (Provider, bool) {
	provider, ok := r.providers[name]
	return provider, ok
}

func (r *Registry) Descriptors() []ProviderDescriptor {
	items := make([]ProviderDescriptor, 0, len(r.providers))
	for _, provider := range r.providers {
		items = append(items, ProviderDescriptor{
			Name:        provider.Name(),
			DisplayName: provider.DisplayName(),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].DisplayName < items[j].DisplayName
	})

	return items
}

type Service struct {
	registry       *Registry
	promptBuilder  *PromptBuilder
	modelDiscovery *ModelDiscoveryClient
	maxAttempts    int
	retryDelay     time.Duration
}

func NewService(registry *Registry, promptBuilder *PromptBuilder) *Service {
	return &Service{
		registry:       registry,
		promptBuilder:  promptBuilder,
		modelDiscovery: NewModelDiscoveryClient(),
		maxAttempts:    2,
		retryDelay:     2 * time.Second,
	}
}

func (s *Service) Descriptors() []ProviderDescriptor {
	return s.registry.Descriptors()
}

func (s *Service) FetchModels(ctx context.Context, providerName string, providerConfig config.ProviderConfig) ([]ModelDescriptor, error) {
	if providerName != "openai-compatible" {
		return nil, ErrModelDiscoveryUnavailable
	}

	return s.modelDiscovery.Fetch(ctx, providerConfig)
}

func (s *Service) Translate(ctx context.Context, providerName, sourceLanguage, targetLanguage string, providerConfig config.ProviderConfig, doc pdf.SourceDocument) (Result, error) {
	provider, ok := s.registry.Get(providerName)
	if !ok {
		return Result{}, fmt.Errorf("%w: %s", ErrProviderNotFound, providerName)
	}

	prompt, err := s.promptBuilder.Build(doc, sourceLanguage, targetLanguage)
	if err != nil {
		return Result{}, err
	}

	request := Request{
		ProviderName:   providerName,
		SourceLanguage: sourceLanguage,
		TargetLanguage: targetLanguage,
		Prompt:         prompt,
		ProviderConfig: providerConfig,
		Document:       doc,
	}

	var lastErr error
	for attempt := 1; attempt <= s.maxAttempts; attempt++ {
		response, err := provider.Translate(ctx, request)
		if err == nil {
			normalized, warnings, normalizeErr := normalizeResponse(response)
			if normalizeErr != nil {
				return Result{}, normalizeErr
			}

			return Result{
				Prompt:   prompt,
				Response: normalized,
				Attempts: attempt,
				Warnings: warnings,
			}, nil
		}

		lastErr = err
		if !isRetryable(err) || attempt == s.maxAttempts {
			break
		}

		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		case <-time.After(s.retryDelay):
		}
	}

	return Result{}, lastErr
}

func normalizeResponse(response Response) (Response, []string, error) {
	warnings := make([]string, 0, 2)
	content := strings.TrimSpace(response.LatexCandidate)
	if content == "" {
		content = strings.TrimSpace(response.RawContent)
		warnings = append(warnings, "provider response did not include an explicit LaTeX candidate; raw content was used")
	}

	content = stripCodeFence(content)
	if content == "" {
		return Response{}, nil, errors.New("provider response was empty after normalization")
	}

	if looksLikeRefusal(content) {
		return Response{}, nil, errors.New("provider response looked like a refusal instead of LaTeX output")
	}

	response.LatexCandidate = content
	response.RawContent = content
	response.FullDocument = strings.Contains(content, `\documentclass`) || strings.Contains(content, `\begin{document}`)
	if response.Metadata == nil {
		response.Metadata = map[string]string{}
	}

	return response, warnings, nil
}

func stripCodeFence(input string) string {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "```") {
		return input
	}

	trimmed := strings.TrimPrefix(input, "```latex")
	trimmed = strings.TrimPrefix(trimmed, "```tex")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)
	trimmed = strings.TrimSuffix(trimmed, "```")
	return strings.TrimSpace(trimmed)
}

func looksLikeRefusal(input string) bool {
	lowered := strings.ToLower(input)
	return (strings.Contains(lowered, "i cannot") || strings.Contains(lowered, "i'm sorry")) &&
		!strings.Contains(input, `\begin{`) &&
		!strings.Contains(input, `\section`)
}

func isRetryable(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) || errors.Is(err, context.DeadlineExceeded)
}
