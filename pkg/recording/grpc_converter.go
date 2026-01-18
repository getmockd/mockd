// Package recording provides conversion from gRPC recordings to mock configurations.
package recording

import (
	"github.com/getmockd/mockd/pkg/grpc"
)

// GRPCConvertOptions configures how gRPC recordings are converted to configs.
type GRPCConvertOptions struct {
	// IncludeMetadata includes gRPC metadata in match conditions
	IncludeMetadata bool `json:"includeMetadata,omitempty"`

	// Deduplicate removes duplicate request patterns
	Deduplicate bool `json:"deduplicate,omitempty"`

	// IncludeDelay adds recorded latency as delay in config
	IncludeDelay bool `json:"includeDelay,omitempty"`
}

// DefaultGRPCConvertOptions returns default conversion options.
func DefaultGRPCConvertOptions() GRPCConvertOptions {
	return GRPCConvertOptions{
		IncludeMetadata: false,
		Deduplicate:     true,
		IncludeDelay:    false,
	}
}

// ToGRPCMethodConfig converts a single gRPC recording to a MethodConfig.
func ToGRPCMethodConfig(rec *GRPCRecording, opts GRPCConvertOptions) *grpc.MethodConfig {
	if rec == nil {
		return nil
	}

	cfg := &grpc.MethodConfig{}

	// Handle response based on stream type
	switch rec.StreamType {
	case GRPCStreamUnary:
		cfg.Response = rec.Response
	case GRPCStreamServerStream, GRPCStreamBidi:
		// For streaming responses, use Responses array
		if responses, ok := rec.Response.([]interface{}); ok {
			cfg.Responses = responses
		} else if rec.Response != nil {
			cfg.Responses = []interface{}{rec.Response}
		}
	case GRPCStreamClientStream:
		// Client streaming returns single response
		cfg.Response = rec.Response
	}

	// Handle error
	if rec.Error != nil {
		cfg.Error = &grpc.GRPCErrorConfig{
			Code:    rec.Error.Code,
			Message: rec.Error.Message,
		}
	}

	// Include delay if configured
	if opts.IncludeDelay && rec.Duration > 0 {
		cfg.Delay = rec.Duration.String()
	}

	// Include metadata matching if configured
	if opts.IncludeMetadata && len(rec.Metadata) > 0 {
		mdMatch := make(map[string]string)
		for k, v := range rec.Metadata {
			if len(v) > 0 {
				mdMatch[k] = v[0]
			}
		}
		if len(mdMatch) > 0 {
			cfg.Match = &grpc.MethodMatch{
				Metadata: mdMatch,
			}
		}
	}

	return cfg
}

// ToGRPCServiceConfig converts recordings to a ServiceConfig.
func ToGRPCServiceConfig(recordings []*GRPCRecording, opts GRPCConvertOptions) map[string]grpc.ServiceConfig {
	if len(recordings) == 0 {
		return nil
	}

	// Group recordings by service and method
	type key struct {
		service string
		method  string
	}
	groups := make(map[key][]*GRPCRecording)
	order := make([]key, 0)

	for _, rec := range recordings {
		k := key{service: rec.Service, method: rec.Method}
		if _, exists := groups[k]; !exists {
			order = append(order, k)
		}
		groups[k] = append(groups[k], rec)
	}

	// Build service configs
	result := make(map[string]grpc.ServiceConfig)

	for _, k := range order {
		recs := groups[k]
		var selectedRec *GRPCRecording

		if opts.Deduplicate {
			// Use first recording for each method
			selectedRec = recs[0]
		} else {
			// Use last recording
			selectedRec = recs[len(recs)-1]
		}

		// Get or create service config
		svcCfg, ok := result[k.service]
		if !ok {
			svcCfg = grpc.ServiceConfig{
				Methods: make(map[string]grpc.MethodConfig),
			}
		}

		// Add method config
		methodCfg := ToGRPCMethodConfig(selectedRec, opts)
		if methodCfg != nil {
			svcCfg.Methods[k.method] = *methodCfg
		}

		result[k.service] = svcCfg
	}

	return result
}

// ToGRPCConfig converts recordings to a complete GRPCConfig.
func ToGRPCConfig(recordings []*GRPCRecording, opts GRPCConvertOptions) *grpc.GRPCConfig {
	if len(recordings) == 0 {
		return nil
	}

	// Find the proto file from recordings
	var protoFile string
	for _, rec := range recordings {
		if rec.ProtoFile != "" {
			protoFile = rec.ProtoFile
			break
		}
	}

	services := ToGRPCServiceConfig(recordings, opts)

	return &grpc.GRPCConfig{
		ProtoFile:  protoFile,
		Services:   services,
		Reflection: true,
		Enabled:    true,
	}
}

// GRPCConvertResult contains the result of converting gRPC recordings.
type GRPCConvertResult struct {
	Config   *grpc.GRPCConfig `json:"config"`
	Services int              `json:"services"`
	Methods  int              `json:"methods"`
	Total    int              `json:"total"`
	Warnings []string         `json:"warnings,omitempty"`
}

// ConvertGRPCRecordings converts a set of recordings to a GRPCConfig with stats.
func ConvertGRPCRecordings(recordings []*GRPCRecording, opts GRPCConvertOptions) *GRPCConvertResult {
	result := &GRPCConvertResult{
		Total:    len(recordings),
		Warnings: make([]string, 0),
	}

	if len(recordings) == 0 {
		return result
	}

	result.Config = ToGRPCConfig(recordings, opts)

	// Count services and methods
	if result.Config != nil {
		result.Services = len(result.Config.Services)
		for _, svc := range result.Config.Services {
			result.Methods += len(svc.Methods)
		}
	}

	// Add warnings for streaming calls (limited support)
	for _, rec := range recordings {
		if rec.StreamType != GRPCStreamUnary {
			result.Warnings = append(result.Warnings,
				"Streaming call "+rec.Service+"/"+rec.Method+" converted with limited support")
			break
		}
	}

	return result
}

// MergeGRPCConfigs merges recordings into an existing GRPCConfig.
func MergeGRPCConfigs(base *grpc.GRPCConfig, recordings []*GRPCRecording, opts GRPCConvertOptions) *grpc.GRPCConfig {
	if base == nil {
		return ToGRPCConfig(recordings, opts)
	}

	newServices := ToGRPCServiceConfig(recordings, opts)
	if newServices == nil {
		return base
	}

	// Initialize services map if nil
	if base.Services == nil {
		base.Services = make(map[string]grpc.ServiceConfig)
	}

	// Merge new services into base
	for svcName, svcCfg := range newServices {
		if existing, ok := base.Services[svcName]; ok {
			// Merge methods
			if existing.Methods == nil {
				existing.Methods = make(map[string]grpc.MethodConfig)
			}
			for methodName, methodCfg := range svcCfg.Methods {
				existing.Methods[methodName] = methodCfg
			}
			base.Services[svcName] = existing
		} else {
			base.Services[svcName] = svcCfg
		}
	}

	return base
}
