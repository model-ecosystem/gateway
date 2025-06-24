package health

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"gateway/internal/config"
	"gateway/internal/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// BackendMonitor monitors backend service health
type BackendMonitor struct {
	registry        core.ServiceRegistry
	config          *config.Health
	healthCheckers  map[string]BackendChecker
	updateCallbacks []func(service string, instance *core.ServiceInstance, healthy bool)
	logger          *slog.Logger
	
	mu       sync.RWMutex
	statuses map[string]map[string]*InstanceHealth // service -> instance -> health
	
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// InstanceHealth holds health status for an instance
type InstanceHealth struct {
	Instance         *core.ServiceInstance
	Healthy          bool
	LastCheck        time.Time
	ConsecutiveFails int
	LastError        error
}

// BackendChecker checks health of a specific backend type
type BackendChecker interface {
	Check(ctx context.Context, instance *core.ServiceInstance) error
}

// NewBackendMonitor creates a new backend health monitor
func NewBackendMonitor(registry core.ServiceRegistry, config *config.Health, logger *slog.Logger) *BackendMonitor {
	return &BackendMonitor{
		registry:       registry,
		config:         config,
		healthCheckers: make(map[string]BackendChecker),
		statuses:       make(map[string]map[string]*InstanceHealth),
		logger:         logger,
	}
}

// RegisterChecker registers a health checker for a check type
func (m *BackendMonitor) RegisterChecker(checkType string, checker BackendChecker) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthCheckers[checkType] = checker
}

// RegisterUpdateCallback registers a callback for health status updates
func (m *BackendMonitor) RegisterUpdateCallback(callback func(service string, instance *core.ServiceInstance, healthy bool)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCallbacks = append(m.updateCallbacks, callback)
}

// Start starts the backend monitoring
func (m *BackendMonitor) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.cancel != nil {
		m.mu.Unlock()
		return fmt.Errorf("monitor already started")
	}
	
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.mu.Unlock()
	
	// Register default checkers
	m.registerDefaultCheckers()
	
	// Start monitoring configured checks
	for name, check := range m.config.Checks {
		if checkType, ok := check.Config["type"]; ok {
			m.wg.Add(1)
			go m.monitorService(name, checkType, check)
		}
	}
	
	m.logger.Info("Backend monitor started", "checks", len(m.config.Checks))
	return nil
}

// Stop stops the backend monitoring
func (m *BackendMonitor) Stop() error {
	m.mu.Lock()
	if m.cancel == nil {
		m.mu.Unlock()
		return fmt.Errorf("monitor not started")
	}
	
	cancel := m.cancel
	m.cancel = nil
	m.mu.Unlock()
	
	cancel()
	m.wg.Wait()
	
	m.logger.Info("Backend monitor stopped")
	return nil
}

// GetHealth returns the current health status
func (m *BackendMonitor) GetHealth(service, instanceID string) (*InstanceHealth, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if serviceMap, ok := m.statuses[service]; ok {
		if health, ok := serviceMap[instanceID]; ok {
			return health, true
		}
	}
	return nil, false
}

// GetServiceHealth returns health status for all instances of a service
func (m *BackendMonitor) GetServiceHealth(service string) map[string]*InstanceHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if serviceMap, ok := m.statuses[service]; ok {
		// Return a copy
		result := make(map[string]*InstanceHealth, len(serviceMap))
		for k, v := range serviceMap {
			result[k] = v
		}
		return result
	}
	return nil
}

// monitorService monitors health of a service
func (m *BackendMonitor) monitorService(name, checkType string, check config.Check) {
	defer m.wg.Done()
	
	interval := time.Duration(check.Interval) * time.Second
	if interval <= 0 {
		interval = 30 * time.Second
	}
	
	timeout := time.Duration(check.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	// Initial check
	m.checkService(name, checkType, timeout)
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkService(name, checkType, timeout)
		}
	}
}

// checkService performs health checks on all instances of a service
func (m *BackendMonitor) checkService(serviceName, checkType string, timeout time.Duration) {
	// Get instances from registry
	instances, err := m.registry.GetService(serviceName)
	if err != nil {
		m.logger.Error("Failed to get service instances",
			"service", serviceName,
			"error", err,
		)
		return
	}
	
	// Check each instance
	var wg sync.WaitGroup
	for i := range instances {
		instance := instances[i]
		wg.Add(1)
		go func(inst core.ServiceInstance) {
			defer wg.Done()
			m.checkInstance(serviceName, &inst, checkType, timeout)
		}(instance)
	}
	
	wg.Wait()
}

