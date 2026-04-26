package translate

import (
	"strings"
	"testing"

	"pdftranslator/internal/config"
	"pdftranslator/internal/pdf"
)

func TestPromptBuilderBuildsChineseEnglishPrompt(t *testing.T) {
	builder := NewPromptBuilder()
	doc := pdf.SourceDocument{
		Name:      "sample.pdf",
		SizeBytes: 1234,
		SHA256:    "abc123",
		RawBytes:  []byte("%PDF"),
	}

	prompt, err := builder.Build(doc, config.LanguageChinese, config.LanguageEnglish)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if prompt.System == "" {
		t.Fatal("system prompt was empty")
	}
	if prompt.User == "" {
		t.Fatal("user prompt was empty")
	}
	if strings.Contains(prompt.User, "{{") || strings.Contains(prompt.System, "{{") {
		t.Fatalf("prompt contains unresolved placeholder: %#v", prompt)
	}
	if !strings.Contains(prompt.User, "sample.pdf") {
		t.Fatalf("user prompt did not include PDF metadata: %s", prompt.User)
	}
}

func TestPromptBuilderRejectsUnsupportedLanguagePair(t *testing.T) {
	builder := NewPromptBuilder()
	doc := pdf.SourceDocument{Name: "sample.pdf", RawBytes: []byte("%PDF")}

	if _, err := builder.Build(doc, "Auto Detect", config.LanguageEnglish); err == nil {
		t.Fatal("expected unsupported language pair to fail")
	}
	if _, err := builder.Build(doc, config.LanguageEnglish, config.LanguageEnglish); err == nil {
		t.Fatal("expected same-language pair to fail")
	}
}
