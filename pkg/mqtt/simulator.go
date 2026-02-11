package mqtt

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Simulator generates mock IoT data by publishing messages to configured topics
type Simulator struct {
	broker              *Broker
	topics              []TopicConfig
	sequences           *SequenceStore
	done                chan struct{}
	wg                  sync.WaitGroup
	perTopicDeviceSims  map[string]*PerTopicDeviceSimulator // key: topic pattern
	perTopicDeviceSimMu sync.RWMutex
}

// PerTopicDeviceSimulator handles device simulation for a single topic
type PerTopicDeviceSimulator struct {
	broker       *Broker
	topic        TopicConfig
	settings     *DeviceSimulationSettings
	sequences    *SequenceStore
	devices      []*perTopicSimulatedDevice
	done         chan struct{}
	wg           sync.WaitGroup
	mu           sync.RWMutex
	running      bool
	startedAt    time.Time
	topicPattern string // original topic pattern containing {device_id}
}

// perTopicSimulatedDevice represents a single simulated device for per-topic simulation
type perTopicSimulatedDevice struct {
	deviceID     string
	topic        string
	connected    bool
	lastPublish  time.Time
	messageCount int64
	mu           sync.RWMutex
}

// PerTopicDeviceSimulationStatus represents the status of per-topic device simulation
type PerTopicDeviceSimulationStatus struct {
	TopicPattern  string                   `json:"topicPattern"`
	Running       bool                     `json:"running"`
	DeviceCount   int                      `json:"deviceCount"`
	ActiveDevices int                      `json:"activeDevices"`
	TotalMessages int64                    `json:"totalMessages"`
	StartedAt     int64                    `json:"startedAt,omitempty"`
	Devices       []DeviceSimulationStatus `json:"devices,omitempty"`
}

// NewSimulator creates a new IoT device simulator
func NewSimulator(broker *Broker, topics []TopicConfig, sequences *SequenceStore) *Simulator {
	if sequences == nil {
		sequences = NewSequenceStore()
	}
	return &Simulator{
		broker:             broker,
		topics:             topics,
		sequences:          sequences,
		done:               make(chan struct{}),
		perTopicDeviceSims: make(map[string]*PerTopicDeviceSimulator),
	}
}

// Start begins simulating messages for all configured topics
func (s *Simulator) Start() {
	for _, topic := range s.topics {
		// Check if this topic has device simulation enabled
		if topic.DeviceSimulation != nil && topic.DeviceSimulation.Enabled {
			if strings.Contains(topic.Topic, "{device_id}") {
				// Start per-topic device simulation
				s.startPerTopicDeviceSimulation(topic)
			} else {
				log.Printf("MQTT simulator: WARNING: deviceSimulation enabled for topic %q but topic does not contain {device_id} placeholder; skipping device simulation", topic.Topic)
				// Fall through to regular message simulation
				for _, msg := range topic.Messages {
					s.wg.Add(1)
					go s.runMessageLoop(topic, msg)
				}
			}
		} else {
			// Regular message simulation
			for _, msg := range topic.Messages {
				s.wg.Add(1)
				go s.runMessageLoop(topic, msg)
			}
		}
	}
}

// startPerTopicDeviceSimulation starts device simulation for a single topic
func (s *Simulator) startPerTopicDeviceSimulation(topic TopicConfig) {
	sim := NewPerTopicDeviceSimulator(s.broker, topic, s.sequences)
	if err := sim.Start(); err != nil {
		log.Printf("MQTT simulator: failed to start device simulation for topic %s: %v", topic.Topic, err)
		return
	}

	s.perTopicDeviceSimMu.Lock()
	s.perTopicDeviceSims[topic.Topic] = sim
	s.perTopicDeviceSimMu.Unlock()
}

// Stop stops all message simulation
func (s *Simulator) Stop() {
	close(s.done)
	s.wg.Wait()

	// Stop all per-topic device simulations
	s.perTopicDeviceSimMu.Lock()
	for _, sim := range s.perTopicDeviceSims {
		sim.Stop()
	}
	s.perTopicDeviceSims = make(map[string]*PerTopicDeviceSimulator)
	s.perTopicDeviceSimMu.Unlock()
}

