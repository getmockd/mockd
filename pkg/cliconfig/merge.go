package cliconfig

// MergeConfig merges source config into target, updating sources tracking.
// Only non-zero values from source are applied.
func MergeConfig(target, source *CLIConfig, sourceType string) {
	if source == nil {
		return
	}
	if target.Sources == nil {
		target.Sources = make(map[string]string)
	}

	if source.Port != 0 {
		target.Port = source.Port
		target.Sources["port"] = sourceType
	}
	if source.AdminPort != 0 {
		target.AdminPort = source.AdminPort
		target.Sources["adminPort"] = sourceType
	}
	if source.HTTPSPort != 0 {
		target.HTTPSPort = source.HTTPSPort
		target.Sources["httpsPort"] = sourceType
	}
	if source.ConfigFile != "" {
		target.ConfigFile = source.ConfigFile
		target.Sources["configFile"] = sourceType
	}
	if source.ReadTimeout != 0 {
		target.ReadTimeout = source.ReadTimeout
		target.Sources["readTimeout"] = sourceType
	}
	if source.WriteTimeout != 0 {
		target.WriteTimeout = source.WriteTimeout
		target.Sources["writeTimeout"] = sourceType
	}
	if source.AdminURL != "" {
		target.AdminURL = source.AdminURL
		target.Sources["adminUrl"] = sourceType
	}
	if source.MaxLogEntries != 0 {
		target.MaxLogEntries = source.MaxLogEntries
		target.Sources["maxLogEntries"] = sourceType
	}
	// For booleans, checking `if source.X` cannot detect an explicit false.
	// We use SetFields (populated during file loading) to know whether a
	// boolean was explicitly present in the source. If SetFields is nil
	// (e.g., config built programmatically), fall back to the old behavior
	// of only merging true values.
	if boolIsSet(source, "autoCert") {
		target.AutoCert = source.AutoCert
		target.Sources["autoCert"] = sourceType
	}
	if source.CertFile != "" {
		target.CertFile = source.CertFile
		target.Sources["certFile"] = sourceType
	}
	if source.KeyFile != "" {
		target.KeyFile = source.KeyFile
		target.Sources["keyFile"] = sourceType
	}
	if boolIsSet(source, "verbose") {
		target.Verbose = source.Verbose
		target.Sources["verbose"] = sourceType
	}
	if boolIsSet(source, "json") {
		target.JSON = source.JSON
		target.Sources["json"] = sourceType
	}
}

// boolIsSet reports whether a boolean field identified by its YAML key was
// explicitly set in the source config. When SetFields is available (file-loaded
// configs), it checks for the key's presence. Otherwise it falls back to
// treating true as "set" (the old behavior, safe for programmatic configs).
func boolIsSet(cfg *CLIConfig, yamlKey string) bool {
	if cfg.SetFields != nil {
		return cfg.SetFields[yamlKey]
	}
	// Fallback: only merge when the value is true (cannot detect explicit false
	// without SetFields).
	switch yamlKey {
	case "autoCert":
		return cfg.AutoCert
	case "verbose":
		return cfg.Verbose
	case "json":
		return cfg.JSON
	}
	return false
}
