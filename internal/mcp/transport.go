package mcp

import (
	"fmt"

	"github.com/yeisme/taskbridge-mcp/pkg/logger"
)

// TransportConfig holds configuration for a specific transport type.
type TransportConfig struct {
	Type    TransportType
	Address string
	Port    int
	Timeout int // in seconds
	Options map[string]any
}

// TransportManager manages multiple MCP transports.
type TransportManager struct {
	transports map[TransportType]TransportConfig
}

// NewTransportManager creates a new transport manager.
func NewTransportManager() *TransportManager {
	return &TransportManager{
		transports: make(map[TransportType]TransportConfig),
	}
}

// RegisterTransport registers a transport configuration.
func (tm *TransportManager) RegisterTransport(config TransportConfig) error {
	if config.Type == "" {
		return fmt.Errorf("transport type cannot be empty")
	}

	tm.transports[config.Type] = config
	logger.Debugf("Registered transport: %s", config.Type)

	return nil
}

// GetTransport retrieves a transport configuration.
func (tm *TransportManager) GetTransport(t TransportType) (TransportConfig, error) {
	config, exists := tm.transports[t]
	if !exists {
		return TransportConfig{}, fmt.Errorf("transport not found: %s", t)
	}

	return config, nil
}

// ListTransports returns all registered transport types.
func (tm *TransportManager) ListTransports() []TransportType {
	types := make([]TransportType, 0, len(tm.transports))
	for t := range tm.transports {
		types = append(types, t)
	}

	return types
}

// ValidateTransportType validates if a transport type is supported.
func ValidateTransportType(t string) error {
	switch TransportType(t) {
	case TransportStdio, TransportSSE, TransportHTTP:
		return nil
	default:
		return fmt.Errorf("unsupported transport type: %s (supported: stdio, sse, http)", t)
	}
}

// GetTransportDescription returns a description of a transport type.
func GetTransportDescription(t TransportType) string {
	descriptions := map[TransportType]string{
		TransportStdio: "Standard input/output - Direct CLI communication",
		TransportSSE:   "Server-Sent Events - Alternative HTTP streaming transport",
		TransportHTTP:  "Streaming HTTP (RECOMMENDED) - Best performance, MCP community standard",
	}

	if desc, exists := descriptions[t]; exists {
		return desc
	}

	return "Unknown transport type"
}

// TransportFeatures describes capabilities of each transport.
type TransportFeatures struct {
	Name         string
	Description  string
	IsStreaming  bool
	RequiresHTTP bool
	DefaultPort  int
	Capabilities []string
}

// GetTransportFeatures returns features information for a transport type.
func GetTransportFeatures(t TransportType) TransportFeatures {
	features := map[TransportType]TransportFeatures{
		TransportStdio: {
			Name:         "Standard Input/Output",
			Description:  "Direct CLI communication using stdin/stdout",
			IsStreaming:  false,
			RequiresHTTP: false,
			Capabilities: []string{"tools", "resources", "prompts", "sampling"},
		},
		TransportSSE: {
			Name:         "Server-Sent Events",
			Description:  "Alternative HTTP-based streaming communication",
			IsStreaming:  true,
			RequiresHTTP: true,
			DefaultPort:  8080,
			Capabilities: []string{"tools", "resources", "prompts", "sampling"},
		},
		TransportHTTP: {
			Name:         "Streaming HTTP (RECOMMENDED) ⭐",
			Description:  "Best performance streaming HTTP transport - MCP community standard",
			IsStreaming:  true,
			RequiresHTTP: true,
			DefaultPort:  8080,
			Capabilities: []string{"tools", "resources", "prompts", "sampling", "streaming"},
		},
	}

	if f, exists := features[t]; exists {
		return f
	}

	return TransportFeatures{
		Name:         "Unknown",
		Description:  "Unknown transport type",
		Capabilities: []string{},
	}
}
