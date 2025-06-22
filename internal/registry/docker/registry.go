package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"gateway/internal/core"
	"gateway/pkg/errors"
)

// Config represents Docker registry configuration
type Config struct {
	// Docker connection settings
	Host     string `yaml:"host"`     // Docker daemon host
	Version  string `yaml:"version"`  // Docker API version
	CertPath string `yaml:"certPath"` // Path to certificates for TLS

	// Service discovery settings
	LabelPrefix     string `yaml:"labelPrefix"`     // Label prefix for gateway config
	Network         string `yaml:"network"`         // Docker network to use
	RefreshInterval int    `yaml:"refreshInterval"` // Service refresh interval in seconds
}

// DefaultConfig returns default Docker registry configuration
func DefaultConfig() *Config {
	return &Config{
		Host:            "", // Use default Docker socket
		Version:         "", // Use default API version
		LabelPrefix:     "gateway",
		RefreshInterval: 10,
	}
}

// Registry implements service discovery using Docker
type Registry struct {
	config     *Config
	httpClient *http.Client
	baseURL    string
	services   map[string][]core.ServiceInstance
	mu         sync.RWMutex
	logger     *slog.Logger
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// Container represents a Docker container
type Container struct {
	ID              string            `json:"Id"`
	Names           []string          `json:"Names"`
	Labels          map[string]string `json:"Labels"`
	State           string            `json:"State"`
	NetworkSettings struct {
		Networks map[string]struct {
			IPAddress string `json:"IPAddress"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
}

// NewRegistry creates a new Docker registry using HTTP API
func NewRegistry(config *Config, logger *slog.Logger) (*Registry, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Determine Docker API URL
	var baseURL string
	if config.Host != "" {
		baseURL = config.Host
	} else {
		// Use Unix socket if no host specified
		baseURL = "http://localhost"
	}

	// Create HTTP client
	var httpClient *http.Client
	if strings.HasPrefix(baseURL, "unix://") {
		// Unix socket connection
		socketPath := strings.TrimPrefix(baseURL, "unix://")
		httpClient = &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					dialer := net.Dialer{}
					return dialer.DialContext(ctx, "unix", socketPath)
				},
			},
			Timeout: 10 * time.Second,
		}
		baseURL = "http://localhost"
	} else {
		// TCP connection
		httpClient = &http.Client{
			Timeout: 10 * time.Second,
		}
	}

	r := &Registry{
		config:     config,
		httpClient: httpClient,
		baseURL:    baseURL,
		services:   make(map[string][]core.ServiceInstance),
		logger:     logger,
		stopCh:     make(chan struct{}),
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := r.ping(ctx); err != nil {
		return nil, errors.NewError(errors.ErrorTypeUnavailable, "failed to connect to Docker daemon").WithCause(err)
	}

	// Start background refresh if enabled
	if config.RefreshInterval > 0 {
		r.wg.Add(1)
		go r.refreshLoop()
	}

	// Initial load
	if err := r.refresh(); err != nil {
		logger.Error("Initial service discovery failed", "error", err)
	}

	return r, nil
}

// ping tests the connection to Docker daemon
func (r *Registry) ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", r.baseURL+"/_ping", nil)
	if err != nil {
		return err
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("docker ping failed with status: %d", resp.StatusCode)
	}

	return nil
}

// GetService returns instances for a service
func (r *Registry) GetService(name string) ([]core.ServiceInstance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instances, ok := r.services[name]
	if !ok {
		return nil, errors.NewError(errors.ErrorTypeNotFound, fmt.Sprintf("service %s not found", name))
	}

	// Return a copy to avoid race conditions
	result := make([]core.ServiceInstance, len(instances))
	copy(result, instances)

	return result, nil
}

// refresh discovers services from Docker
func (r *Registry) refresh() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build query parameters for filtering
	query := fmt.Sprintf("filters={\"label\":[\"%s.service\"]}", r.config.LabelPrefix)
	if r.config.Network != "" {
		query = fmt.Sprintf("filters={\"label\":[\"%s.service\"],\"network\":[\"%s\"]}",
			r.config.LabelPrefix, r.config.Network)
	}

	req, err := http.NewRequestWithContext(ctx, "GET",
		r.baseURL+"/containers/json?"+query, nil)
	if err != nil {
		return err
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return errors.NewError(errors.ErrorTypeUnavailable, "failed to list containers").WithCause(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("docker API error: %d - %s", resp.StatusCode, string(body))
	}

	var containers []Container
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return errors.NewError(errors.ErrorTypeInternal, "failed to decode container list").WithCause(err)
	}

	// Build service map
	services := make(map[string][]core.ServiceInstance)

	for _, container := range containers {
		if container.State != "running" {
			continue
		}

		// Extract service info from labels
		serviceName := container.Labels[r.config.LabelPrefix+".service"]
		if serviceName == "" {
			continue
		}

		// Get port from label
		portStr := container.Labels[r.config.LabelPrefix+".port"]
		if portStr == "" {
			continue
		}

		port, err := strconv.Atoi(portStr)
		if err != nil {
			r.logger.Warn("Invalid port in container label",
				"container", container.ID,
				"port", portStr,
				"error", err,
			)
			continue
		}

		// Get IP address
		var ipAddress string
		if r.config.Network != "" {
			if net, ok := container.NetworkSettings.Networks[r.config.Network]; ok {
				ipAddress = net.IPAddress
			}
		} else {
			// Use first available network
			for _, net := range container.NetworkSettings.Networks {
				if net.IPAddress != "" {
					ipAddress = net.IPAddress
					break
				}
			}
		}

		if ipAddress == "" {
			r.logger.Warn("No IP address found for container",
				"container", container.ID,
				"service", serviceName,
			)
			continue
		}

		// Extract metadata
		metadata := make(map[string]any)
		for k, v := range container.Labels {
			if strings.HasPrefix(k, r.config.LabelPrefix+".meta.") {
				key := strings.TrimPrefix(k, r.config.LabelPrefix+".meta.")
				metadata[key] = v
			}
		}

		// Determine scheme
		scheme := "http"
		if v, ok := container.Labels[r.config.LabelPrefix+".scheme"]; ok {
			scheme = v
		}

		// Check health
		healthy := true
		if v, ok := container.Labels[r.config.LabelPrefix+".health"]; ok {
			healthy = v == "healthy" || v == "true"
		}

		// Use short container ID (first 12 chars) if ID is long enough
		containerID := container.ID
		if len(containerID) > 12 {
			containerID = containerID[:12]
		}

		instance := core.ServiceInstance{
			ID:       containerID,
			Name:     serviceName,
			Address:  ipAddress,
			Port:     port,
			Scheme:   scheme,
			Healthy:  healthy,
			Metadata: metadata,
		}

		services[serviceName] = append(services[serviceName], instance)

		r.logger.Debug("Discovered service instance",
			"service", serviceName,
			"id", instance.ID,
			"address", instance.Address,
			"port", instance.Port,
		)
	}

	// Update services atomically
	r.mu.Lock()
	r.services = services
	r.mu.Unlock()

	r.logger.Info("Service discovery completed",
		"services", len(services),
		"total_instances", r.countInstances(services),
	)

	return nil
}

// refreshLoop runs periodic service discovery
func (r *Registry) refreshLoop() {
	defer r.wg.Done()

	ticker := time.NewTicker(time.Duration(r.config.RefreshInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := r.refresh(); err != nil {
				r.logger.Error("Service refresh failed", "error", err)
			}
		case <-r.stopCh:
			return
		}
	}
}

// Close stops the registry
func (r *Registry) Close() error {
	close(r.stopCh)
	r.wg.Wait()
	return nil
}

// countInstances counts total instances across all services
func (r *Registry) countInstances(services map[string][]core.ServiceInstance) int {
	count := 0
	for _, instances := range services {
		count += len(instances)
	}
	return count
}
