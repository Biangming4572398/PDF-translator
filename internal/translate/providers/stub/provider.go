package stub

import (
	"context"
	"fmt"
	"strings"

	"pdftranslator/internal/translate"
)

type Provider struct{}

func New() *Provider {
	return &Provider{}
}

func (p *Provider) Name() string {
	return "stub-preview"
}

func (p *Provider) DisplayName() string {
	return "Starter Stub Provider"
}

func (p *Provider) Translate(_ context.Context, request translate.Request) (translate.Response, error) {
	body := fmt.Sprintf(strings.TrimSpace(`
\section*{Translated Draft}
This starter response came from the built-in stub provider.

\textbf{Requested translation:} %s to %s

\textbf{Source file:} %s

\textbf{About this draft}
\begin{itemize}
\item It proves the raw-PDF-in to XeLaTeX-out workflow end to end.
\item It keeps the provider layer replaceable.
\item It is safe for local validation and XeLaTeX compilation tests.
\end{itemize}
`),
		escapeText(request.SourceLanguage),
		escapeText(request.TargetLanguage),
		escapeText(request.Document.Name),
	)

	return translate.Response{
		Provider:       p.Name(),
		Model:          "starter-stub-v1",
		RawContent:     body,
		LatexCandidate: body,
		Metadata: map[string]string{
			"mode": "stub",
		},
	}, nil
}

func escapeText(input string) string {
	replacer := strings.NewReplacer(
		`\`, `\textbackslash{}`,
		`&`, `\&`,
		`%`, `\%`,
		`$`, `\$`,
		`#`, `\#`,
		`_`, `\_`,
		`{`, `\{`,
		`}`, `\}`,
		`~`, `\textasciitilde{}`,
		`^`, `\textasciicircum{}`,
	)
	return replacer.Replace(input)
}
