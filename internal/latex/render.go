package latex

import "fmt"

type Options struct {
	Title          string
	MainFont       string
	SourceLanguage string
	TargetLanguage string
}

type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

func (r *Renderer) Render(content SanitizedContent, options Options) string {
	if content.FullDocument {
		return content.Content
	}

	mainFont := options.MainFont
	if mainFont == "" {
		mainFont = "TeX Gyre Termes"
	}

	title := options.Title
	if title == "" {
		title = "Translated PDF"
	}

	return fmt.Sprintf(`\documentclass[12pt]{article}
\usepackage[a4paper,margin=1in]{geometry}
\usepackage{fontspec}
\usepackage{setspace}
\usepackage{longtable}
\usepackage{booktabs}
\usepackage{array}
\usepackage{graphicx}
\usepackage{hyperref}
\usepackage{xcolor}
\setmainfont{%s}
\setstretch{1.15}
\setlength{\parindent}{0pt}
\setlength{\parskip}{0.7em}
\hypersetup{colorlinks=true, linkcolor=blue, urlcolor=blue}
\begin{document}
\section*{%s}
\textbf{Translation:} %s to %s

%s
\end{document}
`,
		mainFont,
		EscapeText(title),
		EscapeText(options.SourceLanguage),
		EscapeText(options.TargetLanguage),
		content.Content,
	)
}
