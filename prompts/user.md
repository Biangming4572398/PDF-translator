Translate the attached PDF from {{SOURCE_LANGUAGE}} to {{TARGET_LANGUAGE}}.

Use the attached PDF file as the source of truth. The application is intentionally using a raw-PDF workflow, so do not ask for extracted text or a reconstructed document model.
Only Chinese to English and English to Chinese translation directions are supported in this version.

PDF metadata:
- filename: {{PDF_FILENAME}}
- size_bytes: {{PDF_SIZE_BYTES}}
- sha256: {{PDF_SHA256}}

Output requirements:
- Return only XeLaTeX-compatible content.
- Keep Unicode text intact for XeLaTeX.
- Use simple LaTeX commands that are likely to compile.
- Do not use \input, \include, \write18, shell escape, or external file commands.
- Preserve visible reading order, headings, lists, tables, and emphasis as best as the attached PDF allows.
