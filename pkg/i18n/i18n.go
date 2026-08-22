// Package i18n provides lightweight localization support for the mockd CLI.
//
// Language resolution order:
//  1. --lang flag (set via SetLangFromFlag during Execute)
//  2. MOCKD_LANG environment variable
//  3. language field in config file (via SetLang)
//  4. Default: en
//
// Translation files are embedded into the binary via go:embed, keeping the
// single-binary deployment model. Missing keys gracefully fall back to the
// English source string, so partial translations never break the CLI.
package i18n

import (
	"embed"
	"encoding/json"
	"os"
	"strings"
	"sync"
)

//go:embed locales/*.json
var localeFS embed.FS

// SupportedLanguages lists language codes with available locale files.
var SupportedLanguages = []string{"en", "zh-CN"}

var (
	mu       sync.RWMutex
	current  = "en"
	fallback = "en"
	dicts    map[string]map[string]string
)

// Lang returns the currently active language code.
func Lang() string {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// SetLang switches the active language. Unsupported codes are ignored.
// Returns the effective language code.
func SetLang(lang string) string {
	lang = Normalize(lang)
	mu.Lock()
	defer mu.Unlock()
	if dicts == nil {
		loadLocked()
	}
	if _, ok := dicts[lang]; ok {
		current = lang
	}
	return current
}

// Normalize canonicalizes a language code (e.g. "zh_CN" -> "zh-CN").
func Normalize(lang string) string {
	if lang == "" {
		return fallback
	}
	lang = strings.ToLower(strings.TrimSpace(lang))
	lang = strings.ReplaceAll(lang, "_", "-")
	// zh-cn -> zh-CN
	if strings.HasPrefix(lang, "zh") {
		if lang == "zh" || lang == "zh-cn" || lang == "zh-hans" {
			return "zh-CN"
		}
		if lang == "zh-tw" || lang == "zh-hant" {
			return "zh-TW"
		}
	}
	// en-us -> en
	if strings.HasPrefix(lang, "en") {
		return "en"
	}
	return lang
}

// T returns the localized text for msg in the current language.
// If no translation exists, msg itself is returned.
func T(msg string) string {
	if msg == "" {
		return ""
	}
	mu.RLock()
	if dicts == nil {
		mu.RUnlock()
		mu.Lock()
		if dicts == nil {
			loadLocked()
		}
		mu.Unlock()
		mu.RLock()
	}
	if d, ok := dicts[current]; ok {
		if s, ok := d[msg]; ok && s != "" {
			mu.RUnlock()
			return s
		}
	}
	// Fall back to source string (already the English text in most cases).
	mu.RUnlock()
	return msg
}

// Tf returns the localized text for msg formatted with fmt.Sprintf-style args.
func Tf(msg string, args ...interface{}) string {
	return sprintf(T(msg), args...)
}

// DetectFromEnv reads MOCKD_LANG and applies it. Returns effective lang.
func DetectFromEnv() string {
	return SetLang(os.Getenv("MOCKD_LANG"))
}

// loadLocked loads all locale files into dicts. Caller must hold mu.
func loadLocked() {
	dicts = make(map[string]map[string]string)
	entries, err := localeFS.ReadDir("locales")
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		code := strings.TrimSuffix(e.Name(), ".json")
		data, err := localeFS.ReadFile("locales/" + e.Name())
		if err != nil {
			continue
		}
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		dicts[code] = m
	}
}

// UsageTemplate returns the cobra usage template localized for the current language.
func UsageTemplate() string {
	if current == "zh-CN" {
		return `用法:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

别名:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

示例:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

可用命令:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

选项:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

全局选项:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

其他帮助主题:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

使用 "{{.CommandPath}} [command] --help" 查看某条命令的更多信息。{{end}}`
	}
	return `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}`
}