// GetPerTopicDeviceSimulationStatus returns the status of per-topic device simulations
func (s *Simulator) GetPerTopicDeviceSimulationStatus() map[string]*PerTopicDeviceSimulationStatus {
	s.perTopicDeviceSimMu.RLock()
	defer s.perTopicDeviceSimMu.RUnlock()

	statuses := make(map[string]*PerTopicDeviceSimulationStatus)
	for topicPattern, sim := range s.perTopicDeviceSims {
		statuses[topicPattern] = sim.GetStatus()
	}
	return statuses
}

// NewPerTopicDeviceSimulator creates a new per-topic device simulator
func NewPerTopicDeviceSimulator(broker *Broker, topic TopicConfig, sequences *SequenceStore) *PerTopicDeviceSimulator {
	if sequences == nil {
		sequences = NewSequenceStore()
	}
	return &PerTopicDeviceSimulator{
		broker:       broker,
		topic:        topic,
		settings:     topic.DeviceSimulation,
		sequences:    sequences,
		topicPattern: topic.Topic,
		done:         make(chan struct{}),
	}
}

// ValidateDeviceSimulationSettings validates per-topic device simulation settings
func ValidateDeviceSimulationSettings(settings *DeviceSimulationSettings, topicPattern string) error {
	if settings == nil {
		return errors.New("settings cannot be nil")
	}
	if !settings.Enabled {
		return nil // Not enabled, nothing to validate
	}
	if settings.DeviceCount < 1 || settings.DeviceCount > 1000 {
		return fmt.Errorf("deviceCount must be between 1 and 1000, got %d", settings.DeviceCount)
	}
	if settings.DeviceIDPattern == "" {
		return errors.New("deviceIdPattern is required")
	}
	if !strings.Contains(settings.DeviceIDPattern, "{n}") && !strings.Contains(settings.DeviceIDPattern, "{id}") && !strings.Contains(settings.DeviceIDPattern, "{index}") {
		return errors.New("deviceIdPattern must contain {n}, {id}, or {index} placeholder")
	}
	if !strings.Contains(topicPattern, "{device_id}") {
		return errors.New("topic pattern must contain {device_id} placeholder for device simulation")
	}
	return nil
}

// Start begins the per-topic device simulation
func (p *PerTopicDeviceSimulator) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return errors.New("simulation is already running")
	}

	if err := ValidateDeviceSimulationSettings(p.settings, p.topicPattern); err != nil {
		return err
	}

	// Create devices
	p.devices = make([]*perTopicSimulatedDevice, p.settings.DeviceCount)
	for i := 0; i < p.settings.DeviceCount; i++ {
		deviceID := p.generateDeviceID(i + 1)
		topic := p.generateTopic(deviceID)
		p.devices[i] = &perTopicSimulatedDevice{
			deviceID:  deviceID,
			topic:     topic,
			connected: true,
		}
	}

	p.running = true
	p.startedAt = time.Now()
	p.done = make(chan struct{})

	// Start a goroutine for each device
	for _, device := range p.devices {
		p.wg.Add(1)
		go p.runDevice(device)
	}

	return nil
}

// Stop stops the per-topic device simulation
func (p *PerTopicDeviceSimulator) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	close(p.done)
	p.mu.Unlock()

	p.wg.Wait()

	// Mark all devices as disconnected
	p.mu.Lock()
	for _, device := range p.devices {
		device.mu.Lock()
		device.connected = false
		device.mu.Unlock()
	}
	p.mu.Unlock()
}

// IsRunning returns whether the simulation is running
func (p *PerTopicDeviceSimulator) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// GetStatus returns the current status of the per-topic device simulation
func (p *PerTopicDeviceSimulator) GetStatus() *PerTopicDeviceSimulationStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	status := &PerTopicDeviceSimulationStatus{
		TopicPattern: p.topicPattern,
		Running:      p.running,
		DeviceCount:  len(p.devices),
	}

	if p.running {
		status.StartedAt = p.startedAt.Unix()
	}

	var totalMessages int64
	activeDevices := 0
	deviceStatuses := make([]DeviceSimulationStatus, 0, len(p.devices))

	for _, device := range p.devices {
		device.mu.RLock()
		ds := DeviceSimulationStatus{
			DeviceID:     device.deviceID,
			Connected:    device.connected,
			MessageCount: device.messageCount,
			Topic:        device.topic,
		}
		if !device.lastPublish.IsZero() {
			ds.LastPublish = device.lastPublish.Unix()
		}
		device.mu.RUnlock()

		totalMessages += ds.MessageCount
		if ds.Connected {
			activeDevices++
		}
		deviceStatuses = append(deviceStatuses, ds)
	}

	status.TotalMessages = totalMessages
	status.ActiveDevices = activeDevices
	status.Devices = deviceStatuses

	return status
}

