package cliconfig

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	// GlobalConfigDir is the directory for global config
	GlobalConfigDir = "mockd"
)

// LocalConfigFileNames are the names to search for local config (in order).
var LocalConfigFileNames = []string{".mockdrc.yaml", ".mockdrc.yml"}

// GlobalConfigFileNames are the names to search for global config (in order).
var GlobalConfigFileNames = []string{"config.yaml", "config.yml"}

// FindLocalConfig searches for .mockdrc.yaml or .mockdrc.yml in the current directory.
func FindLocalConfig() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for _, name := range LocalConfigFileNames {
		path := filepath.Join(cwd, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", nil
}

// GetLocalConfigSearchPaths returns the paths that will be searched for local config.
func GetLocalConfigSearchPaths() []string {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}
	paths := make([]string, len(LocalConfigFileNames))
	for i, name := range LocalConfigFileNames {
		paths[i] = filepath.Join(cwd, name)
	}
	return paths
}

// FindGlobalConfig returns the path to the global config file.
// Returns empty string if not found.
func FindGlobalConfig() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		//nolint:nilerr // intentionally returning empty string when no config dir is available
		return "", nil
	}
	for _, name := range GlobalConfigFileNames {
		path := filepath.Join(configDir, GlobalConfigDir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", nil
}

// GetGlobalConfigSearchPaths returns the paths that will be searched for global config.
func GetGlobalConfigSearchPaths() []string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil
	}
	paths := make([]string, len(GlobalConfigFileNames))
	for i, name := range GlobalConfigFileNames {
		paths[i] = filepath.Join(configDir, GlobalConfigDir, name)
	}
	return paths
}

// LoadConfigFile loads a CLIConfig from a YAML file.
func LoadConfigFile(path string) (*CLIConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg CLIConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
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
