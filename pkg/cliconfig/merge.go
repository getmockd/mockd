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
	// For booleans, we use the zero value check differently
	// We'll only merge if the source struct has explicit settings
	// This is a limitation - in practice booleans from JSON will be set
	if source.AutoCert {
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
	if source.Verbose {
		target.Verbose = source.Verbose
		target.Sources["verbose"] = sourceType
	}
	if source.JSON {
		target.JSON = source.JSON
		target.Sources["json"] = sourceType
	}
}