// runDevice runs the simulation loop for a single device
func (p *PerTopicDeviceSimulator) runDevice(device *perTopicSimulatedDevice) {
	defer p.wg.Done()

	// Get interval from the first message config, default to 5s
	interval := 5 * time.Second
	if len(p.topic.Messages) > 0 && p.topic.Messages[0].Interval != "" {
		if parsed, err := time.ParseDuration(p.topic.Messages[0].Interval); err == nil {
			interval = parsed
		}
	}

	// Handle initial delay
	if len(p.topic.Messages) > 0 && p.topic.Messages[0].Delay != "" {
		if delay, err := time.ParseDuration(p.topic.Messages[0].Delay); err == nil {
			select {
			case <-time.After(delay):
			case <-p.done:
				return
			}
		}
	}

	qos := byte(p.topic.QoS)
	if qos > 2 {
		qos = 0
	}

	// Publish initial message
	p.publishForDevice(device, qos)

	// Check if repeating is enabled
	shouldRepeat := len(p.topic.Messages) > 0 && p.topic.Messages[0].Repeat
	if !shouldRepeat {
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.publishForDevice(device, qos)
		case <-p.done:
			return
		}
	}
}

// publishForDevice publishes a message for a specific device
func (p *PerTopicDeviceSimulator) publishForDevice(device *perTopicSimulatedDevice, qos byte) {
	payload := p.generatePayload(device.deviceID)

	if err := p.broker.Publish(device.topic, payload, qos, p.topic.Retain); err != nil {
		log.Printf("Per-topic device simulator: failed to publish for device %s: %v", device.deviceID, err)
		return
	}

	device.mu.Lock()
	device.lastPublish = time.Now()
	atomic.AddInt64(&device.messageCount, 1)
	device.mu.Unlock()
}

// generateDeviceID generates a device ID from the pattern
func (p *PerTopicDeviceSimulator) generateDeviceID(index int) string {
	pattern := p.settings.DeviceIDPattern
	// Support {n}, {id}, and {index} placeholders
	pattern = strings.ReplaceAll(pattern, "{n}", strconv.Itoa(index))
	pattern = strings.ReplaceAll(pattern, "{id}", strconv.Itoa(index))
	pattern = strings.ReplaceAll(pattern, "{index}", strconv.Itoa(index))
	return pattern
}

// generateTopic generates a topic for a device
func (p *PerTopicDeviceSimulator) generateTopic(deviceID string) string {
	return strings.ReplaceAll(p.topicPattern, "{device_id}", deviceID)
}

// generatePayload generates a payload for a device using the message template
func (p *PerTopicDeviceSimulator) generatePayload(deviceID string) []byte {
	// Get payload template from first message config
	payloadTemplate := ""
	if len(p.topic.Messages) > 0 {
		payloadTemplate = p.topic.Messages[0].Payload
	}

	if payloadTemplate == "" {
		// Default payload if none specified
		return []byte(fmt.Sprintf(`{"deviceId":"%s","timestamp":%d}`, deviceID, time.Now().UnixMilli()))
	}

	// Use the template engine
	tmpl := NewTemplate(payloadTemplate, p.sequences)
	ctx := &TemplateContext{
		DeviceID: deviceID,
		Topic:    p.generateTopic(deviceID),
	}

	rendered := tmpl.Render(ctx)
	if rendered == "" {
		return []byte(payloadTemplate)
	}

	// Process shared template variables ({{now}}, {{uuid.short}}, etc.)
	rendered = processSharedTemplateVars(rendered)

	return []byte(rendered)
}

