# Prompt Templates

Edit these Markdown files to control the LLM instructions:

- `system.md`: system/developer-style instruction sent as the system message.
- `user.md`: user instruction sent with the attached raw PDF.

Supported placeholders:

- `{{SOURCE_LANGUAGE}}`
- `{{TARGET_LANGUAGE}}`
- `{{PDF_FILENAME}}`
- `{{PDF_SIZE_BYTES}}`
- `{{PDF_SHA256}}`

The defaults are embedded into the executable. At runtime, the app checks for
external overrides in this order:

1. The `PDF_TRANSLATOR_PROMPTS_DIR` environment variable.
2. A `prompts` folder beside `pdftranslator.exe`.
3. A `prompts` folder in the current working directory.
