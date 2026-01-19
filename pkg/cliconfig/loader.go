package cliconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	// LocalConfigFileName is the name of the local config file
	LocalConfigFileName = ".mockdrc.json"
	// GlobalConfigDir is the directory for global config
	GlobalConfigDir = "mockd"
	// GlobalConfigFileName is the name of the global config file
	GlobalConfigFileName = "config.json"
)

// FindLocalConfig searches for .mockdrc.json in the current directory.
func FindLocalConfig() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	path := filepath.Join(cwd, LocalConfigFileName)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	return "", nil
}

// FindGlobalConfig returns the path to the global config file.
// Returns empty string if not found.
func FindGlobalConfig() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", nil // No config dir available
	}
	path := filepath.Join(configDir, GlobalConfigDir, GlobalConfigFileName)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	return "", nil
}

// LoadConfigFile loads a CLIConfig from a JSON file.
func LoadConfigFile(path string) (*CLIConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg CLIConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		// Try to get line/column info from JSON error
		if syntaxErr, ok := err.(*json.SyntaxError); ok {
			line, col := FindLineColumn(data, syntaxErr.Offset)
			return nil, &ConfigError{
				Path:    path,
				Line:    line,
				Column:  col,
				Message: syntaxErr.Error(),
			}
		}
		return nil, &ConfigError{
			Path:    path,
			Message: err.Error(),
		}
	}

	cfg.Sources = make(map[string]string)
	return &cfg, nil
}

// ConfigError represents a configuration file error with location info.
type ConfigError struct {
	Path    string
	Line    int
	Column  int
	Message string
}

func (e *ConfigError) Error() string {
	if e.Line > 0 {
		return e.Path + " (line " + itoa(e.Line) + ", column " + itoa(e.Column) + "): " + e.Message
	}
	return e.Path + ": " + e.Message
}

// FindLineColumn finds the line and column number for a byte offset.
func FindLineColumn(data []byte, offset int64) (line, col int) {
	line = 1
	col = 1
	for i := int64(0); i < offset && int(i) < len(data); i++ {
		if data[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}

// LoadAll loads configuration from all sources and merges them.
// Precedence: flags > env > local config > global config > defaults
func LoadAll() (*CLIConfig, error) {
	// Start with defaults
	cfg := NewDefault()

	// Load global config
	if globalPath, err := FindGlobalConfig(); err == nil && globalPath != "" {
		if globalCfg, err := LoadConfigFile(globalPath); err == nil {
			MergeConfig(cfg, globalCfg, SourceGlobal)
		}
	}

	// Load local config
	if localPath, err := FindLocalConfig(); err == nil && localPath != "" {
		if localCfg, err := LoadConfigFile(localPath); err == nil {
			MergeConfig(cfg, localCfg, SourceLocal)
		}
	}

	// Load environment variables
	LoadEnvConfig(cfg)

	return cfg, nil
}