// checkInstance checks health of a single instance
func (m *BackendMonitor) checkInstance(serviceName string, instance *core.ServiceInstance, checkType string, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(m.ctx, timeout)
	defer cancel()
	
	// Get the checker
	m.mu.RLock()
	checker, ok := m.healthCheckers[checkType]
	m.mu.RUnlock()
	
	if !ok {
		m.logger.Error("Unknown check type",
			"service", serviceName,
			"instance", instance.ID,
			"checkType", checkType,
		)
		return
	}
	
	// Perform the check
	err := checker.Check(ctx, instance)
	
	// Update status
	m.updateInstanceHealth(serviceName, instance, err)
}

// updateInstanceHealth updates the health status of an instance
func (m *BackendMonitor) updateInstanceHealth(serviceName string, instance *core.ServiceInstance, checkErr error) {
	m.mu.Lock()
	
	// Initialize maps if needed
	if m.statuses[serviceName] == nil {
		m.statuses[serviceName] = make(map[string]*InstanceHealth)
	}
	
	// Get or create health status
	health, exists := m.statuses[serviceName][instance.ID]
	if !exists {
		health = &InstanceHealth{
			Instance: instance,
		}
		m.statuses[serviceName][instance.ID] = health
	}
	
	// Update health status
	health.LastCheck = time.Now()
	health.LastError = checkErr
	
	previouslyHealthy := health.Healthy
	
	if checkErr != nil {
		health.ConsecutiveFails++
		health.Healthy = false
		m.logger.Debug("Health check failed",
			"service", serviceName,
			"instance", instance.ID,
			"consecutiveFails", health.ConsecutiveFails,
			"error", checkErr,
		)
	} else {
		health.ConsecutiveFails = 0
		health.Healthy = true
		if !previouslyHealthy {
			m.logger.Info("Instance became healthy",
				"service", serviceName,
				"instance", instance.ID,
			)
		}
	}
	
	// Get callbacks while holding the lock
	callbacks := make([]func(string, *core.ServiceInstance, bool), len(m.updateCallbacks))
	copy(callbacks, m.updateCallbacks)
	
	m.mu.Unlock()
	
	// Notify callbacks if health changed
	if health.Healthy != previouslyHealthy {
		for _, callback := range callbacks {
			callback(serviceName, instance, health.Healthy)
		}
	}
}

// registerDefaultCheckers registers the default health checkers
func (m *BackendMonitor) registerDefaultCheckers() {
	// HTTP health checker
	m.RegisterChecker("http", &HTTPHealthChecker{})
	
	// TCP health checker
	m.RegisterChecker("tcp", &TCPHealthChecker{})
	
	// gRPC health checker
	m.RegisterChecker("grpc", &GRPCHealthChecker{})
}

// HTTPHealthChecker checks HTTP endpoints
type HTTPHealthChecker struct{}

func (h *HTTPHealthChecker) Check(ctx context.Context, instance *core.ServiceInstance) error {
	// Build health check URL
	scheme := instance.Scheme
	if scheme == "" {
		scheme = "http"
	}
	
	url := fmt.Sprintf("%s://%s:%d/health", scheme, instance.Address, instance.Port)
	
	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	
	// Make request
	client := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
	
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	// Check status
	if resp.StatusCode >= 400 {
		return fmt.Errorf("unhealthy status: %d", resp.StatusCode)
	}
	
	return nil
}

// TCPHealthChecker checks TCP connectivity
type TCPHealthChecker struct{}

func (t *TCPHealthChecker) Check(ctx context.Context, instance *core.ServiceInstance) error {
	addr := fmt.Sprintf("%s:%d", instance.Address, instance.Port)
	
	// Try to connect
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("tcp connect failed: %w", err)
	}
	
	// Close connection
	if err := conn.Close(); err != nil {
		return fmt.Errorf("close failed: %w", err)
	}
	
	return nil
}

// GRPCHealthChecker checks gRPC health
type GRPCHealthChecker struct{}

func (g *GRPCHealthChecker) Check(ctx context.Context, instance *core.ServiceInstance) error {
	addr := fmt.Sprintf("%s:%d", instance.Address, instance.Port)
	
	// Create gRPC connection
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithInsecure(),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("grpc dial failed: %w", err)
	}
	defer conn.Close()
	
	// Create health client
	client := grpc_health_v1.NewHealthClient(conn)
	
	// Check health
	resp, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	
	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("not serving: %v", resp.Status)
	}
	
	return nil
}