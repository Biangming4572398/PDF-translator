package translate

import (
	"context"

	"pdftranslator/internal/config"
	"pdftranslator/internal/pdf"
)

type Provider interface {
	Name() string
	DisplayName() string
	Translate(ctx context.Context, request Request) (Response, error)
}

type ProviderDescriptor struct {
	Name        string
	DisplayName string
}

type Request struct {
	ProviderName   string
	SourceLanguage string
	TargetLanguage string
	Prompt         Prompt
	ProviderConfig config.ProviderConfig
	Document       pdf.SourceDocument
}

type Prompt struct {
	System string
	User   string
}

type Response struct {
	Provider       string
	Model          string
	RawContent     string
	LatexCandidate string
	FullDocument   bool
	Metadata       map[string]string
}

type Result struct {
	Prompt   Prompt
	Response Response
	Attempts int
	Warnings []string
}