// runMessageLoop handles publishing messages for a single message configuration
func (s *Simulator) runMessageLoop(topic TopicConfig, msg MessageConfig) {
	defer s.wg.Done()

	// Parse initial delay
	if msg.Delay != "" {
		delay, err := time.ParseDuration(msg.Delay)
		if err == nil {
			select {
			case <-time.After(delay):
			case <-s.done:
				return
			}
		}
	}

	// Publish initial message
	s.publishMessage(topic, msg)

	// If not repeating, we're done
	if !msg.Repeat {
		return
	}

	// Parse interval for repeating messages
	interval := 5 * time.Second // Default interval
	if msg.Interval != "" {
		if parsed, err := time.ParseDuration(msg.Interval); err == nil {
			interval = parsed
		}
	}

	// Create ticker for repeating messages
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.publishMessage(topic, msg)
		case <-s.done:
			return
		}
	}
}

// publishMessage publishes a single message to a topic
func (s *Simulator) publishMessage(topic TopicConfig, msg MessageConfig) {
	qos := byte(topic.QoS)
	if qos > 2 {
		qos = 0
	}

	// Process the payload through the template engine for dynamic values
	payload := s.processPayload(msg.Payload, topic.Topic)
	_ = s.broker.Publish(topic.Topic, payload, qos, topic.Retain)
}

// processPayload processes the payload through the template engine
func (s *Simulator) processPayload(rawPayload, topicName string) []byte {
	tmpl := NewTemplate(rawPayload, s.sequences)
	ctx := &TemplateContext{
		Topic: topicName,
	}

	rendered := tmpl.Render(ctx)
	if rendered == "" {
		log.Printf("MQTT simulator: template rendered empty for topic %s, using raw payload", topicName)
		return []byte(rawPayload)
	}

	// Process shared template variables ({{now}}, {{uuid.short}}, etc.)
	rendered = processSharedTemplateVars(rendered)

	return []byte(rendered)
}

// MessageGenerator is an interface for generating dynamic message payloads
type MessageGenerator interface {
	Generate() []byte
}

// StaticGenerator returns a static payload
type StaticGenerator struct {
	payload []byte
}

// NewStaticGenerator creates a new static generator
func NewStaticGenerator(payload string) *StaticGenerator {
	return &StaticGenerator{
		payload: []byte(payload),
	}
}

// Generate returns the static payload
func (g *StaticGenerator) Generate() []byte {
	return g.payload
}

// TemplateGenerator generates payloads from a template
type TemplateGenerator struct {
	template string
	values   map[string]func() interface{}
}

// NewTemplateGenerator creates a new template generator
func NewTemplateGenerator(template string, values map[string]func() interface{}) *TemplateGenerator {
	return &TemplateGenerator{
		template: template,
		values:   values,
	}
}

// Generate returns a payload with substituted values
func (g *TemplateGenerator) Generate() []byte {
	// For now, return the template as-is
	// Future enhancement: implement template substitution
	return []byte(g.template)
}

// DeviceSimulator simulates a specific IoT device type
type DeviceSimulator struct {
	broker     *Broker
	deviceID   string
	deviceType string
	baseTopic  string
	generators map[string]MessageGenerator
	interval   time.Duration
	done       chan struct{}
	wg         sync.WaitGroup
}

// NewDeviceSimulator creates a simulator for a specific device
func NewDeviceSimulator(broker *Broker, deviceID, deviceType, baseTopic string, interval time.Duration) *DeviceSimulator {
	return &DeviceSimulator{
		broker:     broker,
		deviceID:   deviceID,
		deviceType: deviceType,
		baseTopic:  baseTopic,
		generators: make(map[string]MessageGenerator),
		interval:   interval,
		done:       make(chan struct{}),
	}
}

// AddGenerator adds a message generator for a sub-topic
func (d *DeviceSimulator) AddGenerator(subTopic string, generator MessageGenerator) {
	d.generators[subTopic] = generator
}

// Start begins device simulation
func (d *DeviceSimulator) Start() {
	for subTopic, generator := range d.generators {
		d.wg.Add(1)
		go d.runGenerator(subTopic, generator)
	}
}

// Stop stops device simulation
func (d *DeviceSimulator) Stop() {
	close(d.done)
	d.wg.Wait()
}

