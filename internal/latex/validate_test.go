package latex

import (
	"strings"
	"testing"
)

func TestSanitizeUnwrapsPartialDocumentEnvironment(t *testing.T) {
	validator := NewValidator()
	input := `\begin{document}
\section*{Translated Report}
Hello.
\end{document}`

	sanitized, err := validator.Sanitize(input)
	if err != nil {
		t.Fatalf("Sanitize returned error: %v", err)
	}
	if sanitized.FullDocument {
		t.Fatal("partial document environment should be wrapped by the app scaffold")
	}
	if strings.Contains(sanitized.Content, `\begin{document}`) || strings.Contains(sanitized.Content, `\end{document}`) {
		t.Fatalf("document environment was not unwrapped: %s", sanitized.Content)
	}
	if !strings.Contains(sanitized.Content, `\section*{Translated Report}`) {
		t.Fatalf("body content was not preserved: %s", sanitized.Content)
	}
	if len(sanitized.Warnings) == 0 {
		t.Fatal("expected warning about partial document environment")
	}
}

func TestRenderWrapsUnwrappedPartialDocument(t *testing.T) {
	validator := NewValidator()
	sanitized, err := validator.Sanitize(`\begin{document}Hello.\end{document}`)
	if err != nil {
		t.Fatalf("Sanitize returned error: %v", err)
	}

	rendered := NewRenderer().Render(sanitized, Options{
		Title:          "Example",
		SourceLanguage: "Chinese",
		TargetLanguage: "English",
	})
	if !strings.Contains(rendered, `\documentclass`) {
		t.Fatalf("rendered output did not include scaffold documentclass: %s", rendered)
	}
	if !strings.Contains(rendered, "Hello.") {
		t.Fatalf("rendered output did not preserve body: %s", rendered)
	}
}
