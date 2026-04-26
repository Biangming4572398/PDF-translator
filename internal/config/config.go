package config

import "strings"

const (
	AppName                  = "PDF Translator"
	AppID                    = "com.openai.pdftranslator"
	DefaultOpenRouterBaseURL = "https://openrouter.ai/api/v1"
	DefaultOpenRouterModel   = "deepseek/deepseek-chat-v3.1"
	DefaultProviderTimeout   = 420
	LanguageChinese          = "Chinese"
	LanguageEnglish          = "English"
)

type Settings struct {
	CurrentProvider       string                    `json:"current_provider"`
	DefaultSourceLanguage string                    `json:"default_source_language"`
	DefaultTargetLanguage string                    `json:"default_target_language"`
	Providers             map[string]ProviderConfig `json:"providers"`
	Compiler              CompilerSettings          `json:"compiler"`
	PDF                   PDFSettings               `json:"pdf"`
}

type ProviderConfig struct {
	APIKey         string `json:"api_key"`
	BaseURL        string `json:"base_url"`
	Model          string `json:"model"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type CompilerSettings struct {
	XeLaTeXPath         string `json:"xelatex_path"`
	TeXRoot             string `json:"tex_root"`
	Passes              int    `json:"passes"`
	MainFont            string `json:"main_font"`
	TimeoutSeconds      int    `json:"timeout_seconds"`
	AutoInstallPackages bool   `json:"auto_install_packages"`
}

type PDFSettings struct {
	MaxFileSizeMB int `json:"max_file_size_mb"`
}

func DefaultSettings() Settings {
	return Settings{
		CurrentProvider:       "openai-compatible",
		DefaultSourceLanguage: LanguageChinese,
		DefaultTargetLanguage: LanguageEnglish,
		Providers: map[string]ProviderConfig{
			"stub-preview": {
				Model:          "starter-stub-v1",
				TimeoutSeconds: 30,
			},
			"openai-compatible": {
				BaseURL:        DefaultOpenRouterBaseURL,
				Model:          DefaultOpenRouterModel,
				TimeoutSeconds: DefaultProviderTimeout,
			},
		},
		Compiler: CompilerSettings{
			Passes:              2,
			MainFont:            "TeX Gyre Termes",
			TimeoutSeconds:      120,
			AutoInstallPackages: true,
		},
		PDF: PDFSettings{
			MaxFileSizeMB: 20,
		},
	}
}

func SupportedTranslationLanguages() []string {
	return []string{LanguageChinese, LanguageEnglish}
}

func IsSupportedTranslationLanguage(language string) bool {
	switch strings.TrimSpace(language) {
	case LanguageChinese, LanguageEnglish:
		return true
	default:
		return false
	}
}

func NormalizeTranslationLanguage(language, fallback string) string {
	if IsSupportedTranslationLanguage(language) {
		return strings.TrimSpace(language)
	}
	if IsSupportedTranslationLanguage(fallback) {
		return strings.TrimSpace(fallback)
	}
	return LanguageChinese
}

func IsSupportedTranslationPair(sourceLanguage, targetLanguage string) bool {
	sourceLanguage = strings.TrimSpace(sourceLanguage)
	targetLanguage = strings.TrimSpace(targetLanguage)
	return IsSupportedTranslationLanguage(sourceLanguage) &&
		IsSupportedTranslationLanguage(targetLanguage) &&
		sourceLanguage != targetLanguage
}

func (s Settings) Provider(name string) ProviderConfig {
	if cfg, ok := s.Providers[name]; ok {
		return cfg
	}

	return ProviderConfig{}
}

func NormalizeSettings(settings Settings) Settings {
	defaults := DefaultSettings()

	if settings.CurrentProvider == "" || settings.CurrentProvider == "stub-preview" {
		settings.CurrentProvider = defaults.CurrentProvider
	}
	if settings.DefaultSourceLanguage == "" {
		settings.DefaultSourceLanguage = defaults.DefaultSourceLanguage
	}
	settings.DefaultSourceLanguage = NormalizeTranslationLanguage(settings.DefaultSourceLanguage, defaults.DefaultSourceLanguage)
	if settings.DefaultTargetLanguage == "" {
		settings.DefaultTargetLanguage = defaults.DefaultTargetLanguage
	}
	settings.DefaultTargetLanguage = NormalizeTranslationLanguage(settings.DefaultTargetLanguage, defaults.DefaultTargetLanguage)
	if settings.DefaultSourceLanguage == settings.DefaultTargetLanguage {
		if settings.DefaultSourceLanguage == LanguageChinese {
			settings.DefaultTargetLanguage = LanguageEnglish
		} else {
			settings.DefaultTargetLanguage = LanguageChinese
		}
	}
	if settings.Providers == nil {
		settings.Providers = map[string]ProviderConfig{}
	}

	for name, defaultProvider := range defaults.Providers {
		cfg, ok := settings.Providers[name]
		if !ok {
			settings.Providers[name] = defaultProvider
			continue
		}

		if cfg.TimeoutSeconds <= 0 {
			cfg.TimeoutSeconds = defaultProvider.TimeoutSeconds
		}

		if name == "openai-compatible" {
			if cfg.BaseURL == "" || cfg.BaseURL == "https://api.openai.com/v1" {
				cfg.BaseURL = DefaultOpenRouterBaseURL
			}
			if cfg.Model == "" || cfg.Model == "gpt-4.1" {
				cfg.Model = DefaultOpenRouterModel
			}
			if cfg.TimeoutSeconds < DefaultProviderTimeout {
				cfg.TimeoutSeconds = DefaultProviderTimeout
			}
		}

		settings.Providers[name] = cfg
	}

	if settings.Compiler.Passes <= 0 {
		settings.Compiler.Passes = defaults.Compiler.Passes
	}
	if settings.Compiler.MainFont == "" {
		settings.Compiler.MainFont = defaults.Compiler.MainFont
	}
	if settings.Compiler.TimeoutSeconds <= 0 {
		settings.Compiler.TimeoutSeconds = defaults.Compiler.TimeoutSeconds
	}
	if settings.PDF.MaxFileSizeMB <= 0 {
		settings.PDF.MaxFileSizeMB = defaults.PDF.MaxFileSizeMB
	}

	return settings
}