// runGenerator runs a single generator loop
func (d *DeviceSimulator) runGenerator(subTopic string, generator MessageGenerator) {
	defer d.wg.Done()

	topic := d.baseTopic + "/" + d.deviceID + "/" + subTopic
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			payload := generator.Generate()
			_ = d.broker.Publish(topic, payload, 0, false)
		case <-d.done:
			return
		}
	}
}

// ============================================================================
// Multi-Device Simulation
// ============================================================================

// MultiDeviceSimulator manages multiple simulated devices publishing concurrently
type MultiDeviceSimulator struct {
	broker    *Broker
	config    *MultiDeviceSimulationConfig
	sequences *SequenceStore
	devices   []*simulatedDevice
	done      chan struct{}
	wg        sync.WaitGroup
	mu        sync.RWMutex
	running   bool
	startedAt time.Time
}

// simulatedDevice represents a single simulated device with its own state
type simulatedDevice struct {
	deviceID     string
	topic        string
	connected    bool
	lastPublish  time.Time
	messageCount int64
	mu           sync.RWMutex
}

// NewMultiDeviceSimulator creates a new multi-device simulator
func NewMultiDeviceSimulator(broker *Broker, config *MultiDeviceSimulationConfig) (*MultiDeviceSimulator, error) {
	if err := ValidateMultiDeviceConfig(config); err != nil {
		return nil, err
	}

	return &MultiDeviceSimulator{
		broker:    broker,
		config:    config,
		sequences: NewSequenceStore(),
		done:      make(chan struct{}),
	}, nil
}

// ValidateMultiDeviceConfig validates the multi-device simulation configuration
func ValidateMultiDeviceConfig(config *MultiDeviceSimulationConfig) error {
	if config == nil {
		return errors.New("config cannot be nil")
	}
	if config.DeviceCount < 1 || config.DeviceCount > 1000 {
		return fmt.Errorf("deviceCount must be between 1 and 1000, got %d", config.DeviceCount)
	}
	if config.DeviceIDPattern == "" {
		return errors.New("deviceIdPattern is required")
	}
	if !strings.Contains(config.DeviceIDPattern, "{n}") && !strings.Contains(config.DeviceIDPattern, "{id}") && !strings.Contains(config.DeviceIDPattern, "{index}") {
		return errors.New("deviceIdPattern must contain {n}, {id}, or {index} placeholder")
	}
	if config.TopicPattern == "" {
		return errors.New("topicPattern is required")
	}
	if !strings.Contains(config.TopicPattern, "{device_id}") {
		return errors.New("topicPattern must contain {device_id} placeholder")
	}
	if config.IntervalMs < 100 {
		return fmt.Errorf("intervalMs must be at least 100ms, got %d", config.IntervalMs)
	}
	if config.QoS < 0 || config.QoS > 2 {
		return fmt.Errorf("qos must be 0, 1, or 2, got %d", config.QoS)
	}
	return nil
}

// Start begins the multi-device simulation
func (m *MultiDeviceSimulator) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return errors.New("simulation is already running")
	}

	// Create devices
	m.devices = make([]*simulatedDevice, m.config.DeviceCount)
	for i := 0; i < m.config.DeviceCount; i++ {
		deviceID := m.generateDeviceID(i + 1)
		topic := m.generateTopic(deviceID)
		m.devices[i] = &simulatedDevice{
			deviceID:  deviceID,
			topic:     topic,
			connected: true,
		}
	}

	m.running = true
	m.startedAt = time.Now()
	m.done = make(chan struct{})

	// Start a goroutine for each device
	for _, device := range m.devices {
		m.wg.Add(1)
		go m.runDevice(device)
	}

	return nil
}

// Stop stops the multi-device simulation
func (m *MultiDeviceSimulator) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	close(m.done)
	m.mu.Unlock()

	m.wg.Wait()

	// Mark all devices as disconnected
	m.mu.Lock()
	for _, device := range m.devices {
		device.mu.Lock()
		device.connected = false
		device.mu.Unlock()
	}
	m.mu.Unlock()
}

