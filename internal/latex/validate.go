package latex

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var ErrMalformedEnvironment = errors.New("malformed LaTeX environments detected")

type SanitizedContent struct {
	Content          string
	FullDocument     bool
	Warnings         []string
	RemovedCommands  []string
	EscapedLooseText bool
}

type Validator struct {
	blocked []string
}

func NewValidator() *Validator {
	return &Validator{
		blocked: []string{
			`\write18`,
			`\input`,
			`\include`,
			`\openout`,
			`\openin`,
			`\read`,
			`\write`,
			`\catcode`,
			`\newwrite`,
			`\newread`,
			`\immediate`,
			`\usepackage{shellesc}`,
		},
	}
}

func (v *Validator) Sanitize(input string) (SanitizedContent, error) {
	content := normalizeWhitespace(stripCodeFence(input))
	if content == "" {
		return SanitizedContent{}, errors.New("latex content was empty after normalization")
	}

	warnings := make([]string, 0, 4)
	removed := make([]string, 0, 2)

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		for _, blocked := range v.blocked {
			if strings.Contains(line, blocked) {
				lines[i] = "% stripped unsafe latex command"
				warnings = append(warnings, fmt.Sprintf("removed blocked command from line %d", i+1))
				removed = append(removed, blocked)
				break
			}
		}
	}
	content = strings.Join(lines, "\n")

	fullDocument := strings.Contains(content, `\documentclass`)
	if fullDocument {
		if !strings.Contains(content, `\begin{document}`) || !strings.Contains(content, `\end{document}`) {
			return SanitizedContent{}, errors.New("missing document environment in full-document LaTeX response")
		}
	} else if strings.Contains(content, `\begin{document}`) || strings.Contains(content, `\end{document}`) {
		content = unwrapDocumentEnvironment(content)
		warnings = append(warnings, "response included a document environment without \\documentclass; body was wrapped in the application scaffold")
	}

	escapedLooseText := false
	if shouldEscapeLooseText(content) {
		content = EscapeText(content)
		escapedLooseText = true
		warnings = append(warnings, "response looked like plain text; LaTeX special characters were escaped conservatively")
	}

	if err := validateEnvironmentBalance(content); err != nil {
		return SanitizedContent{}, err
	}

	return SanitizedContent{
		Content:          content,
		FullDocument:     fullDocument,
		Warnings:         warnings,
		RemovedCommands:  removed,
		EscapedLooseText: escapedLooseText,
	}, nil
}

func unwrapDocumentEnvironment(content string) string {
	beginToken := `\begin{document}`
	endToken := `\end{document}`

	beginIndex := strings.Index(content, beginToken)
	if beginIndex >= 0 {
		body := content[beginIndex+len(beginToken):]
		if endIndex := strings.LastIndex(body, endToken); endIndex >= 0 {
			return strings.TrimSpace(body[:endIndex])
		}

		return strings.TrimSpace(strings.ReplaceAll(body, endToken, ""))
	}

	return strings.TrimSpace(strings.ReplaceAll(content, endToken, ""))
}

func validateEnvironmentBalance(content string) error {
	re := regexp.MustCompile(`\\(begin|end)\{([^}]+)\}`)
	matches := re.FindAllStringSubmatch(content, -1)

	stack := make([]string, 0, len(matches))
	for _, match := range matches {
		command := match[1]
		name := match[2]
		if command == "begin" {
			stack = append(stack, name)
			continue
		}

		if len(stack) == 0 || stack[len(stack)-1] != name {
			return fmt.Errorf("%w: unexpected \\end{%s}", ErrMalformedEnvironment, name)
		}
		stack = stack[:len(stack)-1]
	}

	if len(stack) > 0 {
		return fmt.Errorf("%w: unclosed environment %s", ErrMalformedEnvironment, stack[len(stack)-1])
	}

	return nil
}

func shouldEscapeLooseText(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false
	}

	return !strings.Contains(trimmed, `\`) &&
		!strings.Contains(trimmed, "{") &&
		!strings.Contains(trimmed, "}")
}

func normalizeWhitespace(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	content = strings.ReplaceAll(content, "\x00", "")
	return strings.TrimSpace(content)
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
