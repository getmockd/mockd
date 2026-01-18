package mqtt

// MQTTConfig configures the MQTT broker
type MQTTConfig struct {
	ID          string          `json:"id" yaml:"id"`
	Name        string          `json:"name,omitempty" yaml:"name,omitempty"`
	ParentID    string          `json:"parentId,omitempty" yaml:"parentId,omitempty"`
	MetaSortKey float64         `json:"metaSortKey,omitempty" yaml:"metaSortKey,omitempty"`
	Port        int             `json:"port" yaml:"port"`
	TLS         *MQTTTLSConfig  `json:"tls,omitempty" yaml:"tls,omitempty"`
	Auth        *MQTTAuthConfig `json:"auth,omitempty" yaml:"auth,omitempty"`
	Topics      []TopicConfig   `json:"topics,omitempty" yaml:"topics,omitempty"`
	Enabled     bool            `json:"enabled" yaml:"enabled"`
}

// MQTTTLSConfig configures TLS for the MQTT broker
type MQTTTLSConfig struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	CertFile string `json:"certFile" yaml:"certFile"`
	KeyFile  string `json:"keyFile" yaml:"keyFile"`
}

// MQTTAuthConfig configures authentication for the MQTT broker
type MQTTAuthConfig struct {
	Enabled bool       `json:"enabled" yaml:"enabled"`
	Users   []MQTTUser `json:"users,omitempty" yaml:"users,omitempty"`
}

// MQTTUser represents an authenticated MQTT user
type MQTTUser struct {
	Username string    `json:"username" yaml:"username"`
	Password string    `json:"password" yaml:"password"`
	ACL      []ACLRule `json:"acl,omitempty" yaml:"acl,omitempty"`
}

// ACLRule defines access control for topics
type ACLRule struct {
	Topic  string `json:"topic" yaml:"topic"`   // e.g., "sensors/#"
	Access string `json:"access" yaml:"access"` // "read", "write", "readwrite"
}

// TopicConfig configures a mock topic
type TopicConfig struct {
	Topic            string                    `json:"topic" yaml:"topic"`
	QoS              int                       `json:"qos,omitempty" yaml:"qos,omitempty"`
	Retain           bool                      `json:"retain,omitempty" yaml:"retain,omitempty"`
	Messages         []MessageConfig           `json:"messages,omitempty" yaml:"messages,omitempty"`
	OnPublish        *PublishHandler           `json:"onPublish,omitempty" yaml:"onPublish,omitempty"`
	DeviceSimulation *DeviceSimulationSettings `json:"deviceSimulation,omitempty" yaml:"deviceSimulation,omitempty"`
}

// DeviceSimulationSettings configures per-topic device simulation
type DeviceSimulationSettings struct {
	Enabled         bool   `json:"enabled" yaml:"enabled"`
	DeviceCount     int    `json:"deviceCount" yaml:"deviceCount"`         // 1-1000
	DeviceIDPattern string `json:"deviceIdPattern" yaml:"deviceIdPattern"` // e.g., "sensor-{n}"
}

// MessageConfig configures a message to be published
type MessageConfig struct {
	Payload  string `json:"payload" yaml:"payload"`
	Delay    string `json:"delay,omitempty" yaml:"delay,omitempty"`
	Repeat   bool   `json:"repeat,omitempty" yaml:"repeat,omitempty"`
	Interval string `json:"interval,omitempty" yaml:"interval,omitempty"`
}

// PublishHandler configures behavior when a message is received
type PublishHandler struct {
	Response *MessageConfig `json:"response,omitempty" yaml:"response,omitempty"`
	Forward  string         `json:"forward,omitempty" yaml:"forward,omitempty"` // Forward to another topic
}

// MultiDeviceSimulationConfig configures multi-device MQTT simulation
type MultiDeviceSimulationConfig struct {
	DeviceCount     int    `json:"deviceCount"`               // Number of devices to simulate (1-1000)
	DeviceIDPattern string `json:"deviceIdPattern"`           // Pattern for device IDs, e.g., "device-{id}" or "sensor-{id}"
	TopicPattern    string `json:"topicPattern"`              // Topic pattern with {device_id} placeholder, e.g., "sensors/{device_id}/temp"
	PayloadTemplate string `json:"payloadTemplate,omitempty"` // Payload template with dynamic values
	IntervalMs      int    `json:"intervalMs"`                // Publish interval in milliseconds
	QoS             int    `json:"qos,omitempty"`             // QoS level (0, 1, or 2)
	Retain          bool   `json:"retain,omitempty"`          // Retain flag
}

// DeviceSimulationStatus represents the status of a single simulated device
type DeviceSimulationStatus struct {
	DeviceID     string `json:"deviceId"`
	Connected    bool   `json:"connected"`
	LastPublish  int64  `json:"lastPublish,omitempty"` // Unix timestamp
	MessageCount int64  `json:"messageCount"`
	Topic        string `json:"topic"`
}

// MultiDeviceSimulationStatus represents the overall status of a multi-device simulation
type MultiDeviceSimulationStatus struct {
	Running       bool                         `json:"running"`
	DeviceCount   int                          `json:"deviceCount"`
	ActiveDevices int                          `json:"activeDevices"`
	TotalMessages int64                        `json:"totalMessages"`
	Config        *MultiDeviceSimulationConfig `json:"config,omitempty"`
	Devices       []DeviceSimulationStatus     `json:"devices,omitempty"`
	StartedAt     int64                        `json:"startedAt,omitempty"` // Unix timestamp
}