// IsRunning returns whether the simulation is running
func (m *MultiDeviceSimulator) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetStatus returns the current status of the multi-device simulation
func (m *MultiDeviceSimulator) GetStatus() *MultiDeviceSimulationStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := &MultiDeviceSimulationStatus{
		Running:     m.running,
		DeviceCount: len(m.devices),
		Config:      m.config,
	}

	if m.running {
		status.StartedAt = m.startedAt.Unix()
	}

	var totalMessages int64
	activeDevices := 0
	deviceStatuses := make([]DeviceSimulationStatus, 0, len(m.devices))

	for _, device := range m.devices {
		device.mu.RLock()
		ds := DeviceSimulationStatus{
			DeviceID:     device.deviceID,
			Connected:    device.connected,
			MessageCount: device.messageCount,
			Topic:        device.topic,
		}
		if !device.lastPublish.IsZero() {
			ds.LastPublish = device.lastPublish.Unix()
		}
		device.mu.RUnlock()

		totalMessages += ds.MessageCount
		if ds.Connected {
			activeDevices++
		}
		deviceStatuses = append(deviceStatuses, ds)
	}

	status.TotalMessages = totalMessages
	status.ActiveDevices = activeDevices
	status.Devices = deviceStatuses

	return status
}

// GetDeviceStatus returns the status of a specific device
func (m *MultiDeviceSimulator) GetDeviceStatus(deviceID string) *DeviceSimulationStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, device := range m.devices {
		device.mu.RLock()
		if device.deviceID == deviceID {
			status := &DeviceSimulationStatus{
				DeviceID:     device.deviceID,
				Connected:    device.connected,
				MessageCount: device.messageCount,
				Topic:        device.topic,
			}
			if !device.lastPublish.IsZero() {
				status.LastPublish = device.lastPublish.Unix()
			}
			device.mu.RUnlock()
			return status
		}
		device.mu.RUnlock()
	}
	return nil
}

// runDevice runs the simulation loop for a single device
func (m *MultiDeviceSimulator) runDevice(device *simulatedDevice) {
	defer m.wg.Done()

	interval := time.Duration(m.config.IntervalMs) * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	qos := byte(m.config.QoS)
	if qos > 2 {
		qos = 0
	}

	// Publish initial message
	m.publishForDevice(device, qos)

	for {
		select {
		case <-ticker.C:
			m.publishForDevice(device, qos)
		case <-m.done:
			return
		}
	}
}

// publishForDevice publishes a message for a specific device
func (m *MultiDeviceSimulator) publishForDevice(device *simulatedDevice, qos byte) {
	payload := m.generatePayload(device.deviceID)

	if err := m.broker.Publish(device.topic, payload, qos, m.config.Retain); err != nil {
		log.Printf("Multi-device simulator: failed to publish for device %s: %v", device.deviceID, err)
		return
	}

	device.mu.Lock()
	device.lastPublish = time.Now()
	atomic.AddInt64(&device.messageCount, 1)
	device.mu.Unlock()
}

// generateDeviceID generates a device ID from the pattern
func (m *MultiDeviceSimulator) generateDeviceID(index int) string {
	pattern := m.config.DeviceIDPattern
	pattern = strings.ReplaceAll(pattern, "{n}", strconv.Itoa(index))
	pattern = strings.ReplaceAll(pattern, "{id}", strconv.Itoa(index))
	pattern = strings.ReplaceAll(pattern, "{index}", strconv.Itoa(index))
	return pattern
}

// generateTopic generates a topic for a device
func (m *MultiDeviceSimulator) generateTopic(deviceID string) string {
	return strings.ReplaceAll(m.config.TopicPattern, "{device_id}", deviceID)
}

// generatePayload generates a payload for a device using the template
func (m *MultiDeviceSimulator) generatePayload(deviceID string) []byte {
	if m.config.PayloadTemplate == "" {
		// Default payload if none specified
		return []byte(fmt.Sprintf(`{"deviceId":"%s","timestamp":%d}`, deviceID, time.Now().UnixMilli()))
	}

	// Use the template engine
	tmpl := NewTemplate(m.config.PayloadTemplate, m.sequences)
	ctx := &TemplateContext{
		DeviceID: deviceID,
	}

	rendered := tmpl.Render(ctx)
	if rendered == "" {
		return []byte(m.config.PayloadTemplate)
	}

	// Process shared template variables ({{now}}, {{uuid.short}}, etc.)
	rendered = processSharedTemplateVars(rendered)

	return []byte(rendered)
}
