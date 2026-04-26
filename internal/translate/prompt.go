package translate

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"pdftranslator/internal/config"
	"pdftranslator/internal/pdf"
	"pdftranslator/prompts"
)

var unresolvedPlaceholderPattern = regexp.MustCompile(`\{\{[A-Z0-9_]+\}\}`)

type PromptBuilder struct{}

func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{}
}

func (b *PromptBuilder) Build(doc pdf.SourceDocument, sourceLanguage, targetLanguage string) (Prompt, error) {
	sourceLanguage = strings.TrimSpace(sourceLanguage)
	targetLanguage = strings.TrimSpace(targetLanguage)
	if !config.IsSupportedTranslationPair(sourceLanguage, targetLanguage) {
		return Prompt{}, fmt.Errorf("translation direction must be Chinese to English or English to Chinese, got %q to %q", sourceLanguage, targetLanguage)
	}

	templates := prompts.Load()
	if strings.TrimSpace(templates.System) == "" {
		return Prompt{}, errors.New("system prompt template is empty")
	}
	if strings.TrimSpace(templates.User) == "" {
		return Prompt{}, errors.New("user prompt template is empty")
	}

	values := map[string]string{
		"SOURCE_LANGUAGE": sourceLanguage,
		"TARGET_LANGUAGE": targetLanguage,
		"PDF_FILENAME":    doc.Name,
		"PDF_SIZE_BYTES":  strconv.FormatInt(doc.SizeBytes, 10),
		"PDF_SHA256":      doc.SHA256,
	}

	system := prompts.Render(templates.System, values)
	user := prompts.Render(templates.User, values)

	if unresolved := unresolvedPlaceholder(system); unresolved != "" {
		return Prompt{}, fmt.Errorf("system prompt still contains unresolved placeholder %s", unresolved)
	}
	if unresolved := unresolvedPlaceholder(user); unresolved != "" {
		return Prompt{}, fmt.Errorf("user prompt still contains unresolved placeholder %s", unresolved)
	}
	if strings.TrimSpace(system) == "" {
		return Prompt{}, errors.New("system prompt is empty after rendering")
	}
	if strings.TrimSpace(user) == "" {
		return Prompt{}, errors.New("user prompt is empty after rendering")
	}

	return Prompt{
		System: strings.TrimSpace(system),
		User:   strings.TrimSpace(user),
	}, nil
}

func unresolvedPlaceholder(prompt string) string {
	return unresolvedPlaceholderPattern.FindString(prompt)
}
